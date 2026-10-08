// Package transcript resume el detalle de una sesión (F3 del design) leyendo
// solo la cola de ~/.claude/projects/<slug>/<sessionId>.jsonl.
//
// F3 es un formato interno sin contrato (DD-1): las líneas que no parsean se
// saltan y un archivo inexistente produce un Detail vacío, nunca un error fatal.
package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	initialWindow = 256 << 10 // 256 KB
	maxWindow     = 2 << 20   // 2 MB
	maxPrompt     = 500
	maxAction     = 200
)

// Action es la última acción de la sesión. Misma forma que agents.LastAction
// (kind "tool" | "text"); se define acá para no acoplar los paquetes.
type Action struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Detail es el resumen de una sesión sacado de su transcript.
type Detail struct {
	Title          string  `json:"title"`
	LastPrompt     string  `json:"lastPrompt"`
	LastAction     *Action `json:"lastAction"`
	LastActivityAt int64   `json:"lastActivityAt"` // ms epoch; 0 si no hay
}

// Root es el directorio de proyectos. Vacío = ~/.claude/projects.
// Se expone para tests.
var Root string

// Contadores de E/S, solo para verificar en tests que se lee poco y que el
// cache evita releer.
var (
	bytesRead atomic.Int64
	readCalls atomic.Int64
)

type cacheKey struct {
	size  int64
	mtime time.Time
}

type cacheEntry struct {
	key    cacheKey
	detail Detail
}

var (
	mu    sync.Mutex
	cache = map[string]cacheEntry{}
)

func projectsRoot() (string, error) {
	if Root != "" {
		return Root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolviendo home: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// slugFor replica el slug de Claude Code: "/" y "." pasan a "-".
func slugFor(cwd string) string {
	return strings.NewReplacer("/", "-", ".", "-").Replace(cwd)
}

// findPath ubica el .jsonl: primero por slug, si no por glob. "" si no existe.
func findPath(root, cwd, sessionID string) string {
	if sessionID == "" || strings.ContainsAny(sessionID, `/\`) {
		return ""
	}
	name := sessionID + ".jsonl"
	if cwd != "" {
		p := filepath.Join(root, slugFor(cwd), name)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	matches, err := filepath.Glob(filepath.Join(root, "*", name))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[0]
}

// Summary devuelve el detalle de la sesión. Archivo inexistente → Detail vacío
// y error nil. Cachea por (path, size, mtime).
func Summary(cwd, sessionID string) (Detail, error) {
	root, err := projectsRoot()
	if err != nil {
		return Detail{}, err
	}
	path := findPath(root, cwd, sessionID)
	if path == "" {
		return Detail{}, nil
	}
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Detail{}, nil
		}
		return Detail{}, fmt.Errorf("stat %s: %w", path, err)
	}
	key := cacheKey{size: st.Size(), mtime: st.ModTime()}

	mu.Lock()
	if e, ok := cache[path]; ok && e.key == key {
		mu.Unlock()
		return e.detail, nil
	}
	mu.Unlock()

	d, err := summarizeFile(path, st.Size())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Detail{}, nil
		}
		return Detail{}, fmt.Errorf("leyendo %s: %w", path, err)
	}

	mu.Lock()
	cache[path] = cacheEntry{key: key, detail: d}
	mu.Unlock()
	return d, nil
}

// summarizeFile lee la cola, ampliando la ventana hasta maxWindow mientras no
// aparezca un prompt real. Al ampliar solo se lee el tramo nuevo (el anterior
// se reusa), así el total leído nunca pasa de maxWindow.
func summarizeFile(path string, size int64) (Detail, error) {
	f, err := os.Open(path)
	if err != nil {
		return Detail{}, err
	}
	defer f.Close()

	var buf []byte // bytes [size-len(buf), size)
	window := int64(initialWindow)
	for {
		if window > size {
			window = size
		}
		have := int64(len(buf))
		if window > have {
			chunk := make([]byte, window-have)
			n, err := f.ReadAt(chunk, size-window)
			if err != nil && !errors.Is(err, io.EOF) {
				return Detail{}, err
			}
			bytesRead.Add(int64(n))
			readCalls.Add(1)
			buf = append(chunk[:n], buf...)
		}

		data := buf
		if int64(len(buf)) < size { // empieza a mitad de archivo: primera línea parcial
			if i := bytes.IndexByte(data, '\n'); i >= 0 {
				data = data[i+1:]
			} else {
				data = nil
			}
		}
		d, hasRealUser := parse(data)
		if hasRealUser || window >= size || window >= maxWindow {
			return d, nil
		}
		window *= 2
		if window > maxWindow {
			window = maxWindow
		}
	}
}

type line struct {
	Type       string `json:"type"`
	Timestamp  string `json:"timestamp"`
	AITitle    string `json:"aiTitle"`
	LastPrompt string `json:"lastPrompt"`
	Message    struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type  string                     `json:"type"`
	Text  string                     `json:"text"`
	Name  string                     `json:"name"`
	Input map[string]json.RawMessage `json:"input"`
}

// parse recorre las líneas (tolerante) y extrae el detalle.
func parse(data []byte) (Detail, bool) {
	var d Detail
	var fallbackPrompt string
	var action *Action
	var realUser bool

	lines := bytes.Split(data, []byte{'\n'})
	for i := len(lines) - 1; i >= 0; i-- { // de atrás hacia adelante: lo último gana
		raw := bytes.TrimSpace(lines[i])
		if len(raw) == 0 {
			continue
		}
		var l line
		if err := json.Unmarshal(raw, &l); err != nil {
			continue
		}
		if ts := parseTime(l.Timestamp); ts > d.LastActivityAt {
			d.LastActivityAt = ts
		}
		switch l.Type {
		case "ai-title":
			if d.Title == "" {
				d.Title = strings.TrimSpace(l.AITitle)
			}
		case "last-prompt":
			if fallbackPrompt == "" {
				fallbackPrompt = strings.TrimSpace(l.LastPrompt)
			}
		case "user":
			if !realUser {
				if text, ok := realUserText(l.Message.Content); ok {
					d.LastPrompt = truncate(text, maxPrompt)
					realUser = true
				}
			}
		case "assistant":
			if action == nil {
				action = assistantAction(l.Message.Content)
			}
		}
	}
	if !realUser && fallbackPrompt != "" {
		d.LastPrompt = truncate(fallbackPrompt, maxPrompt)
	}
	d.LastAction = action
	return d, realUser
}

func parseTime(s string) int64 {
	if s == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

// realUserText devuelve el texto si el mensaje user es un prompt real
// (string, o bloques de texto sin tool_result).
func realUserText(content json.RawMessage) (string, bool) {
	if len(content) == 0 {
		return "", false
	}
	var s string
	if json.Unmarshal(content, &s) == nil {
		s = strings.TrimSpace(s)
		return s, s != ""
	}
	var blocks []block
	if json.Unmarshal(content, &blocks) != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "tool_result":
			return "", false
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" {
				parts = append(parts, t)
			}
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n"), true
}

// assistantAction toma el último tool_use del mensaje; si no hay, el último
// bloque de texto. nil si el mensaje no aporta nada.
func assistantAction(content json.RawMessage) *Action {
	var blocks []block
	if json.Unmarshal(content, &blocks) != nil {
		return nil
	}
	var text string
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		if b.Type == "tool_use" && b.Name != "" {
			t := b.Name
			if sum := summarizeInput(b.Input); sum != "" {
				t += ": " + sum
			}
			return &Action{Kind: "tool", Text: truncate(t, maxAction)}
		}
		if b.Type == "text" && text == "" {
			text = strings.TrimSpace(b.Text)
		}
	}
	if text != "" {
		return &Action{Kind: "text", Text: truncate(text, maxAction)}
	}
	return nil
}

var inputKeys = []string{"command", "file_path", "path", "pattern", "url", "query", "description", "prompt"}

func summarizeInput(in map[string]json.RawMessage) string {
	str := func(k string) string {
		var s string
		if json.Unmarshal(in[k], &s) == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	for _, k := range inputKeys {
		if s := str(k); s != "" {
			return oneLine(s)
		}
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s := str(k); s != "" {
			return oneLine(s)
		}
	}
	return ""
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate corta a n runas.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
