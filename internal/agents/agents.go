// Package agents recolecta las sesiones de Claude Code (F1 + F2 del design).
//
// F1 (`claude agents --json --all`) es la fuente de verdad del estado (DD-1).
// F2 (~/.claude/sessions/<pid>.json) solo enriquece; si falta o no parsea se ignora.
package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Estados normalizados (DD-6).
const (
	StatusBusy    = "busy"
	StatusWaiting = "waiting"
	StatusIdle    = "idle"
	StatusBlocked = "blocked"
	StatusDone    = "done"
)

const runTimeout = 10 * time.Second

// LastAction es la última acción de la sesión (la llena el paquete transcript).
type LastAction struct {
	Kind string `json:"kind"` // "tool" | "text"
	Text string `json:"text"`
}

// Session es una sesión normalizada según el contrato de GET /api/sessions.
type Session struct {
	ID             string      `json:"id"`
	SessionID      string      `json:"sessionId"`
	PID            int         `json:"pid"`
	Kind           string      `json:"kind"`
	Name           string      `json:"name"`
	Cwd            string      `json:"cwd"`
	Project        string      `json:"project"`
	Status         string      `json:"status"`
	WaitingFor     *string     `json:"waitingFor"`
	StartedAt      int64       `json:"startedAt"`
	LastActivityAt int64       `json:"lastActivityAt"`
	Remote         bool        `json:"remote"`
	RemoteURL      *string     `json:"remoteUrl"`
	Title          string      `json:"title"`
	LastPrompt     string      `json:"lastPrompt"`
	LastAction     *LastAction `json:"lastAction"`
	Stale          bool        `json:"stale"`
	Orq            *Orq        `json:"orq"`
	Tokens         *Tokens     `json:"tokens"`
	Context        *Context    `json:"context"`
	Rate           *Rate       `json:"rate"`
}

// Context es el contexto actual y las compactaciones (DD-13). Sin porcentaje: el
// tamaño de ventana no está en ninguna fuente.
type Context struct {
	Current       int64  `json:"current"`
	Compactions   int    `json:"compactions"`
	LastCompactAt *int64 `json:"lastCompactAt"`
	LastPreTokens *int64 `json:"lastPreTokens"`
	Warn          bool   `json:"warn"`
}

// Rate es la actividad de tokens en la ventana de 5 min (DD-14).
type Rate struct {
	WindowMin int   `json:"windowMin"`
	Output    int64 `json:"output"`
	Calls     int   `json:"calls"`
	High      bool  `json:"high"`
}

// Orq identifica a la sesión como worker de orq (DD-10). Plan y Task son nil
// si el key del job no matchea p<N>-task<M>.
type Orq struct {
	Job        string  `json:"job"`
	Plan       *string `json:"plan"`
	Task       *string `json:"task"`
	Specialist string  `json:"specialist"`
	WS         string  `json:"ws"`
}

// Tokens son los totales de uso de la sesión (DD-9). Sin costo (DD-11).
type Tokens struct {
	Input         int64  `json:"input"`
	Output        int64  `json:"output"`
	CacheRead     int64  `json:"cacheRead"`
	CacheCreation int64  `json:"cacheCreation"`
	Model         string `json:"model"`
}

// bridgeIDRe es el formato visto de bridgeSessionId (DD-18). No está documentado:
// se valida estricto porque el valor termina en un href y en un QR.
var bridgeIDRe = regexp.MustCompile(`^session_[A-Za-z0-9]{10,64}$`)

// remoteURL devuelve el link de claude.ai para un bridgeSessionId válido; si no, nil.
func remoteURL(bridgeID string) *string {
	if !bridgeIDRe.MatchString(bridgeID) {
		return nil
	}
	u := "https://claude.ai/code/" + bridgeID
	return &u
}

// rawAgent es una entrada cruda de F1.
type rawAgent struct {
	ID         string  `json:"id"`
	SessionID  string  `json:"sessionId"`
	PID        int     `json:"pid"`
	Cwd        string  `json:"cwd"`
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	StartedAt  int64   `json:"startedAt"`
	Status     string  `json:"status"`
	WaitingFor *string `json:"waitingFor"`
	State      string  `json:"state"`
}

// rawSession es el subconjunto de F2 que nos interesa.
type rawSession struct {
	StatusUpdatedAt int64  `json:"statusUpdatedAt"`
	BridgeSessionID string `json:"bridgeSessionId"`
}

// Runner ejecuta `claude agents --json --all` y devuelve el stdout.
type Runner func(ctx context.Context) ([]byte, error)

// Collector junta F1 y F2. Las dependencias externas se inyectan como función.
type Collector struct {
	Run         Runner
	SessionsDir string // directorio de F2; "" desactiva el enriquecimiento
}

// List usa el runner real (`claude`) y ~/.claude/sessions.
func List(ctx context.Context) ([]Session, error) {
	c := Collector{Run: runClaude}
	if home, err := os.UserHomeDir(); err == nil {
		c.SessionsDir = filepath.Join(home, ".claude", "sessions")
	}
	return c.List(ctx)
}

// List devuelve las sesiones normalizadas.
func (c Collector) List(ctx context.Context) ([]Session, error) {
	if c.Run == nil {
		return nil, errors.New("agents: runner no configurado")
	}
	out, err := c.Run(ctx)
	if err != nil {
		return nil, err
	}
	var raws []rawAgent
	if err := json.Unmarshal(bytes.TrimSpace(out), &raws); err != nil {
		return nil, fmt.Errorf("parseando salida de claude agents: %w", err)
	}
	sessions := make([]Session, 0, len(raws))
	for _, r := range raws {
		var f2 rawSession
		if c.SessionsDir != "" && r.PID > 0 {
			f2 = readF2(c.SessionsDir, r.PID)
		}
		sessions = append(sessions, normalize(r, f2))
	}
	return sessions, nil
}

func runClaude(ctx context.Context) ([]byte, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("no se encontró el binario 'claude' en PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "agents", "--json", "--all")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("claude agents: %w", ctx.Err())
		}
		return nil, fmt.Errorf("ejecutando claude agents: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// readF2 lee <dir>/<pid>.json; cualquier fallo devuelve el valor cero.
func readF2(dir string, pid int) rawSession {
	var s rawSession
	data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(pid)+".json"))
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return rawSession{}
	}
	return s
}

// finishedStates son los `state` de F1 que indican una background terminada.
var finishedStates = map[string]bool{
	"done": true, "completed": true, "complete": true, "finished": true,
	"failed": true, "error": true, "stopped": true, "exited": true,
	"cancelled": true, "canceled": true, "killed": true,
}

// normalizeStatus aplica DD-6.
func normalizeStatus(r rawAgent) string {
	if r.Kind != "background" {
		if r.Status == "" {
			return StatusIdle
		}
		return r.Status
	}
	state := strings.ToLower(r.State)
	switch {
	case state == StatusBlocked:
		return StatusBlocked
	case finishedStates[state]:
		return StatusDone
	case state == "working" || state == "running" || state == "busy":
		return StatusBusy
	case r.Status != "":
		return r.Status
	case state == "":
		return StatusDone
	}
	return StatusBusy
}

// projectCache evita re-stat/re-read de .git en cada poll (3 s por sesión).
var projectCache sync.Map // cwd -> string

// projectName devuelve el nombre del repo al que pertenece cwd (no del
// worktree). Sin ejecutar git: sube buscando .git; si es directorio, repo =
// su padre; si es archivo "gitdir: …/.git/worktrees/x", repo = lo previo a
// "/.git/worktrees/". Ante cualquier fallo, basename(cwd).
func projectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	if v, ok := projectCache.Load(cwd); ok {
		return v.(string)
	}
	p := resolveProject(cwd)
	projectCache.Store(cwd, p)
	return p
}

func resolveProject(cwd string) string {
	clean := filepath.Clean(cwd)
	fallback := filepath.Base(clean)
	for dir := clean; ; {
		gitPath := filepath.Join(dir, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			if fi.IsDir() {
				return filepath.Base(dir)
			}
			data, err := os.ReadFile(gitPath)
			if err != nil {
				return fallback
			}
			line := strings.TrimSpace(string(data))
			target, ok := strings.CutPrefix(line, "gitdir:")
			if !ok {
				return fallback
			}
			target = strings.TrimSpace(target)
			if !filepath.IsAbs(target) {
				target = filepath.Join(dir, target)
			}
			target = filepath.ToSlash(filepath.Clean(target))
			if i := strings.Index(target, "/.git/worktrees/"); i > 0 {
				return filepath.Base(target[:i])
			}
			return fallback
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fallback
		}
		dir = parent
	}
}

func normalize(r rawAgent, f2 rawSession) Session {
	id := r.ID
	if id == "" {
		id = r.SessionID
		if len(id) > 8 {
			id = id[:8]
		}
	}
	last := r.StartedAt
	if f2.StatusUpdatedAt > 0 {
		last = f2.StatusUpdatedAt
	}
	project := projectName(r.Cwd)
	return Session{
		ID:             id,
		SessionID:      r.SessionID,
		PID:            r.PID,
		Kind:           r.Kind,
		Name:           r.Name,
		Cwd:            r.Cwd,
		Project:        project,
		Status:         normalizeStatus(r),
		WaitingFor:     r.WaitingFor,
		StartedAt:      r.StartedAt,
		LastActivityAt: last,
		Remote:         f2.BridgeSessionID != "",
		RemoteURL:      remoteURL(f2.BridgeSessionID),
	}
}
