package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Tokens son los totales de uso de una sesión (DD-9). Sin costo en dólares (DD-11).
type Tokens struct {
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cacheRead"`
	CacheCreation int64  `json:"cacheCreation"`
	Model         string `json:"model"`
}

// usageChunk es el tamaño de bloque de lectura; maxLine acota una línea que cruza bloques.
var usageChunk = 4 << 20 // var: los tests lo achican para cruzar bordes de bloque

const maxLine = 64 << 20

// usageBytesRead cuenta los bytes leídos por el acumulador (para tests).
var usageBytesRead atomic.Int64

// usageState es el acumulador de un archivo. Se lee de offset al final.
type usageState struct {
	mu            sync.Mutex
	offset        int64
	totals        Tokens
	lastMessageID string
	counted       bool // hubo al menos un mensaje con usage

	// PLAN-003 (DD-13, DD-14): contexto, compactaciones y cola para el ritmo.
	lastContext   int64
	compactions   int
	lastCompactAt int64 // ms; 0 = nunca
	lastPreTokens int64
	recent        []rateEntry // mensajes únicos de los últimos 10 min (respecto del más nuevo)
}

// rateEntry es el output de un mensaje del asistente con su timestamp (ms).
type rateEntry struct {
	at  int64
	out int64
}

const (
	rateKeep      = 10 * time.Minute // cuánto se guarda en la cola
	rateWindowMin = 5                // ventana del ritmo, en minutos
)

// Context es el contexto actual de la sesión y su historial de compactaciones (DD-13).
// LastCompactAt y LastPreTokens son nil si nunca compactó. El umbral `warn` lo aplica el server.
type Context struct {
	Current       int64  `json:"current"`
	Compactions   int    `json:"compactions"`
	LastCompactAt *int64 `json:"lastCompactAt"`
	LastPreTokens *int64 `json:"lastPreTokens"`
}

// Rate es la actividad de la ventana de 5 min (DD-14). El umbral `high` lo aplica el server.
type Rate struct {
	WindowMin int   `json:"windowMin"`
	Output    int64 `json:"output"`
	Calls     int   `json:"calls"`
}

// usageResult es la vista consistente del acumulador tras una pasada.
type usageResult struct {
	tokens  *Tokens
	context *Context
	rate    *Rate
}

var (
	usageMu     sync.Mutex
	usageStates = map[string]*usageState{}
)

func stateFor(path string) *usageState {
	usageMu.Lock()
	defer usageMu.Unlock()
	st, ok := usageStates[path]
	if !ok {
		st = &usageState{}
		usageStates[path] = st
	}
	return st
}

type usageLine struct {
	Type            string `json:"type"`
	Subtype         string `json:"subtype"`
	Timestamp       string `json:"timestamp"`
	CompactMetadata struct {
		PreTokens int64 `json:"preTokens"`
	} `json:"compactMetadata"`
	Message struct {
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

// Usage devuelve los tokens acumulados de la sesión. nil si no hay transcript
// o todavía no hay mensajes del asistente con usage. Lee solo lo nuevo desde el
// último ciclo; la primera lectura de un transcript grande es entera y se loguea.
func Usage(cwd, sessionID string) (*Tokens, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}
	path := findPath(root, cwd, sessionID)
	if path == "" {
		return nil, nil
	}
	r, err := scan(path, time.Now())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("tokens de %s: %w", path, err)
	}
	return r.tokens, nil
}

// Activity devuelve contexto y ritmo de la sesión en la MISMA pasada incremental
// que Usage (no relee el archivo). `now` es inyectable para testear la ventana.
// Cada valor es nil si no hay datos (sin transcript, sin usage, sin actividad reciente).
func Activity(cwd, sessionID string, now time.Time) (*Context, *Rate, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, nil, err
	}
	path := findPath(root, cwd, sessionID)
	if path == "" {
		return nil, nil, nil
	}
	r, err := scan(path, now)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("actividad de %s: %w", path, err)
	}
	return r.context, r.rate, nil
}

func scan(path string, now time.Time) (*usageResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()

	st := stateFor(path)
	st.mu.Lock()
	defer st.mu.Unlock()

	if size < st.offset { // truncado o rotado
		st.offset, st.totals, st.lastMessageID, st.counted = 0, Tokens{}, "", false
		st.lastContext, st.compactions, st.lastCompactAt, st.lastPreTokens, st.recent = 0, 0, 0, 0, nil
	}
	if size > st.offset {
		first := st.offset == 0
		start, leidos := time.Now(), int64(0)
		// Por bloques: hay transcripts de >100 MB y la primera lectura es entera;
		// con un solo buffer del tamaño del archivo, 8 en paralelo eran cientos de MB.
		buf := make([]byte, usageChunk)
		var resto []byte // línea incompleta que cruza el borde del bloque
		for pos := st.offset; pos < size; {
			n, err := f.ReadAt(buf[:min(int64(len(buf)), size-pos)], pos)
			if err != nil && !errors.Is(err, io.EOF) {
				return nil, err
			}
			if n == 0 {
				break
			}
			usageBytesRead.Add(int64(n))
			leidos += int64(n)
			pos += int64(n)
			data := append(resto, buf[:n]...)
			// solo líneas completas: lo que sigue al último '\n' queda para después
			end := bytes.LastIndexByte(data, '\n')
			if end < 0 {
				resto = data
				continue
			}
			st.consume(data[:end+1])
			st.offset += int64(end + 1)
			resto = append([]byte(nil), data[end+1:]...)
			if int64(len(resto)) > maxLine { // línea patológica: no acumular sin límite
				return nil, fmt.Errorf("línea de más de %d bytes en %s", maxLine, path)
			}
		}
		if first && leidos > initialWindow {
			log.Printf("transcript: primera lectura completa de %s: %d bytes en %s", path, leidos, time.Since(start).Round(time.Millisecond))
		}
	}
	return st.result(now), nil
}

// result arma la vista; st.mu tiene que estar tomado.
func (st *usageState) result(now time.Time) *usageResult {
	res := &usageResult{}
	if !st.counted {
		return res
	}
	t := st.totals
	res.tokens = &t
	c := &Context{Current: st.lastContext, Compactions: st.compactions}
	if st.lastCompactAt > 0 {
		at := st.lastCompactAt
		c.LastCompactAt = &at
	}
	if st.compactions > 0 {
		pre := st.lastPreTokens
		c.LastPreTokens = &pre
	}
	res.context = c

	from := now.Add(-rateWindowMin * time.Minute).UnixMilli()
	to := now.UnixMilli()
	r := &Rate{WindowMin: rateWindowMin}
	for _, e := range st.recent {
		if e.at > from && e.at <= to {
			r.Output += e.out
			r.Calls++
		}
	}
	if r.Calls > 0 {
		res.rate = r
	}
	return res
}

// pruneRecent descarta lo que quedó más de 10 min detrás de la entrada más nueva.
func (st *usageState) pruneRecent() {
	var newest int64
	for _, e := range st.recent {
		newest = max(newest, e.at)
	}
	cut := newest - rateKeep.Milliseconds()
	i := 0
	for i < len(st.recent) && st.recent[i].at < cut {
		i++
	}
	if i > 0 {
		st.recent = append([]rateEntry(nil), st.recent[i:]...)
	}
}

func parseTS(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func (st *usageState) consume(data []byte) {
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		raw := data[:i]
		data = data[i+1:]
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		var l usageLine
		if json.Unmarshal(raw, &l) != nil {
			continue
		}
		if l.Type == "system" && l.Subtype == "compact_boundary" {
			st.compactions++
			st.lastPreTokens = l.CompactMetadata.PreTokens
			if at := parseTS(l.Timestamp); at > 0 {
				st.lastCompactAt = at
			}
			continue
		}
		if l.Type != "assistant" || l.Message.Usage == nil {
			continue
		}
		if m := l.Message.Model; m != "" {
			st.totals.Model = m
		}
		id := l.Message.ID
		if id != "" && id == st.lastMessageID { // línea repetida del mismo mensaje
			continue
		}
		u := l.Message.Usage
		st.totals.Input += u.Input
		st.totals.Output += u.Output
		st.totals.CacheRead += u.CacheRead
		st.totals.CacheCreation += u.CacheCreation
		st.lastMessageID = id
		st.counted = true
		st.lastContext = u.Input + u.CacheRead + u.CacheCreation
		if at := parseTS(l.Timestamp); at > 0 {
			st.recent = append(st.recent, rateEntry{at: at, out: u.Output})
			st.pruneRecent()
		}
	}
}
