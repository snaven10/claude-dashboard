package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"claude-dashboard/internal/agents"
	"claude-dashboard/internal/remote"
)

const testPublicURL = "https://mi-pc.tailnet-ejemplo.ts.net"

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// remoteFixture: server con modo remoto activo, estado en un TempDir y reloj falso.
type remoteFixture struct {
	s         *Server
	clock     *fakeClock
	dir       string
	local     string // URL del listener local
	remoteURL string // URL del listener remoto
}

func newRemoteServer(t *testing.T) (*Server, *fakeClock, string) {
	t.Helper()
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	s.Interval = time.Hour // el poller no interfiere: Refresh lazy en las rutas
	clock := &fakeClock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	st, err := remote.NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := remote.NewDevices(st)
	if err != nil {
		t.Fatal(err)
	}
	dev.Now = clock.Now
	s.PublicURL = testPublicURL
	s.Pairing = &remote.Pairing{Now: clock.Now}
	s.Devices = dev
	return s, clock, dir
}

// enableRemote activa el modo remoto en s y devuelve la cookie de un dispositivo ya emparejado.
func enableRemote(t *testing.T, s *Server) string {
	t.Helper()
	st, err := remote.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dev, err := remote.NewDevices(st)
	if err != nil {
		t.Fatal(err)
	}
	s.PublicURL = testPublicURL
	s.Pairing = &remote.Pairing{}
	s.Devices = dev
	cookie, _, err := dev.Create("tel", "")
	if err != nil {
		t.Fatal(err)
	}
	return cookie
}

func startRemote(t *testing.T) *remoteFixture {
	t.Helper()
	s, clock, dir := newRemoteServer(t)
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
	return f
}

type reqOpt func(*http.Request)

func withCookie(v string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "cd_device", Value: v}) }
}
func withHeader(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

func do(t *testing.T, method, url, body string, opts ...reqOpt) (*http.Response, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	if method == "POST" || method == "DELETE" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// pair hace new (local) + redeem (remoto) y devuelve la cookie y el deviceId.
func (f *remoteFixture) pair(t *testing.T, name string) (cookie, deviceID string) {
	t.Helper()
	resp, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	if resp.StatusCode != 200 {
		t.Fatalf("pair/new: %d %s", resp.StatusCode, body)
	}
	var pn struct {
		URL  string `json:"url"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &pn); err != nil {
		t.Fatal(err)
	}
	tok := strings.TrimPrefix(pn.URL, testPublicURL+"/#pair=")
	resp, body = do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`","name":"`+name+`"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("redeem: %d %s", resp.StatusCode, body)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "cd_device" {
			cookie = c.Value
		}
	}
	var out struct {
		DeviceID string `json:"deviceId"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return cookie, out.DeviceID
}

func TestRedeemSetsDeviceCookie(t *testing.T) {
	f := startRemote(t)
	resp, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	var pn struct {
		URL       string `json:"url"`
		Code      string `json:"code"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := json.Unmarshal([]byte(body), &pn); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	if !strings.HasPrefix(pn.URL, testPublicURL+"/#pair=") || len(pn.Code) != 9 || pn.Code[4] != '-' {
		t.Fatalf("respuesta de pair/new: %+v", pn)
	}
	if want := f.clock.Now().Add(remote.PairingTTL).UnixMilli(); pn.ExpiresAt != want {
		t.Errorf("expiresAt = %d, quería %d", pn.ExpiresAt, want)
	}
	tok := strings.TrimPrefix(pn.URL, testPublicURL+"/#pair=")
	resp, body = do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`","name":"Mi teléfono"}`,
		withHeader("Tailscale-User-Login", "ana@ejemplo.test"))
	if resp.StatusCode != 200 {
		t.Fatalf("redeem: %d %s", resp.StatusCode, body)
	}
	var c *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == "cd_device" {
			c = ck
		}
	}
	if c == nil {
		t.Fatalf("sin cookie cd_device: %v", resp.Header)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 7776000 {
		t.Errorf("atributos de la cookie: %+v", c)
	}
	if len(c.Value) < 40 {
		t.Errorf("cookie corta: %q", c.Value)
	}
	devs := f.s.Devices.List()
	if len(devs) != 1 || devs[0].Name != "Mi teléfono" || devs[0].Login != "ana@ejemplo.test" {
		t.Fatalf("dispositivos: %+v", devs)
	}
	// La cookie vale en el remoto.
	if r, _ := do(t, "GET", f.remoteURL+"/api/sessions", "", withCookie(c.Value)); r.StatusCode != 200 {
		t.Errorf("sessions con la cookie nueva: %d", r.StatusCode)
	}
}

func TestRedeemExpiredToken(t *testing.T) {
	f := startRemote(t)
	_, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	var pn struct{ URL string }
	_ = json.Unmarshal([]byte(body), &pn)
	tok := strings.TrimPrefix(pn.URL, testPublicURL+"/#pair=")
	f.clock.Add(5*time.Minute + time.Second)
	resp, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`"}`)
	if resp.StatusCode != 401 && resp.StatusCode != 410 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 0 || len(f.s.Devices.List()) != 0 {
		t.Error("el token vencido creó un dispositivo")
	}
}

func TestRedeemSingleUseTokenThenCode(t *testing.T) {
	f := startRemote(t)
	_, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	var pn struct{ URL, Code string }
	_ = json.Unmarshal([]byte(body), &pn)
	tok := strings.TrimPrefix(pn.URL, testPublicURL+"/#pair=")
	if r, b := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`"}`); r.StatusCode != 200 {
		t.Fatalf("primer canje: %d %s", r.StatusCode, b)
	}
	if r, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`"}`); r.StatusCode != 401 {
		t.Errorf("token reusado: %d", r.StatusCode)
	}
	if r, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"code":"`+pn.Code+`"}`); r.StatusCode != 401 {
		t.Errorf("código después del token: %d", r.StatusCode)
	}
	if n := len(f.s.Devices.List()); n != 1 {
		t.Errorf("dispositivos = %d, quería 1", n)
	}
}

func TestRedeemCodeLowercaseNoDash(t *testing.T) {
	f := startRemote(t)
	_, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	var pn struct{ Code string }
	_ = json.Unmarshal([]byte(body), &pn)
	typed := strings.ToLower(strings.ReplaceAll(pn.Code, "-", ""))
	if r, b := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"code":"`+typed+`"}`); r.StatusCode != 200 {
		t.Fatalf("código a mano: %d %s", r.StatusCode, b)
	}
}

func TestNewPairingInvalidatesPrevious(t *testing.T) {
	f := startRemote(t)
	_, b1 := do(t, "POST", f.local+"/api/pair/new", "{}")
	_, b2 := do(t, "POST", f.local+"/api/pair/new", "{}")
	var p1, p2 struct{ URL string }
	_ = json.Unmarshal([]byte(b1), &p1)
	_ = json.Unmarshal([]byte(b2), &p2)
	t1 := strings.TrimPrefix(p1.URL, testPublicURL+"/#pair=")
	t2 := strings.TrimPrefix(p2.URL, testPublicURL+"/#pair=")
	if r, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+t1+`"}`); r.StatusCode != 401 {
		t.Errorf("token anterior: %d", r.StatusCode)
	}
	// El fallo anterior cuenta, pero el nuevo sigue vivo (1 de 5).
	if r, b := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+t2+`"}`); r.StatusCode != 200 {
		t.Errorf("token nuevo: %d %s", r.StatusCode, b)
	}
}

func TestFiveFailuresInvalidatePending(t *testing.T) {
	f := startRemote(t)
	_, body := do(t, "POST", f.local+"/api/pair/new", "{}")
	var pn struct{ URL string }
	_ = json.Unmarshal([]byte(body), &pn)
	tok := strings.TrimPrefix(pn.URL, testPublicURL+"/#pair=")
	for i := 0; i < 5; i++ {
		if r, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"code":"ZZZZ-ZZZZ"}`); r.StatusCode != 401 {
			t.Fatalf("fallo %d: %d", i, r.StatusCode)
		}
	}
	if r, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"token":"`+tok+`"}`); r.StatusCode != 401 {
		t.Errorf("sexto canje (correcto) = %d, quería 401", r.StatusCode)
	}
	if len(f.s.Devices.List()) != 0 {
		t.Error("se creó un dispositivo tras el bloqueo")
	}
}

func TestInvalidCookieRejectedOnRemote(t *testing.T) {
	f := startRemote(t)
	good, _ := f.pair(t, "a")
	// Un dispositivo revocado: emparejo otro y lo borro.
	revoked, rid := f.pair(t, "b")
	if r, _ := do(t, "DELETE", f.local+"/api/devices/"+rid, ""); r.StatusCode != 204 {
		t.Fatalf("revocar: %d", r.StatusCode)
	}
	// Para que /timeline no sea 404 por sesión desconocida, cargo el snapshot.
	if _, err := f.s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/api/sessions", "/api/events", "/api/sessions/sess-00/timeline", "/api/history"}
	cookies := map[string][]reqOpt{
		"sin cookie":  nil,
		"vacía":       {withCookie("")},
		"aleatoria":   {withCookie("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")},
		"revocada":    {withCookie(revoked)},
		"otra cookie": {withHeader("Cookie", "otra=cosa")},
	}
	for _, p := range paths {
		for name, opts := range cookies {
			resp, body := do(t, "GET", f.remoteURL+p, "", opts...)
			if resp.StatusCode != 401 || !strings.Contains(body, `"unpaired"`) {
				t.Errorf("%s con cookie %s: %d %s", p, name, resp.StatusCode, body)
			}
		}
		if p == "/api/events" {
			continue // stream abierto: lo cubre TestRevokeClosesOnlyThatDevicesSSE
		}
		if resp, _ := do(t, "GET", f.remoteURL+p, "", withCookie(good)); resp.StatusCode != 200 {
			t.Errorf("cookie buena en %s: %d", p, resp.StatusCode)
		}
	}
	// Ruta /api/ desconocida: también exige cookie (denegar por defecto).
	if resp, _ := do(t, "GET", f.remoteURL+"/api/otra", ""); resp.StatusCode != 401 {
		t.Errorf("/api/otra sin cookie: %d", resp.StatusCode)
	}
}

func TestLocalWithoutCookieStill200(t *testing.T) {
	f := startRemote(t)
	for _, p := range []string{"/api/sessions", "/api/history"} {
		if resp, _ := do(t, "GET", f.local+p, ""); resp.StatusCode != 200 {
			t.Errorf("local %s sin cookie: %d", p, resp.StatusCode)
		}
	}
}

func TestStaticsAndRemoteInfoWithoutCookie(t *testing.T) {
	f := startRemote(t)
	resp, body := do(t, "GET", f.remoteURL+"/", "")
	if resp.StatusCode != 200 || !strings.Contains(body, "ok") {
		t.Errorf("estático remoto: %d %q", resp.StatusCode, body)
	}
	resp, body = do(t, "GET", f.remoteURL+"/api/remote", "")
	if resp.StatusCode != 200 {
		t.Fatalf("/api/remote remoto: %d", resp.StatusCode)
	}
	var info struct{ Enabled, Local, Paired bool }
	_ = json.Unmarshal([]byte(body), &info)
	if !info.Enabled || info.Local || info.Paired {
		t.Errorf("remoto sin cookie: %+v", info)
	}
	cookie, _ := f.pair(t, "x")
	_, body = do(t, "GET", f.remoteURL+"/api/remote", "", withCookie(cookie))
	_ = json.Unmarshal([]byte(body), &info)
	if !info.Paired {
		t.Errorf("remoto con cookie: %+v", info)
	}
	_, body = do(t, "GET", f.local+"/api/remote", "")
	info = struct{ Enabled, Local, Paired bool }{}
	_ = json.Unmarshal([]byte(body), &info)
	if !info.Enabled || !info.Local {
		t.Errorf("local: %+v", info)
	}
}

func TestRoutesHiddenPerListener(t *testing.T) {
	f := startRemote(t)
	cookie, id := f.pair(t, "x")
	// Por el remoto, aun con cookie válida: 404.
	if r, _ := do(t, "POST", f.remoteURL+"/api/pair/new", "{}", withCookie(cookie)); r.StatusCode != 404 {
		t.Errorf("remoto pair/new: %d", r.StatusCode)
	}
	if r, _ := do(t, "GET", f.remoteURL+"/api/devices", "", withCookie(cookie)); r.StatusCode != 404 {
		t.Errorf("remoto devices: %d", r.StatusCode)
	}
	if r, _ := do(t, "DELETE", f.remoteURL+"/api/devices/"+id, "", withCookie(cookie)); r.StatusCode != 404 {
		t.Errorf("remoto revocar: %d", r.StatusCode)
	}
	if !f.s.Devices.Has(id) {
		t.Error("el remoto revocó un dispositivo")
	}
	// Por el local, redeem no existe.
	if r, _ := do(t, "POST", f.local+"/api/pair/redeem", `{"code":"AAAA-AAAA"}`); r.StatusCode != 404 {
		t.Errorf("local redeem: %d", r.StatusCode)
	}
	// Y la lista local no filtra el hash.
	_, body := do(t, "GET", f.local+"/api/devices", "")
	if strings.Contains(strings.ToLower(body), "cookiehash") || !strings.Contains(body, id) {
		t.Errorf("lista local: %s", body)
	}
	if r, _ := do(t, "DELETE", f.local+"/api/devices/no-existe", ""); r.StatusCode != 404 {
		t.Errorf("revocar inexistente: %d", r.StatusCode)
	}
}

func TestCSRFGuards(t *testing.T) {
	f := startRemote(t)
	cases := []struct {
		name string
		url  string
		opts []reqOpt
	}{
		{"sin content-type json (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Content-Type", "text/plain")}},
		{"form (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Content-Type", "application/x-www-form-urlencoded")}},
		{"origin ajeno (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Origin", "https://malo.ejemplo.test")}},
		{"sec-fetch-site cross-site (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Sec-Fetch-Site", "cross-site")}},
		{"sec-fetch-site same-site (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Sec-Fetch-Site", "same-site")}},
		{"origin con otro puerto (local)", f.local + "/api/pair/new", []reqOpt{withHeader("Origin", "http://127.0.0.1:1")}},
		{"origin ajeno (remoto)", f.remoteURL + "/api/pair/redeem", []reqOpt{withHeader("Origin", "https://malo.ejemplo.test")}},
		{"content-type (remoto)", f.remoteURL + "/api/pair/redeem", []reqOpt{withHeader("Content-Type", "text/plain")}},
	}
	for _, c := range cases {
		resp, _ := do(t, "POST", c.url, `{"code":"AAAA-AAAA"}`, c.opts...)
		if resp.StatusCode != 403 {
			t.Errorf("%s: %d, quería 403", c.name, resp.StatusCode)
		}
	}
	// Los permitidos: Origin propio y Sec-Fetch-Site same-origin / none.
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(f.local, "http://"))
	for _, o := range [][]reqOpt{
		{withHeader("Origin", "http://127.0.0.1:"+port)},
		{withHeader("Origin", "http://localhost:"+port)},
		{withHeader("Sec-Fetch-Site", "same-origin")},
		{withHeader("Sec-Fetch-Site", "none")},
		{withHeader("Content-Type", "application/json; charset=utf-8")},
	} {
		if resp, b := do(t, "POST", f.local+"/api/pair/new", "{}", o...); resp.StatusCode != 200 {
			t.Errorf("legítimo rechazado: %d %s", resp.StatusCode, b)
		}
	}
	if resp, _ := do(t, "POST", f.remoteURL+"/api/pair/redeem", `{"code":"AAAA-AAAA"}`, withHeader("Origin", testPublicURL)); resp.StatusCode != 401 {
		t.Errorf("origin = public-url debería pasar el CSRF y fallar el canje: %d", resp.StatusCode)
	}
	// DELETE también.
	if resp, _ := do(t, "DELETE", f.local+"/api/devices/x", "", withHeader("Content-Type", "text/plain")); resp.StatusCode != 403 {
		t.Errorf("DELETE sin json: %d", resp.StatusCode)
	}
}

func TestRevokeClosesOnlyThatDevicesSSE(t *testing.T) {
	f := startRemote(t)
	c1, id1 := f.pair(t, "uno")
	c2, _ := f.pair(t, "dos")
	if _, err := f.s.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}

	open := func(cookie string) (*bufio.Reader, func()) {
		req, _ := http.NewRequest("GET", f.remoteURL+"/api/events", nil)
		req.AddCookie(&http.Cookie{Name: "cd_device", Value: cookie})
		ctx, cancel := context.WithCancel(context.Background())
		resp, err := http.DefaultClient.Do(req.WithContext(ctx))
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			cancel()
			t.Fatalf("events: %d", resp.StatusCode)
		}
		br := bufio.NewReader(resp.Body)
		// Esperar al primer snapshot: el suscriptor ya está registrado.
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				cancel()
				t.Fatalf("leyendo el primer evento: %v", err)
			}
			if strings.HasPrefix(line, "data:") {
				break
			}
		}
		return br, func() { cancel(); resp.Body.Close() }
	}
	br1, close1 := open(c1)
	defer close1()
	br2, close2 := open(c2)
	defer close2()

	if r, _ := do(t, "DELETE", f.local+"/api/devices/"+id1, ""); r.StatusCode != 204 {
		t.Fatalf("revocar: %d", r.StatusCode)
	}

	ended := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, br1); ended <- err }()
	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("el SSE del dispositivo revocado sigue abierto")
	}

	// El otro sigue vivo: el heartbeat (50 ms) llega.
	got := make(chan string, 1)
	go func() { l, _ := br2.ReadString('\n'); got <- l }()
	select {
	case l := <-got:
		if !strings.HasPrefix(l, ":") && !strings.HasPrefix(l, "event") && !strings.HasPrefix(l, "data") && l != "\n" {
			t.Errorf("línea inesperada: %q", l)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("el SSE del otro dispositivo dejó de entregar")
	}
	if r, _ := do(t, "GET", f.remoteURL+"/api/sessions", "", withCookie(c1)); r.StatusCode != 401 {
		t.Errorf("cookie revocada: %d", r.StatusCode)
	}
	if r, _ := do(t, "GET", f.remoteURL+"/api/sessions", "", withCookie(c2)); r.StatusCode != 200 {
		t.Errorf("la otra cookie: %d", r.StatusCode)
	}
}

func TestDevicesFileHasNoPlainCookieAnd0600(t *testing.T) {
	f := startRemote(t)
	cookie, _ := f.pair(t, "x")
	path := filepath.Join(f.dir, "devices.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), cookie) {
		t.Error("devices.json contiene la cookie en claro")
	}
	if !strings.Contains(string(data), "cookieHash") {
		t.Errorf("devices.json sin hash: %s", data)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("modo = %v", st.Mode().Perm())
	}
}

func TestNoPublicURLNoRemoteRoutes(t *testing.T) {
	s := newTestServer(func(context.Context) ([]agents.Session, error) { return fakeSessions(1), nil })
	s.Interval = time.Hour
	var urls [2]string
	for i, rem := range []bool{false, true} {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		srv := s.newHTTPServer(rem)
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() { _ = srv.Close() })
		urls[i] = "http://" + ln.Addr().String()
	}
	for _, c := range []struct{ m, p string }{
		{"POST", "/api/pair/new"}, {"GET", "/api/devices"}, {"GET", "/api/remote"}, {"POST", "/api/pair/redeem"},
	} {
		if r, _ := do(t, c.m, urls[0]+c.p, "{}"); r.StatusCode != 404 {
			t.Errorf("%s %s: %d, quería 404", c.m, c.p, r.StatusCode)
		}
	}
	if r, _ := do(t, "GET", urls[0]+"/api/sessions", ""); r.StatusCode != 200 {
		t.Errorf("local: %d", r.StatusCode)
	}
	// Un listener remoto sin el modo completo no sirve datos (falla cerrado).
	if r, _ := do(t, "GET", urls[1]+"/api/sessions", ""); r.StatusCode != 404 {
		t.Errorf("remoto sin modo remoto: %d", r.StatusCode)
	}
}
