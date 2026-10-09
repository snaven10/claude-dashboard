package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/orq"
	"claude-dashboard/internal/transcript"
)

func fakeSessions(n int) []agents.Session {
	out := make([]agents.Session, n)
	for i := range out {
		out[i] = agents.Session{
			ID: fmt.Sprintf("id%02d", i), SessionID: fmt.Sprintf("sess-%02d", i), PID: 1000 + i,
			Kind: "interactive", Name: "demo", Cwd: "/work/demo", Project: "demo",
			Status: "busy", StartedAt: 1000, LastActivityAt: 2000,
		}
	}
	return out
}

func newTestServer(list func(context.Context) ([]agents.Session, error)) *Server {
	return &Server{
		List: list,
		Summary: func(cwd, id string) (transcript.Detail, error) {
			return transcript.Detail{
				Title: "T " + id, LastPrompt: "<b>hola</b>",
				LastAction:     &transcript.Action{Kind: "tool", Text: "Bash: git status"},
				LastActivityAt: 3000,
			}, nil
		},
		Static:    fstest.MapFS{"index.html": {Data: []byte("<html>ok</html>")}},
		Heartbeat: 50 * time.Millisecond,
		Logf:      func(string, ...any) {},
	}
}

func TestSessionsContract(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(2), nil })
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("status=%d ct=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var snap map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatal(err)
	}
	if _, ok := snap["generatedAt"]; !ok {
		t.Fatal("falta generatedAt")
	}
	var sessions []map[string]any
	if err := json.Unmarshal(snap["sessions"], &sessions); err != nil || len(sessions) != 2 {
		t.Fatalf("sessions: %v %d", err, len(sessions))
	}
	for _, k := range []string{"id", "sessionId", "pid", "kind", "name", "cwd", "project", "status",
		"waitingFor", "startedAt", "lastActivityAt", "remote", "title", "lastPrompt", "lastAction"} {
		if _, ok := sessions[0][k]; !ok {
			t.Errorf("falta campo %q", k)
		}
	}
	if sessions[0]["title"] != "T sess-00" || sessions[0]["lastActivityAt"].(float64) != 3000 {
		t.Errorf("detalle no aplicado: %v", sessions[0])
	}
}

func TestStaticAndListError(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return nil, errors.New("sin claude") })
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	r, _ := http.Get(ts.URL + "/")
	if r.StatusCode != 200 {
		t.Errorf("/ = %d", r.StatusCode)
	}
	r.Body.Close()
	r, _ = http.Get(ts.URL + "/api/sessions")
	if r.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("sin claude debería dar 503, dio %d", r.StatusCode)
	}
	r.Body.Close()
}

func TestRefreshEmitsOnlyOnChange(t *testing.T) {
	var n atomic.Int32
	s := newTestServer(func(context.Context) ([]agents.Session, error) {
		ss := fakeSessions(1)
		if n.Add(1) >= 3 {
			ss[0].Status = "waiting"
		}
		return ss, nil
	})
	ch, _ := s.subscribe()
	var got []bool
	for i := 0; i < 4; i++ {
		c, err := s.Refresh(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	want := []bool{true, false, true, false}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cambios=%v want %v", got, want)
		}
	}
	if len(ch) != 2 {
		t.Fatalf("eventos emitidos=%d, want 2", len(ch))
	}
}

func TestListErrorKeepsLastSnapshot(t *testing.T) {
	var fail atomic.Bool
	s := newTestServer(func(context.Context) ([]agents.Session, error) {
		if fail.Load() {
			return nil, errors.New("boom")
		}
		return fakeSessions(1), nil
	})
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := string(s.snapshot())
	fail.Store(true)
	if _, err := s.Refresh(context.Background()); err == nil {
		t.Fatal("esperaba error")
	}
	if string(s.snapshot()) != before {
		t.Fatal("el snapshot cambió tras un error")
	}
}

// readEvent lee un evento `snapshot` (ignora comentarios/heartbeats).
func readEvent(t *testing.T, br *bufio.Reader) string {
	t.Helper()
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("leyendo SSE: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		}
	}
}

func TestSSETwoClientsAndSlowOneDoesNotBlock(t *testing.T) {
	var gen atomic.Int32
	s := newTestServer(func(context.Context) ([]agents.Session, error) {
		ss := fakeSessions(1)
		ss[0].PID = int(gen.Load())
		return ss, nil
	})
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	open := func() (*bufio.Reader, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/events", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
			t.Fatalf("content-type=%s", ct)
		}
		return bufio.NewReader(resp.Body), func() { cancel(); resp.Body.Close() }
	}
	a, closeA := open()
	b, closeB := open()
	defer closeA()
	defer closeB()

	ea, eb := readEvent(t, a), readEvent(t, b)
	if ea != eb || ea != string(s.snapshot()) {
		t.Fatalf("snapshot inicial distinto:\n%s\n%s", ea, eb)
	}

	// Suscriptor colgado: nunca lee su canal.
	stuck, _ := s.subscribe()

	for i := 1; i <= 10; i++ {
		gen.Store(int32(i))
		if _, err := s.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		done := make(chan [2]string, 1)
		go func() { done <- [2]string{readEvent(t, a), readEvent(t, b)} }()
		select {
		case got := <-done:
			if got[0] != got[1] || !strings.Contains(got[0], fmt.Sprintf(`"pid":%d`, i)) {
				t.Fatalf("ciclo %d: eventos distintos o viejos: %v", i, got)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("ciclo %d: el cliente sano quedó bloqueado", i)
		}
	}

	// El colgado fue descartado (canal cerrado tras vaciar el buffer).
	n := 0
	for range stuck {
		n++
	}
	if n != subBuffer {
		t.Errorf("el colgado tenía %d eventos en buffer, want %d", n, subBuffer)
	}
}

func TestSSEHeartbeat(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)
	readEvent(t, br)
	deadline := time.After(2 * time.Second)
	got := make(chan bool, 1)
	go func() {
		for {
			l, err := br.ReadString('\n')
			if err != nil {
				got <- false
				return
			}
			if strings.HasPrefix(l, ": ping") {
				got <- true
				return
			}
		}
	}()
	select {
	case ok := <-got:
		if !ok {
			t.Fatal("stream cerrado sin heartbeat")
		}
	case <-deadline:
		t.Fatal("sin heartbeat")
	}
}

func TestPollerCycleWith20Sessions(t *testing.T) {
	var running, peak atomic.Int32
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(20), nil })
	s.Summary = func(cwd, id string) (transcript.Detail, error) {
		cur := running.Add(1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		running.Add(-1)
		return transcript.Detail{Title: id}, nil
	}
	start := time.Now()
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := time.Since(start)
	t.Logf("ciclo con 20 sesiones: %s (pico de concurrencia %d)", d, peak.Load())
	if d > 1500*time.Millisecond {
		t.Errorf("ciclo %s > 1.5s", d)
	}
	if peak.Load() > summaryConcurrency {
		t.Errorf("concurrencia %d > %d", peak.Load(), summaryConcurrency)
	}
}

func TestSummaryErrorDegrades(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	s.Summary = func(string, string) (transcript.Detail, error) { return transcript.Detail{}, errors.New("roto") }
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatalf("un fallo de F3 no debe romper el ciclo: %v", err)
	}
}

func TestValidateAddr(t *testing.T) {
	cases := []struct {
		addr   string
		remote bool
		ok     bool
	}{
		{"127.0.0.1:7777", false, true},
		{"localhost:7777", false, true},
		{"[::1]:7777", false, true},
		{"0.0.0.0:7777", false, false},
		{":7777", false, false},
		{"192.168.1.5:7777", false, false},
		{"0.0.0.0:7777", true, true},
		{"sin-puerto", false, false},
	}
	for _, c := range cases {
		err := ValidateAddr(c.addr, c.remote)
		if (err == nil) != c.ok {
			t.Errorf("ValidateAddr(%q, %v) err=%v, ok esperado=%v", c.addr, c.remote, err, c.ok)
		}
	}
}

func TestListenAndServeShutdown(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	s.Interval = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); done <- s.ListenAndServe(ctx, "127.0.0.1:0") }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown colgado")
	}
	wg.Wait()
}

func TestOrqJoinAndBrokenJobs(t *testing.T) {
	sess := fakeSessions(2)
	sess[0].Name = "w-006"
	sess[1].Name = "otra"
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return sess, nil })
	s.Orq = func() (map[string]orq.Job, error) {
		return map[string]orq.Job{"w-006": {ID: "9efa2c1a", Key: "claude-dashboard/p2-task006", WS: "claude-dashboard", Specialist: "claude-dashboard-worker", Bgname: "w-006"}}, nil
	}
	got, err := s.build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o := got[0].Orq
	if o == nil || o.Job != "9efa2c1a" || o.Plan == nil || *o.Plan != "2" || o.Task == nil || *o.Task != "006" ||
		o.Specialist != "claude-dashboard-worker" || o.WS != "claude-dashboard" {
		t.Errorf("orq: %+v", o)
	}
	if got[1].Orq != nil {
		t.Errorf("no es worker: %+v", got[1].Orq)
	}

	// jobs.json roto: Load devuelve error + mapa vacío; /api/sessions sigue andando
	s2 := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	dir := t.TempDir()
	bad := filepath.Join(dir, "jobs.json")
	if err := os.WriteFile(bad, []byte("{roto"), 0o600); err != nil {
		t.Fatal(err)
	}
	s2.Orq = func() (map[string]orq.Job, error) { return orq.Load(bad) }
	ts := httptest.NewServer(s2.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d con jobs.json roto", resp.StatusCode)
	}
	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil || len(snap.Sessions) != 1 || snap.Sessions[0].Orq != nil {
		t.Fatalf("snap=%+v err=%v", snap, err)
	}
}

func TestStaleAndHashStable(t *testing.T) {
	now := time.UnixMilli(10_000_000)
	old := now.Add(-11 * time.Minute).UnixMilli()
	recent := now.Add(-1 * time.Minute).UnixMilli()
	sess := fakeSessions(3)
	sess[0].Status, sess[0].LastActivityAt = "busy", old    // stale
	sess[1].Status, sess[1].LastActivityAt = "idle", old    // idle viejo: no
	sess[2].Status, sess[2].LastActivityAt = "busy", recent // busy reciente: no
	s := newTestServer(func(context.Context) ([]agents.Session, error) {
		out := make([]agents.Session, len(sess))
		copy(out, sess)
		return out, nil
	})
	s.Summary = nil // que lastActivityAt quede como lo puse
	s.StaleAfter = 10 * time.Minute
	cur := now
	s.Now = func() time.Time { return cur }

	got, err := s.build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].Stale || got[1].Stale || got[2].Stale {
		t.Errorf("stale = %v %v %v, quiero true false false", got[0].Stale, got[1].Stale, got[2].Stale)
	}

	changed, err := s.Refresh(context.Background())
	if err != nil || !changed {
		t.Fatalf("1er ciclo: %v %v", changed, err)
	}
	// pasa el tiempo sin cambios reales (ninguna sesión cruza el umbral): no emite
	cur = cur.Add(30 * time.Second)
	if changed, err := s.Refresh(context.Background()); err != nil || changed {
		t.Errorf("ciclo sin cambios emitió: %v %v", changed, err)
	}
	// ahora la reciente cruza el umbral: cambia el valor de stale y recién ahí emite
	cur = cur.Add(10 * time.Minute)
	if changed, err := s.Refresh(context.Background()); err != nil || !changed {
		t.Errorf("cambio de stale no emitió: %v %v", changed, err)
	}
}

func TestTimelineRoute(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	var gotCwd, gotID string
	s.Timeline = func(cwd, id string) ([]transcript.Event, error) {
		gotCwd, gotID = cwd, id
		return []transcript.Event{{At: 1, Kind: "prompt", Text: "a"}, {At: 2, Kind: "tool", Text: "Bash: x"}}, nil
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	for _, bad := range []string{"no-esta", "..%2F..%2Fetc%2Fpasswd", "sess-99"} {
		r, err := http.Get(ts.URL + "/api/sessions/" + bad + "/timeline")
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 404 {
			t.Errorf("%s: status %d, quiero 404", bad, r.StatusCode)
		}
	}
	if gotID != "" {
		t.Errorf("Timeline se llamó con un id fuera del snapshot: %q", gotID)
	}

	r, err := http.Get(ts.URL + "/api/sessions/sess-00/timeline")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	var out struct {
		SessionID string
		Events    []transcript.Event
	}
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.SessionID != "sess-00" || len(out.Events) != 2 || out.Events[0].Kind != "prompt" || gotCwd != "/work/demo" || gotID != "sess-00" {
		t.Errorf("out=%+v cwd=%q id=%q", out, gotCwd, gotID)
	}
}

// El poller tiene que llamar a Usage: el acumulador estaba testeado pero nadie lo
// invocaba y /api/sessions salía sin tokens con datos reales (QA PLAN-002).
func TestSessionsCarryTokens(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(2), nil })
	s.Usage = func(cwd, id string) (*transcript.Tokens, error) {
		if strings.HasSuffix(id, "1") {
			return nil, errors.New("roto") // un fallo de tokens no tumba la sesión ni el resumen
		}
		return &transcript.Tokens{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4, Model: "m"}, nil
	}
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	got := append([]agents.Session(nil), s.sessions...)
	s.mu.Unlock()
	var con, sin int
	for _, ss := range got {
		if ss.Title == "" {
			t.Errorf("%s perdió el resumen", ss.SessionID)
		}
		if ss.Tokens != nil {
			con++
			if *ss.Tokens != (agents.Tokens{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4, Model: "m"}) {
				t.Errorf("tokens = %+v", *ss.Tokens)
			}
		} else {
			sin++
		}
	}
	if con != 1 || sin != 1 {
		t.Errorf("con tokens %d, sin tokens %d; esperaba 1 y 1", con, sin)
	}
}

// El poller tiene que pedir contexto y ritmo y aplicar los umbrales; si deja de
// llamar a Activity, este test falla (mismo bug que tuvo Usage en PLAN-002).
func TestSessionsCarryContextAndRate(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(3), nil })
	s.ContextWarn, s.RateOutput, s.RateCalls = 800_000, 20_000, 25
	pre := int64(967001)
	s.Activity = func(cwd, id string, now time.Time) (*transcript.Context, *transcript.Rate, error) {
		switch {
		case strings.HasSuffix(id, "0"): // alto en contexto y en output
			return &transcript.Context{Current: 850_000, Compactions: 2, LastPreTokens: &pre},
				&transcript.Rate{WindowMin: 5, Output: 20_001, Calls: 3}, nil
		case strings.HasSuffix(id, "1"): // bajo; high solo por llamadas = 25 no supera
			return &transcript.Context{Current: 799_999}, &transcript.Rate{WindowMin: 5, Output: 20_000, Calls: 25}, nil
		}
		return nil, nil, errors.New("roto") // un fallo no tumba la sesión
	}
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	got := append([]agents.Session(nil), s.sessions...)
	s.mu.Unlock()
	if len(got) != 3 {
		t.Fatalf("sesiones=%d", len(got))
	}
	if c := got[0].Context; c == nil || !c.Warn || c.Compactions != 2 || c.LastPreTokens == nil || *c.LastPreTokens != pre {
		t.Errorf("sesión 0 context=%+v", c)
	}
	if r := got[0].Rate; r == nil || !r.High {
		t.Errorf("sesión 0 rate=%+v, quiero high por output", r)
	}
	if c := got[1].Context; c == nil || c.Warn || c.LastPreTokens != nil {
		t.Errorf("sesión 1 context=%+v", c)
	}
	if r := got[1].Rate; r == nil || r.High {
		t.Errorf("sesión 1 rate=%+v, quiero no-high en el borde", r)
	}
	if got[2].Context != nil || got[2].Rate != nil || got[2].Title == "" {
		t.Errorf("sesión 2 debe degradar sin perder el resumen: %+v", got[2])
	}
	// por llamadas
	s2 := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	s2.RateCalls = 25
	s2.Activity = func(string, string, time.Time) (*transcript.Context, *transcript.Rate, error) {
		return nil, &transcript.Rate{WindowMin: 5, Calls: 26}, nil
	}
	if _, err := s2.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := s2.sessions[0].Rate; r == nil || !r.High {
		t.Errorf("rate por llamadas=%+v", r)
	}
	// ciclo siguiente con el mismo resultado: no emite
	if changed, _ := s.Refresh(context.Background()); changed {
		t.Error("ciclo sin cambios emitió")
	}
}

// El poller tiene que pedir rama y plan (DD-23, DD-24); si enrich deja de
// llamarlos, este test falla (mismo bug que tuvo Usage en PLAN-002).
func TestSessionsCarryBranchAndPlan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(home, "repo")
	gitDir := filepath.Join(repo, ".git")
	wtGit := filepath.Join(gitDir, "worktrees", "w")
	wt := filepath.Join(home, "wt")
	files := map[string]string{
		filepath.Join(gitDir, "HEAD"):                                "ref: refs/heads/main\n",
		filepath.Join(wtGit, "HEAD"):                                 "ref: refs/heads/feat-plan-5-task-012\n",
		filepath.Join(wt, ".git"):                                    "gitdir: " + wtGit + "\n",
		filepath.Join(repo, "plans", "PLAN-005-x", "tasks", "T1.md"): "- **Estado:** `done`\n",
		filepath.Join(repo, "plans", "PLAN-005-x", "tasks", "T2.md"): "- **Estado:** `pending`\n",
		filepath.Join(repo, "plans", "PLAN-009-y", "tasks", "T1.md"): "- **Estado:** `blocked`\n",
	}
	for p, c := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := func(context.Context) ([]agents.Session, error) {
		ss := fakeSessions(3)
		ss[0].Cwd = wt // plan por la rama del worktree
		ss[1].Cwd = wt // el key de orq gana sobre la rama
		ss[1].Name = "w9"
		ss[2].Cwd = "/work/demo" // sin .git → ni rama ni plan
		return ss, nil
	}
	s := newTestServer(list)
	s.Orq = func() (map[string]orq.Job, error) {
		return map[string]orq.Job{"w9": {ID: "j", Key: "x-p9-task001"}}, nil
	}
	if _, err := s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	got := append([]agents.Session(nil), s.sessions...)
	s.mu.Unlock()

	if got[0].Branch == nil || *got[0].Branch != "feat-plan-5-task-012" {
		t.Errorf("branch[0] = %v", got[0].Branch)
	}
	if p := got[0].Plan; p == nil || p.Number != 5 || p.Total != 2 || p.Done != 1 || p.Pending != 1 {
		t.Errorf("plan[0] = %+v", p)
	}
	if p := got[1].Plan; p == nil || p.Number != 9 || p.Blocked != 1 {
		t.Errorf("plan[1] (orq key) = %+v", p)
	}
	if got[2].Branch != nil || got[2].Plan != nil {
		t.Errorf("sin .git debía ser null: %v %+v", got[2].Branch, got[2].Plan)
	}
	// Contrato: las claves existen en el JSON (null cuando no hay dato).
	body, _ := json.Marshal(got[2])
	var m map[string]any
	_ = json.Unmarshal(body, &m)
	for _, k := range []string{"branch", "plan"} {
		if v, ok := m[k]; !ok || v != nil {
			t.Errorf("clave %q: ok=%v v=%v", k, ok, v)
		}
	}
}
