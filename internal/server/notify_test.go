package server

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/remote"
	"claude-dashboard/internal/webpush"
)

type sink struct {
	mu    sync.Mutex
	items []pushItem
}

func (k *sink) add(p []byte, topic string) {
	k.mu.Lock()
	k.items = append(k.items, pushItem{append([]byte(nil), p...), topic})
	k.mu.Unlock()
}

func (k *sink) take() []pushItem {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := k.items
	k.items = nil
	return out
}

func newNotifier(t *testing.T, content string, cooldown time.Duration) (*Notifier, *sink, *fakeClock) {
	t.Helper()
	k := &sink{}
	clock := &fakeClock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	return &Notifier{Now: clock.Now, Cooldown: cooldown, Content: content, Enqueue: k.add}, k, clock
}

func sess(id, status string, clock *fakeClock) agents.Session {
	return agents.Session{
		ID: id, SessionID: "sid-" + id, PID: 100, Kind: "interactive", Name: "n-" + id,
		Project: "proj", Title: "Título " + id, Status: status,
		StartedAt: clock.Now().UnixMilli() - 60_000, LastActivityAt: clock.Now().UnixMilli() - 1000,
	}
}

func cycle(n *Notifier, k *sink, list ...agents.Session) []pushItem {
	n.Observe(list)
	n.Flush()
	return k.take()
}

func TestNotifierFirstCycleOnlyRecords(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	if got := cycle(n, k, sess("a", "waiting", c), sess("b", "blocked", c)); len(got) != 0 {
		t.Fatalf("primer ciclo avisó %d", len(got))
	}
	// Seguir esperando desde antes de arrancar tampoco es una transición.
	for i := 0; i < 3; i++ {
		if got := cycle(n, k, sess("a", "waiting", c), sess("b", "blocked", c)); len(got) != 0 {
			t.Fatalf("ciclo %d avisó %d por sesiones que ya esperaban", i+2, len(got))
		}
	}
}

func TestNotifierDebounceFiresOnceOnSecondCycle(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	cycle(n, k, sess("a", "busy", c))
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 0 {
		t.Fatalf("1er ciclo waiting avisó %d", len(got))
	}
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 1 {
		t.Fatalf("2do ciclo waiting avisó %d, quería 1", len(got))
	}
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 0 {
		t.Fatalf("3er ciclo avisó %d", len(got))
	}
}

func TestNotifierSingleCycleBlipDoesNotFire(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	cycle(n, k, sess("a", "busy", c))
	cycle(n, k, sess("a", "waiting", c))
	if got := cycle(n, k, sess("a", "busy", c)); len(got) != 0 {
		t.Fatalf("avisó %d por un estado de un ciclo", len(got))
	}
}

func TestNotifierCooldown(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 2*time.Minute)
	cycle(n, k, sess("a", "busy", c))
	cycle(n, k, sess("a", "waiting", c))
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 1 {
		t.Fatalf("aviso inicial: %d", len(got))
	}
	// Sale y vuelve antes del cooldown.
	cycle(n, k, sess("a", "busy", c))
	c.Add(30 * time.Second)
	cycle(n, k, sess("a", "waiting", c))
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 0 {
		t.Fatalf("avisó %d dentro del cooldown", len(got))
	}
	// Otra vez, ya vencido el cooldown.
	cycle(n, k, sess("a", "busy", c))
	c.Add(3 * time.Minute)
	cycle(n, k, sess("a", "waiting", c))
	if got := cycle(n, k, sess("a", "waiting", c)); len(got) != 1 {
		t.Fatalf("después del cooldown avisó %d, quería 1", len(got))
	}
}

func TestNotifierBlockedCountsAndAbandonedIgnored(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	old := sess("old", "waiting", c)
	old.LastActivityAt = c.Now().Add(-25 * time.Hour).UnixMilli()
	cycle(n, k, sess("b", "busy", c), sess("old", "busy", c))
	cycle(n, k, sess("b", "blocked", c), old)
	got := cycle(n, k, sess("b", "blocked", c), old)
	if len(got) != 1 {
		t.Fatalf("avisos = %d, quería 1 (solo blocked; la abandonada no avisa)", len(got))
	}
	if !strings.Contains(string(got[0].payload), "Título b") {
		t.Fatalf("avisó la sesión equivocada: %s", got[0].payload)
	}
}

func TestNotifierGroupsBursts(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	mk := func(status string) []agents.Session {
		var l []agents.Session
		for _, id := range []string{"a", "b", "c", "d", "e"} {
			l = append(l, sess(id, status, c))
		}
		return l
	}
	cycle(n, k, mk("busy")...)
	cycle(n, k, mk("waiting")...)
	got := cycle(n, k, mk("waiting")...)
	if len(got) != 1 {
		t.Fatalf("avisos = %d, quería 1 agrupado", len(got))
	}
	if !strings.Contains(string(got[0].payload), "5 sesiones te esperan") {
		t.Fatalf("payload sin el agrupado: %s", got[0].payload)
	}
}

func TestNotifierThreeSessionsAreNotGrouped(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	mk := func(status string) []agents.Session {
		return []agents.Session{sess("a", status, c), sess("b", status, c), sess("c", status, c)}
	}
	cycle(n, k, mk("busy")...)
	cycle(n, k, mk("waiting")...)
	if got := cycle(n, k, mk("waiting")...); len(got) != 3 {
		t.Fatalf("avisos = %d, quería 3 individuales", len(got))
	}
}

func decodePush(t *testing.T, b []byte) pushPayload {
	t.Helper()
	var p pushPayload
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("payload no es JSON: %v (%s)", err, b)
	}
	return p
}

func TestNotifierContentModes(t *testing.T) {
	long := strings.Repeat("palabra ", 60) // > 180 runas
	build := func(mode string) []pushItem {
		n, k, c := newNotifier(t, mode, 0)
		a := sess("a", "waiting", c)
		wf := "permission-secreto-wf"
		a.WaitingFor = &wf
		a.Ask = &agents.Ask{Kind: "tool", Text: "ASKTEXT " + long}
		busy := a
		busy.Status = "busy"
		cycle(n, k, busy)
		cycle(n, k, a)
		return cycle(n, k, a)
	}

	full := build(PushContentFull)
	if len(full) != 1 {
		t.Fatalf("full: %d avisos", len(full))
	}
	p := decodePush(t, full[0].payload)
	if !strings.Contains(p.Body, "ASKTEXT") || !strings.HasPrefix(p.Body, "proj: ") {
		t.Fatalf("full sin ask.text: %q", p.Body)
	}
	if r := []rune(p.Body); len(r) != 180 || r[len(r)-1] != '…' {
		t.Fatalf("full no truncó a 180 runas con …: %d runas", len(r))
	}
	if p.Title != "Título a — esperándote" || p.URL != "/?s=sid-a" || !strings.HasPrefix(p.Tag, "cd-") || p.V != 1 {
		t.Fatalf("campos: %+v", p)
	}

	title := build(PushContentTitle)
	if len(title) != 1 {
		t.Fatalf("title: %d avisos", len(title))
	}
	raw := string(title[0].payload)
	for _, leak := range []string{"ASKTEXT", "permission-secreto-wf", "palabra"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("title filtró %q: %s", leak, raw)
		}
	}
	if p := decodePush(t, title[0].payload); p.Body != "proj" || p.Title != "Título a — esperándote" {
		t.Fatalf("title: %+v", p)
	}

	none := build(PushContentNone)
	if len(none) != 1 {
		t.Fatalf("none: %d avisos", len(none))
	}
	raw = string(none[0].payload)
	for _, leak := range []string{"Título", "proj", "sid-a", "ASKTEXT", "n-a"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("none filtró %q: %s", leak, raw)
		}
	}
	if p := decodePush(t, none[0].payload); p.URL != "/" || p.Tag != "claude-dashboard" || p.Title != "claude-dashboard" {
		t.Fatalf("none: %+v", p)
	}
}

func TestNotifierWaitingForFallbackAndBadURLID(t *testing.T) {
	n, k, c := newNotifier(t, PushContentFull, 0)
	a := sess("a", "waiting", c)
	a.SessionID = "../x?y=1"
	wf := "permission"
	a.WaitingFor = &wf
	busy := a
	busy.Status = "busy"
	cycle(n, k, busy)
	cycle(n, k, a)
	got := cycle(n, k, a)
	if len(got) != 1 {
		t.Fatalf("avisos = %d", len(got))
	}
	p := decodePush(t, got[0].payload)
	if p.URL != "/" || p.Body != "proj: permission" {
		t.Fatalf("%+v", p)
	}
}

func TestNotifierPayloadSizeBounded(t *testing.T) {
	n, k, c := newNotifier(t, PushContentFull, 0)
	a := sess("a", "waiting", c)
	a.Title = strings.Repeat("😀", 400)
	a.Ask = &agents.Ask{Text: strings.Repeat("😀", 400)}
	busy := a
	busy.Status = "busy"
	cycle(n, k, busy)
	cycle(n, k, a)
	got := cycle(n, k, a)
	if len(got) != 1 {
		t.Fatalf("avisos = %d", len(got))
	}
	if len(got[0].payload) > 3000 {
		t.Fatalf("payload de %d bytes", len(got[0].payload))
	}
	decodePush(t, got[0].payload)
}

func TestNotifierForgetsAbsentSessions(t *testing.T) {
	n, k, c := newNotifier(t, PushContentTitle, 0)
	cycle(n, k, sess("a", "busy", c), sess("b", "busy", c))
	if n.Len() != 2 {
		t.Fatalf("estado = %d", n.Len())
	}
	cycle(n, k, sess("b", "busy", c))
	if n.Len() != 1 {
		t.Fatalf("estado = %d, la ausente no se olvidó", n.Len())
	}
	// Reaparece ya en waiting: transición nueva, avisa recién al 2º ciclo.
	if got := cycle(n, k, sess("a", "waiting", c), sess("b", "busy", c)); len(got) != 0 {
		t.Fatalf("avisó al reaparecer: %d", len(got))
	}
	if got := cycle(n, k, sess("a", "waiting", c), sess("b", "busy", c)); len(got) != 1 {
		t.Fatalf("avisos al 2º ciclo = %d, quería 1", len(got))
	}
}

// ---- integración con el poller ----

func TestRefreshNotifiesOnceAndDoesNotBlockOnSlowEnqueue(t *testing.T) {
	var mu sync.Mutex
	status := "busy"
	list := func(context.Context) ([]agents.Session, error) {
		mu.Lock()
		defer mu.Unlock()
		s := fakeSessions(1)
		s[0].Status = status
		s[0].LastActivityAt = time.Now().UnixMilli() // fakeSessions trae 1970: sería "abandonada"
		return s, nil
	}
	setStatus := func(v string) { mu.Lock(); status = v; mu.Unlock() }

	release := make(chan struct{})
	var calls int
	var cmu sync.Mutex
	srv := newTestServer(list)
	srv.Notifier = &Notifier{
		Content: PushContentTitle,
		Enqueue: func([]byte, string) {
			<-release // tarda "para siempre" hasta que el test lo suelta
			cmu.Lock()
			calls++
			cmu.Unlock()
		},
	}

	ctx := context.Background()
	refresh := func() {
		done := make(chan struct{})
		go func() {
			defer close(done)
			if _, err := srv.Refresh(ctx); err != nil {
				t.Error(err)
			}
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Refresh se bloqueó esperando a Enqueue")
		}
	}
	refresh() // busy (primer ciclo)
	setStatus("waiting")
	refresh() // waiting 1
	refresh() // waiting 2 → aviso (Enqueue bloqueado)
	refresh() // sin cambios: no vuelve a avisar
	refresh()
	close(release)
	srv.Notifier.Flush()
	cmu.Lock()
	defer cmu.Unlock()
	if calls != 1 {
		t.Fatalf("Enqueue llamado %d veces, quería 1", calls)
	}
}

// ---- endpoints HTTP ----

type pushFixture struct {
	*remoteFixture
}

func startPush(t *testing.T) *pushFixture {
	t.Helper()
	s, clock, dir := newRemoteServer(t)
	st, err := remote.NewStore(filepath.Join(dir, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Subs, err = remote.NewSubscriptions(st); err != nil {
		t.Fatal(err)
	}
	if s.Keys, err = webpush.LoadOrCreateKeys(st.Path("vapid.json")); err != nil {
		t.Fatal(err)
	}
	f := &remoteFixture{s: s, clock: clock, dir: dir}
	for i, rem := range []bool{false, true} {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv := s.newHTTPServer(rem)
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() { _ = srv.Close() })
		if i == 0 {
			f.local = "http://" + ln.Addr().String()
		} else {
			f.remoteURL = "http://" + ln.Addr().String()
		}
	}
	return &pushFixture{f}
}

func subBody(t *testing.T, endpoint string) string {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	enc := base64.RawURLEncoding.EncodeToString
	b, _ := json.Marshal(map[string]any{
		"endpoint": endpoint,
		"keys":     map[string]string{"p256dh": enc(priv.PublicKey().Bytes()), "auth": enc(auth)},
	})
	return string(b)
}

const goodEndpoint = "https://push.example.com/send/abc123"

func TestPushSubscribeRemoteNeedsCookie(t *testing.T) {
	f := startPush(t)
	resp, _ := do(t, "POST", f.remoteURL+"/api/push/subscribe", subBody(t, goodEndpoint))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin cookie: %d, quería 401", resp.StatusCode)
	}
	resp, _ = do(t, "GET", f.remoteURL+"/api/push/key", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("key sin cookie: %d, quería 401", resp.StatusCode)
	}
	if n := len(f.s.Subs.All()); n != 0 {
		t.Fatalf("quedaron %d suscripciones", n)
	}
}

func TestPushSubscribeWithCookieKeepsDeviceID(t *testing.T) {
	f := startPush(t)
	cookie, devID := f.pair(t, "tel")
	resp, body := do(t, "GET", f.remoteURL+"/api/push/key", "", withCookie(cookie))
	var k struct {
		Key string `json:"key"`
	}
	_ = json.Unmarshal([]byte(body), &k)
	if resp.StatusCode != 200 || k.Key != f.s.Keys.PublicKeyB64() || k.Key == "" {
		t.Fatalf("key: %d %s", resp.StatusCode, body)
	}
	resp, body = do(t, "POST", f.remoteURL+"/api/push/subscribe", subBody(t, goodEndpoint), withCookie(cookie))
	if resp.StatusCode != 200 {
		t.Fatalf("subscribe: %d %s", resp.StatusCode, body)
	}
	all := f.s.Subs.All()
	if len(all) != 1 || all[0].DeviceID != devID || all[0].Endpoint != goodEndpoint {
		t.Fatalf("suscripciones: %+v (device %s)", all, devID)
	}
	// Otro dispositivo no puede borrar la suscripción ajena.
	other, _ := f.pair(t, "otro")
	do(t, "POST", f.remoteURL+"/api/push/unsubscribe", `{"endpoint":"`+goodEndpoint+`"}`, withCookie(other))
	if len(f.s.Subs.All()) != 1 {
		t.Fatal("un dispositivo borró la suscripción de otro")
	}
	resp, _ = do(t, "POST", f.remoteURL+"/api/push/unsubscribe", `{"endpoint":"`+goodEndpoint+`"}`, withCookie(cookie))
	if resp.StatusCode != http.StatusNoContent || len(f.s.Subs.All()) != 0 {
		t.Fatalf("unsubscribe: %d, quedan %d", resp.StatusCode, len(f.s.Subs.All()))
	}
}

func TestPushSubscribeRejectsBadEndpoints(t *testing.T) {
	f := startPush(t)
	cookie, _ := f.pair(t, "tel")
	for _, ep := range []string{
		"http://push.example.com/x",
		"https://127.0.0.1/x",
		"https://[::1]/x",
		"https://localhost/x",
		"",
	} {
		resp, _ := do(t, "POST", f.remoteURL+"/api/push/subscribe", subBody(t, ep), withCookie(cookie))
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("endpoint %q: %d, quería 400", ep, resp.StatusCode)
		}
	}
	resp, _ := do(t, "POST", f.remoteURL+"/api/push/subscribe",
		`{"endpoint":"`+goodEndpoint+`","keys":{"p256dh":"AAAA","auth":"AAAA"}}`, withCookie(cookie))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("claves cortas: %d, quería 400", resp.StatusCode)
	}
	if n := len(f.s.Subs.All()); n != 0 {
		t.Fatalf("quedaron %d suscripciones", n)
	}
}

func TestPushLocalUsesLocalDevice(t *testing.T) {
	f := startPush(t)
	resp, body := do(t, "POST", f.local+"/api/push/subscribe", subBody(t, goodEndpoint))
	if resp.StatusCode != 200 {
		t.Fatalf("subscribe local: %d %s", resp.StatusCode, body)
	}
	all := f.s.Subs.All()
	if len(all) != 1 || all[0].DeviceID != "local" {
		t.Fatalf("%+v", all)
	}
	// DD-48: sin Content-Type JSON se rechaza.
	resp, _ = do(t, "POST", f.local+"/api/push/subscribe", subBody(t, goodEndpoint+"2"),
		withHeader("Content-Type", "text/plain"))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("text/plain: %d, quería 403", resp.StatusCode)
	}
}

func TestRevokeDeviceRemovesItsSubscriptions(t *testing.T) {
	f := startPush(t)
	cookie, devID := f.pair(t, "tel")
	otherCookie, _ := f.pair(t, "otro")
	do(t, "POST", f.remoteURL+"/api/push/subscribe", subBody(t, goodEndpoint), withCookie(cookie))
	do(t, "POST", f.remoteURL+"/api/push/subscribe", subBody(t, goodEndpoint+"-otro"), withCookie(otherCookie))
	if len(f.s.Subs.All()) != 2 {
		t.Fatalf("setup: %d", len(f.s.Subs.All()))
	}
	resp, _ := do(t, "DELETE", f.local+"/api/devices/"+devID, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("revoke: %d", resp.StatusCode)
	}
	all := f.s.Subs.All()
	if len(all) != 1 || all[0].Endpoint != goodEndpoint+"-otro" {
		t.Fatalf("tras revocar quedaron: %+v", all)
	}
}

func TestPushRoutesAbsentWithoutRemoteMode(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	if s.Notifier != nil {
		t.Fatal("Notifier debería ser nil sin modo remoto")
	}
	h := s.Handler()
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/push/key"},
		{"POST", "/api/push/subscribe"},
		{"POST", "/api/push/unsubscribe"},
	} {
		rec := httptest.NewRecorder()
		req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: %d, quería 404", tc.method, tc.path, rec.Code)
		}
	}
}
