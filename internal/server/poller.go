package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/orq"
	"claude-dashboard/internal/plans"
	"claude-dashboard/internal/transcript"
)

// summaryConcurrency limita los resúmenes de transcript en paralelo.
const summaryConcurrency = 8

// Snapshot es el payload de GET /api/sessions y de cada evento SSE `snapshot`.
type Snapshot struct {
	GeneratedAt int64            `json:"generatedAt"`
	Sessions    []agents.Session `json:"sessions"`
}

// build arma el snapshot: F1(+F2) y después el detalle F3 por sesión, con
// límite de paralelismo. Un fallo de F3 degrada la sesión, no el panel (DD-1).
func (s *Server) build(ctx context.Context) ([]agents.Session, error) {
	sessions, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []agents.Session{}
	}
	jobs := s.loadOrq()
	now := s.now()
	if s.Summary == nil && s.Usage == nil && s.Activity == nil && s.Errors == nil {
		for i := range sessions {
			s.enrich(&sessions[i], jobs, now)
		}
		return sessions, nil
	}
	sem := make(chan struct{}, summaryConcurrency)
	var wg sync.WaitGroup
	for i := range sessions {
		wg.Add(1)
		sem <- struct{}{}
		go func(ss *agents.Session) {
			defer wg.Done()
			defer func() { <-sem }()
			if s.Summary != nil {
				if d, err := s.Summary(ss.Cwd, ss.SessionID); err != nil {
					s.logf("transcript %s: %v", ss.SessionID, err)
				} else {
					applyDetail(ss, d)
				}
			}
			// Tokens independientes del resumen: un fallo de uno no apaga el otro.
			if s.Usage != nil {
				if t, err := s.Usage(ss.Cwd, ss.SessionID); err != nil {
					s.logf("tokens %s: %v", ss.SessionID, err)
				} else {
					ss.Tokens = (*agents.Tokens)(t) // mismos campos: conversión directa
				}
			}
			// Contexto y ritmo: misma pasada incremental del acumulador (no relee el archivo).
			if s.Activity != nil {
				if c, r, err := s.Activity(ss.Cwd, ss.SessionID, now); err != nil {
					s.logf("actividad %s: %v", ss.SessionID, err)
				} else {
					s.applyActivity(ss, c, r)
				}
			}
			// Errores de la ventana de 10 min: misma pasada incremental.
			if s.Errors != nil {
				if e, err := s.Errors(ss.Cwd, ss.SessionID, now); err != nil {
					s.logf("errores %s: %v", ss.SessionID, err)
				} else if e != nil {
					ss.Errors = &agents.Errors{
						WindowMin: e.WindowMin, Count: e.Count, LastAt: e.LastAt,
						High: s.ErrorsWarn > 0 && e.Count >= s.ErrorsWarn,
					}
				}
			}
		}(&sessions[i])
	}
	wg.Wait()
	for i := range sessions {
		s.enrich(&sessions[i], jobs, now)
	}
	return sessions, nil
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// loadOrq lee jobs.json; un fallo se loguea y degrada a "sin orq" (DD-10).
func (s *Server) loadOrq() map[string]orq.Job {
	if s.Orq == nil {
		return nil
	}
	jobs, err := s.Orq()
	if err != nil {
		s.logf("orq: %v", err)
	}
	return jobs
}

// enrich completa orq y stale (DD-8, DD-10). Stale depende de lastActivityAt
// y no de un contador: el hash solo cambia cuando el valor cambia.
func (s *Server) enrich(ss *agents.Session, jobs map[string]orq.Job, now time.Time) {
	if ss.Name != "" {
		if j, ok := jobs[ss.Name]; ok {
			o := &agents.Orq{Job: j.ID, Specialist: j.Specialist, WS: j.WS}
			if plan, task, ok := j.PlanTask(); ok {
				o.Plan, o.Task = &plan, &task
			}
			ss.Orq = o
		}
	}
	ss.Branch = agents.Branch(ss.Cwd)
	ss.Plan = findPlan(ss)
	ss.Stale = s.StaleAfter > 0 && ss.Status == agents.StatusBusy && ss.LastActivityAt > 0 &&
		now.UnixMilli()-ss.LastActivityAt > s.StaleAfter.Milliseconds()
}

var branchPlanRe = regexp.MustCompile(`(?i)plan-?(\d+)`)

// planNumber: primero el key de orq (p<N>-task<M>), si no la rama (DD-23).
func planNumber(ss *agents.Session) int {
	if ss.Orq != nil && ss.Orq.Plan != nil {
		if n, err := strconv.Atoi(*ss.Orq.Plan); err == nil && n > 0 {
			return n
		}
	}
	if ss.Branch != nil {
		if m := branchPlanRe.FindStringSubmatch(*ss.Branch); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}

// findPlan devuelve el avance del plan de la sesión, o nil sin dato.
func findPlan(ss *agents.Session) *agents.Plan {
	n := planNumber(ss)
	if n == 0 {
		return nil
	}
	root := agents.RepoRoot(ss.Cwd)
	home, err := os.UserHomeDir()
	if root == "" || err != nil {
		return nil
	}
	return (*agents.Plan)(plans.Find(n, root, home)) // mismos campos: conversión directa
}

// applyActivity pasa contexto y ritmo al contrato aplicando los umbrales (DD-13, DD-14).
// El hash solo cambia si cambian los números o `warn`/`high`: no hay contadores de tiempo.
func (s *Server) applyActivity(ss *agents.Session, c *transcript.Context, r *transcript.Rate) {
	if c != nil {
		ss.Context = &agents.Context{
			Current: c.Current, Compactions: c.Compactions,
			LastCompactAt: c.LastCompactAt, LastPreTokens: c.LastPreTokens,
			Warn: s.ContextWarn > 0 && c.Current >= s.ContextWarn,
		}
	}
	if r != nil {
		ss.Rate = &agents.Rate{
			WindowMin: r.WindowMin, Output: r.Output, Calls: r.Calls,
			High: (s.RateOutput > 0 && r.Output > s.RateOutput) || (s.RateCalls > 0 && r.Calls > s.RateCalls),
		}
	}
}

func applyDetail(ss *agents.Session, d transcript.Detail) {
	ss.Title = d.Title
	ss.LastPrompt = d.LastPrompt
	if d.Ask != nil {
		ss.Ask = &agents.Ask{Kind: d.Ask.Kind, Text: d.Ask.Text, More: d.Ask.More}
	}
	if d.LastAction != nil {
		ss.LastAction = &agents.LastAction{Kind: d.LastAction.Kind, Text: d.LastAction.Text}
	}
	if d.LastActivityAt > ss.LastActivityAt {
		ss.LastActivityAt = d.LastActivityAt
	}
}

// Refresh ejecuta un ciclo de polling. Si el contenido cambió (hash de las
// sesiones, sin generatedAt) actualiza el snapshot y lo emite a los
// suscriptores. Devuelve si hubo cambio. Ante error de F1 conserva el último
// snapshot bueno.
func (s *Server) Refresh(ctx context.Context) (bool, error) {
	start := time.Now()
	sessions, err := s.build(ctx)
	if err != nil {
		return false, fmt.Errorf("armando snapshot: %w", err)
	}
	body, err := json.Marshal(sessions)
	if err != nil {
		return false, fmt.Errorf("serializando sesiones: %w", err)
	}
	sum := sha256.Sum256(body)

	s.mu.Lock()
	if s.have && sum == s.hash {
		s.mu.Unlock()
		s.logf("poll: %d sesiones, sin cambios (%s)", len(sessions), time.Since(start).Round(time.Millisecond))
		return false, nil
	}
	payload, err := json.Marshal(Snapshot{GeneratedAt: time.Now().UnixMilli(), Sessions: sessions})
	if err != nil {
		s.mu.Unlock()
		return false, fmt.Errorf("serializando snapshot: %w", err)
	}
	s.hash, s.have, s.current, s.sessions = sum, true, payload, sessions
	s.mu.Unlock()

	s.broadcast(payload)
	s.logf("poll: %d sesiones, cambió (%s)", len(sessions), time.Since(start).Round(time.Millisecond))
	return true, nil
}

// Poll corre el único poller hasta que ctx se cancele.
func (s *Server) Poll(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := s.Refresh(ctx); err != nil && ctx.Err() == nil {
			s.logf("poll: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
