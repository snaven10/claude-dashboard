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
	"strconv"
	"strings"
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
	Title          string      `json:"title"`
	LastPrompt     string      `json:"lastPrompt"`
	LastAction     *LastAction `json:"lastAction"`
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
	project := ""
	if r.Cwd != "" {
		project = filepath.Base(filepath.Clean(r.Cwd))
	}
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
	}
}
