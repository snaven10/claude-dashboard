package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"claude-dashboard/internal/agents"
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
