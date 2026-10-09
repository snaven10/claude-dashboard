// Package history calcula el uso de tokens por día (últimos 30 días locales) a
// partir de los transcripts de Claude Code (PLAN-006, DD-26 y DD-27). No persiste
// nada: lee ~/.claude/projects/**/*.jsonl (incluye subagents/) de forma
// incremental por archivo y un escáner propio, fuera del poller de sesiones.
package history

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// Days es la cantidad de días que devuelve Result.
	Days = 30
	// topN es el máximo de filas por modelo/proyecto; el resto va en "otros".
	topN      = 5
	otherName = "otros"
	unknown   = "desconocido"
	maxLine   = 64 << 20
)

// chunkSize es el tamaño de bloque de lectura (var: los tests lo achican).
var chunkSize = 4 << 20

// Row es una fila de desglose (por modelo o por proyecto).
type Row struct {
	Name   string `json:"name"`
	Output int64  `json:"output"`
	Total  int64  `json:"total"`
}

// Day es el uso de un día local.
type Day struct {
	Date          string `json:"date"`
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cacheRead"`
	CacheCreation int64  `json:"cacheCreation"`
	Messages      int64  `json:"messages"`
	Sessions      int    `json:"sessions"`
	ByModel       []Row  `json:"byModel"`
	ByProject     []Row  `json:"byProject"`
}

// Response es el contrato de GET /api/history.
type Response struct {
	GeneratedAt int64 `json:"generatedAt"`
	Ready       bool  `json:"ready"`
	Days        []Day `json:"days"`
}

type key struct {
	day, model, project, session string
}

type counts struct {
	input, output, cacheRead, cacheCreation, messages int64
}

func (c *counts) add(o counts) {
	c.input += o.input
	c.output += o.output
	c.cacheRead += o.cacheRead
	c.cacheCreation += o.cacheCreation
	c.messages += o.messages
}

func (c counts) total() int64 { return c.input + c.output + c.cacheRead + c.cacheCreation }

// fileState es el aporte de un archivo: permite recalcular al achicarse y
// descartarlo si desaparece.
type fileState struct {
	offset    int64
	lastMsgID string
	sessionID string
	contrib   map[key]*counts
}

// Scanner mantiene el estado incremental. Scan lo usa una sola goroutine a la
// vez; Result puede llamarse en paralelo y nunca toca el disco.
type Scanner struct {
	Root      string                  // ~/.claude/projects
	Loc       *time.Location          // nil = time.Local
	ProjectOf func(cwd string) string // nil = basename del cwd
	Logf      func(string, ...any)    // nil = log.Printf

	scanMu sync.Mutex
	files  map[string]*fileState

	bytesRead atomic.Int64

	mu    sync.Mutex
	ready bool
	agg   map[key]counts
}

// BytesRead son los bytes leídos de transcripts desde el inicio (para tests).
func (s *Scanner) BytesRead() int64 { return s.bytesRead.Load() }

func (s *Scanner) loc() *time.Location {
	if s.Loc != nil {
		return s.Loc
	}
	return time.Local
}

func (s *Scanner) logf(f string, a ...any) {
	if s.Logf != nil {
		s.Logf(f, a...)
		return
	}
	log.Printf(f, a...)
}

type usageLine struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input         int64 `json:"input_tokens"`
			Output        int64 `json:"output_tokens"`
			CacheRead     int64 `json:"cache_read_input_tokens"`
			CacheCreation int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// sessionOf deduce el sessionId padre del path relativo a Root:
// <slug>/<sid>.jsonl o <slug>/<sid>/subagents/<x>.jsonl. "" = no es un transcript.
func sessionOf(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	switch {
	case len(parts) == 2 && strings.HasSuffix(parts[1], ".jsonl"):
		return strings.TrimSuffix(parts[1], ".jsonl")
	case len(parts) >= 4 && parts[2] == "subagents" && strings.HasSuffix(parts[len(parts)-1], ".jsonl"):
		return parts[1]
	}
	return ""
}

// Scan recorre Root, lee solo lo nuevo de cada archivo y publica el agregado.
// Los archivos desaparecidos dejan de contar. Un error en un archivo se loguea
// y no corta el resto.
func (s *Scanner) Scan() error {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if s.files == nil {
		s.files = map[string]*fileState{}
	}
	seen := map[string]bool{}
	err := filepath.WalkDir(s.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == s.Root {
				return err
			}
			return nil // subdirectorio ilegible: se salta
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		rel, rerr := filepath.Rel(s.Root, path)
		if rerr != nil {
			return nil
		}
		sid := sessionOf(rel)
		if sid == "" {
			return nil
		}
		seen[path] = true
		if ferr := s.scanFile(path, sid); ferr != nil {
			s.logf("history: %s: %v", path, ferr)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("recorriendo %s: %w", s.Root, err)
	}
	for p := range s.files {
		if !seen[p] {
			delete(s.files, p)
		}
	}
	agg := map[key]counts{}
	for _, fst := range s.files {
		for k, c := range fst.contrib {
			a := agg[k]
			a.add(*c)
			agg[k] = a
		}
	}
	s.mu.Lock()
	s.agg, s.ready = agg, true
	s.mu.Unlock()
	return nil
}

func (s *Scanner) scanFile(path, sid string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	st := s.files[path]
	if st == nil || size < st.offset { // nuevo, o truncado/rotado: desde 0 sin lo anterior
		st = &fileState{sessionID: sid, contrib: map[key]*counts{}}
		s.files[path] = st
	}
	if size == st.offset { // sin bytes nuevos: ni buffer ni lectura (corre cada 60 s sobre ~1000 archivos)
		return nil
	}
	buf := make([]byte, min(int64(chunkSize), size-st.offset))
	var resto []byte
	for pos := st.offset + int64(len(resto)); pos < size; {
		n, err := f.ReadAt(buf[:min(int64(len(buf)), size-pos)], pos)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n == 0 {
			break
		}
		s.bytesRead.Add(int64(n))
		pos += int64(n)
		data := append(resto, buf[:n]...)
		end := bytes.LastIndexByte(data, '\n')
		if end < 0 {
			resto = data
		} else {
			s.consume(st, data[:end+1])
			st.offset += int64(end + 1)
			resto = append([]byte(nil), data[end+1:]...)
		}
		if len(resto) > maxLine {
			return fmt.Errorf("línea de más de %d bytes", maxLine)
		}
	}
	return nil
}

func (s *Scanner) consume(st *fileState, data []byte) {
	loc := s.loc()
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		raw := bytes.TrimSpace(data[:i])
		data = data[i+1:]
		if len(raw) == 0 || !bytes.Contains(raw, []byte(`"usage"`)) {
			continue
		}
		var l usageLine
		if json.Unmarshal(raw, &l) != nil || l.Type != "assistant" || l.Message.Usage == nil {
			continue
		}
		if id := l.Message.ID; id != "" && id == st.lastMsgID { // línea repetida del mismo mensaje
			continue
		}
		st.lastMsgID = l.Message.ID
		t, err := time.Parse(time.RFC3339Nano, l.Timestamp)
		if err != nil {
			continue
		}
		model := l.Message.Model
		if model == "" {
			model = unknown
		}
		project := unknown
		if l.Cwd != "" {
			if s.ProjectOf != nil {
				project = s.ProjectOf(l.Cwd)
			} else {
				project = filepath.Base(l.Cwd)
			}
			if project == "" {
				project = unknown
			}
		}
		k := key{day: t.In(loc).Format("2006-01-02"), model: model, project: project, session: st.sessionID}
		c := st.contrib[k]
		if c == nil {
			c = &counts{}
			st.contrib[k] = c
		}
		u := l.Message.Usage
		c.add(counts{u.Input, u.Output, u.CacheRead, u.CacheCreation, 1})
	}
}

// Run hace el primer escaneo en esta goroutine (loguea cuánto tardó) y después
// uno cada interval, hasta que stop se cierre. interval <= 0 solo escanea una vez.
func (s *Scanner) Run(stop <-chan struct{}, interval time.Duration) {
	start := time.Now()
	if err := s.Scan(); err != nil {
		s.logf("history: primer escaneo: %v", err)
	} else {
		s.logf("history: primer escaneo en %s (%d bytes)", time.Since(start).Round(time.Millisecond), s.BytesRead())
	}
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			if err := s.Scan(); err != nil {
				s.logf("history: %v", err)
			}
		}
	}
}

// Result arma la respuesta con lo último escaneado; no toca el disco.
func (s *Scanner) Result(now time.Time) Response {
	s.mu.Lock()
	ready, agg := s.ready, s.agg
	s.mu.Unlock()
	resp := Response{GeneratedAt: now.UnixMilli(), Ready: ready, Days: []Day{}}
	if !ready {
		return resp
	}
	loc := s.loc()
	ln := now.In(loc)
	type dayAcc struct {
		day      Day
		sessions map[string]bool
		models   map[string]counts
		projects map[string]counts
	}
	accs := make(map[string]*dayAcc, Days)
	order := make([]string, 0, Days)
	for i := Days - 1; i >= 0; i-- {
		d := time.Date(ln.Year(), ln.Month(), ln.Day()-i, 12, 0, 0, 0, loc).Format("2006-01-02")
		accs[d] = &dayAcc{day: Day{Date: d}, sessions: map[string]bool{}, models: map[string]counts{}, projects: map[string]counts{}}
		order = append(order, d)
	}
	for k, c := range agg {
		a := accs[k.day]
		if a == nil {
			continue
		}
		a.day.Input += c.input
		a.day.Output += c.output
		a.day.CacheRead += c.cacheRead
		a.day.CacheCreation += c.cacheCreation
		a.day.Messages += c.messages
		a.sessions[k.session] = true
		m := a.models[k.model]
		m.add(c)
		a.models[k.model] = m
		p := a.projects[k.project]
		p.add(c)
		a.projects[k.project] = p
	}
	for _, d := range order {
		a := accs[d]
		a.day.Sessions = len(a.sessions)
		a.day.ByModel = topRows(a.models)
		a.day.ByProject = topRows(a.projects)
		resp.Days = append(resp.Days, a.day)
	}
	return resp
}

// topRows ordena por total desc (desempate por nombre), deja topN y suma el resto en "otros".
func topRows(m map[string]counts) []Row {
	rows := make([]Row, 0, len(m))
	for n, c := range m {
		rows = append(rows, Row{Name: n, Output: c.output, Total: c.total()})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		return rows[i].Name < rows[j].Name
	})
	if len(rows) <= topN {
		return rows
	}
	other := Row{Name: otherName}
	for _, r := range rows[topN:] {
		other.Output += r.Output
		other.Total += r.Total
	}
	return append(rows[:topN:topN], other)
}
