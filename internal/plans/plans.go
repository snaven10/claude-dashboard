// Package plans lee el avance de un plan desde los markdown de plans/ (DD-23).
// Solo lectura, sin ejecutar nada; los formatos de `**Estado:**` son los reales
// del workspace, así que el parser es tolerante.
package plans

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Progress es el avance de un plan. Mismos campos y tags que agents.Plan
// (el poller convierte directo).
type Progress struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	InProgress int    `json:"inProgress"`
	Blocked    int    `json:"blocked"`
	Pending    int    `json:"pending"`
	Skipped    int    `json:"skipped"`
	Unknown    int    `json:"unknown"`
}

const (
	statePending    = "pending"
	stateInProgress = "in_progress"
	stateBlocked    = "blocked"
	stateDone       = "done"
	stateSkipped    = "skipped"
	stateUnknown    = "unknown"

	maxStateLines = 40
)

var (
	planDirRe = regexp.MustCompile(`^PLAN-0*(\d+)(?:-|$)`)
	stateRe   = regexp.MustCompile(`\b(pending|pendiente|in_progress|blocked|done|skipped)\b`)
)

// parseStateLines devuelve el estado de una task: solo cuenta la PRIMERA línea
// con `**Estado:**` dentro de las primeras 40.
func parseStateLines(sc *bufio.Scanner) string {
	for i := 0; i < maxStateLines && sc.Scan(); i++ {
		_, rest, ok := strings.Cut(sc.Text(), "**Estado:**")
		if !ok {
			continue
		}
		m := stateRe.FindString(strings.ToLower(rest))
		switch m {
		case "pending", "pendiente":
			return statePending
		case "":
			return stateUnknown
		default:
			return m
		}
	}
	return stateUnknown
}

type fileEntry struct {
	mtime time.Time
	size  int64
	state string
}

var (
	mu    sync.Mutex
	cache = map[string]fileEntry{}
)

// taskState lee el estado de un archivo, cacheado por (mtime, size).
func taskState(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return stateUnknown
	}
	mu.Lock()
	e, ok := cache[path]
	mu.Unlock()
	if ok && e.size == st.Size() && e.mtime.Equal(st.ModTime()) {
		return e.state
	}
	f, err := os.Open(path)
	if err != nil {
		return stateUnknown
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	state := parseStateLines(sc)
	mu.Lock()
	cache[path] = fileEntry{mtime: st.ModTime(), size: st.Size(), state: state}
	mu.Unlock()
	return state
}

// Find busca plans/PLAN-<number>-* subiendo desde repoRoot hasta home (inclusive)
// y devuelve el avance. Nunca sale de home; si repoRoot no está bajo home solo
// mira repoRoot. nil si no hay plan.
func Find(number int, repoRoot, home string) *Progress {
	if number <= 0 || repoRoot == "" {
		return nil
	}
	dir := filepath.Clean(repoRoot)
	home = filepath.Clean(home)
	underHome := home != "" && home != "." && (dir == home || strings.HasPrefix(dir, home+string(filepath.Separator)))
	for {
		if p := inDir(number, dir); p != nil {
			return p
		}
		if !underHome || dir == home {
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

func inDir(number int, dir string) *Progress {
	entries, err := os.ReadDir(filepath.Join(dir, "plans"))
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m := planDirRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err != nil || n != number {
			continue
		}
		return progress(number, filepath.Join(dir, "plans", e.Name()))
	}
	return nil
}

func progress(number int, planPath string) *Progress {
	p := &Progress{Number: number, Name: filepath.Base(planPath), Path: planPath}
	tasks, _ := filepath.Glob(filepath.Join(planPath, "tasks", "*.md"))
	for _, t := range tasks {
		p.Total++
		switch taskState(t) {
		case stateDone:
			p.Done++
		case stateInProgress:
			p.InProgress++
		case stateBlocked:
			p.Blocked++
		case statePending:
			p.Pending++
		case stateSkipped:
			p.Skipped++
		default:
			p.Unknown++
		}
	}
	return p
}
