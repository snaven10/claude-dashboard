package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// PLAN-008 (DD-35): datos extra por sesión sacados del acumulador incremental.

const (
	maxPRs       = 5
	maxArtifacts = 5
	maxRepo      = 200
	maxTitle     = 200
	maxURL       = 2000
)

// PR es un pull/merge request linkeado desde la sesión.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Repo   string `json:"repo"`
}

// Artifact es un artifact de claude.ai linkeado desde la sesión.
type Artifact struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Lines son las líneas agregadas y quitadas acumuladas. Sin costo (DD-11).
type Lines struct {
	Added   int64 `json:"added"`
	Removed int64 `json:"removed"`
}

// Extras agrupa lo que se calcula fuera de la cola del transcript. Los campos
// vacíos son nil (las listas, vacías): "sin dato".
type Extras struct {
	PRs            []PR
	Artifacts      []Artifact
	Lines          *Lines
	PermissionMode string // "" = sin dato
	Subagents      *SubagentStats
}

var permissionModeRe = regexp.MustCompile(`^[A-Za-z]{1,32}$`)

// Marcadores baratos: una línea que no los contiene no se parsea para esto.
var (
	markPR         = []byte(`"type":"pr-link"`)
	markFrame      = []byte(`"type":"frame-link"`)
	markCost       = []byte(`"type":"cost-state"`)
	markPermission = []byte(`"type":"permission-mode"`)
)

// extraKind devuelve el tipo de registro extra de la línea, "" si no es ninguno.
func extraKind(raw []byte) string {
	switch {
	case bytes.Contains(raw, markPR):
		return "pr-link"
	case bytes.Contains(raw, markFrame):
		return "frame-link"
	case bytes.Contains(raw, markCost):
		return "cost-state"
	case bytes.Contains(raw, markPermission):
		return "permission-mode"
	}
	return ""
}

// extraLine tiene SOLO los campos que se usan: totalCostUSD no se declara a
// propósito, así nunca sale del paquete (DD-11).
type extraLine struct {
	Type            string `json:"type"`
	PRNumber        int    `json:"prNumber"`
	PRRepository    string `json:"prRepository"`
	PRURL           string `json:"prUrl"`
	FrameURL        string `json:"frameUrl"`
	Title           string `json:"title"`
	LinesAdded      *int64 `json:"totalLinesAdded"`
	LinesRemoved    *int64 `json:"totalLinesRemoved"`
	PermissionModeV string `json:"permissionMode"`
}

func (st *usageState) consumeExtra(kind string, raw []byte) {
	var l extraLine
	if json.Unmarshal(raw, &l) != nil || l.Type != kind {
		return
	}
	switch kind {
	case "pr-link":
		if l.PRNumber <= 0 || !strings.HasPrefix(l.PRURL, "https://") || len(l.PRURL) > maxURL {
			return
		}
		st.prs = pushDistinct(st.prs, PR{Number: l.PRNumber, URL: l.PRURL, Repo: truncate(oneLine(l.PRRepository), maxRepo)},
			func(p PR) string { return p.URL }, maxPRs)
	case "frame-link":
		if !strings.HasPrefix(l.FrameURL, "https://claude.ai/") || len(l.FrameURL) > maxURL {
			return
		}
		st.artifacts = pushDistinct(st.artifacts, Artifact{Title: truncate(oneLine(l.Title), maxTitle), URL: l.FrameURL},
			func(a Artifact) string { return a.URL }, maxArtifacts)
	case "cost-state":
		if l.LinesAdded == nil || l.LinesRemoved == nil || *l.LinesAdded < 0 || *l.LinesRemoved < 0 {
			return
		}
		st.lines = &Lines{Added: *l.LinesAdded, Removed: *l.LinesRemoved}
	case "permission-mode":
		// El último valor manda; si es raro, no hay dato (en vez de arrastrar uno viejo).
		if permissionModeRe.MatchString(l.PermissionModeV) {
			st.permissionMode = l.PermissionModeV
		} else {
			st.permissionMode = ""
		}
	}
}

// pushDistinct agrega v al final; si ya había uno con la misma clave, lo mueve
// (gana el más reciente). Conserva solo los últimos max.
func pushDistinct[T any](list []T, v T, key func(T) string, max int) []T {
	k := key(v)
	out := make([]T, 0, len(list)+1)
	for _, e := range list {
		if key(e) != k {
			out = append(out, e)
		}
	}
	out = append(out, v)
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// extrasView arma la vista sin subagentes; st.mu tiene que estar tomado.
func (st *usageState) extrasView() *Extras {
	e := &Extras{PermissionMode: st.permissionMode}
	e.PRs = append([]PR(nil), st.prs...)
	e.Artifacts = append([]Artifact(nil), st.artifacts...)
	if st.lines != nil {
		l := *st.lines
		e.Lines = &l
	}
	return e
}

// ExtrasFor devuelve PRs, artifacts, líneas, modo de permisos y subagentes en la
// MISMA pasada incremental que Usage (no relee si el archivo no creció). `now`
// es inyectable. Sin transcript → nil, nil.
func ExtrasFor(cwd, sessionID string, now time.Time) (*Extras, error) {
	root, err := projectsRoot()
	if err != nil {
		return nil, err
	}
	path := findPath(root, cwd, sessionID)
	if path == "" {
		return nil, nil
	}
	r, err := scan(path, now)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("extras de %s: %w", path, err)
	}
	e := r.extras
	e.Subagents = Subagents(filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents"), now)
	return e, nil
}
