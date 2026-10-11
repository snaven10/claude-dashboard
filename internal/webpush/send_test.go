package webpush

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"claude-dashboard/internal/remote"
)

const testEndpoint = "https://push.ejemplo.test/wpush/v2/abc123"

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type rig struct {
	sender *Sender
	clock  *fakeClock
	subs   *remote.Subscriptions
	sub    Subscription
	uaPriv *ecdh.PrivateKey
	auth   []byte
	hits   atomic.Int32
}

// newRig levanta un "servicio de push" TLS falso y un Sender cuyo cliente
// manda cualquier host (push.ejemplo.test) a ese servidor.
func newRig(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body []byte)) *rig {
	t.Helper()
	rg := &rig{clock: &fakeClock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rg.hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		handler(w, r, body)
	}))
	t.Cleanup(srv.Close)

	addr := srv.Listener.Addr().String()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			// Solo test: el cert del httptest no es de push.ejemplo.test.
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}

	keys := newKeysT(t)
	var p256dh, auth []byte
	rg.uaPriv, p256dh, auth = newSubscription(t)
	rg.auth = auth
	rg.sub = Subscription{
		Endpoint: testEndpoint,
		P256dh:   b64url.EncodeToString(p256dh),
		Auth:     b64url.EncodeToString(auth),
	}

	store, err := remote.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rg.subs, err = remote.NewSubscriptions(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := rg.subs.Add("dev1", rg.sub.Endpoint, rg.sub.P256dh, rg.sub.Auth); err != nil {
		t.Fatal(err)
	}

	rg.sender = &Sender{
		Client: client,
		Keys:   keys,
		Now:    rg.clock.Now,
		OnGone: func(endpoint string) { _, _ = rg.subs.Remove(endpoint) },
	}
	return rg
}

func respond(status int, headers ...string) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		for i := 0; i+1 < len(headers); i += 2 {
			w.Header().Set(headers[i], headers[i+1])
		}
		w.WriteHeader(status)
	}
}

func TestSendHeadersAndBody(t *testing.T) {
	var gotHdr http.Header
	var gotBody []byte
	var gotPath string
	rg := newRig(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
		gotHdr, gotBody, gotPath = r.Header.Clone(), body, r.URL.Path
		w.WriteHeader(http.StatusCreated)
	})
	payload := []byte(`{"title":"esperándote","body":"proyecto-demo"}`)
	topic := TopicFor("sesion-demo-1")
	if len(topic) != 22 {
		t.Fatalf("topic de %d chars, quería 22", len(topic))
	}

	res := rg.sender.Send(context.Background(), rg.sub, payload, topic)
	if res.Outcome != OK || res.Status != 201 {
		t.Fatalf("resultado = %+v", res)
	}
	if gotPath != "/wpush/v2/abc123" {
		t.Errorf("path = %q", gotPath)
	}

	auth := gotHdr.Get("Authorization")
	wantK := ", k=" + rg.sender.Keys.PublicKeyB64()
	if !strings.HasPrefix(auth, "vapid t=") || !strings.HasSuffix(auth, wantK) {
		t.Errorf("Authorization = %q", auth)
	}
	jwt := strings.TrimSuffix(strings.TrimPrefix(auth, "vapid t="), wantK)
	if n := strings.Count(jwt, "."); n != 2 {
		t.Errorf("JWT con %d puntos", n)
	}
	if got := decodeJSONPart(t, strings.Split(jwt, ".")[1])["aud"]; got != "https://push.ejemplo.test" {
		t.Errorf("aud = %v", got)
	}
	for k, want := range map[string]string{
		"Content-Encoding": "aes128gcm",
		"Content-Type":     "application/octet-stream",
		"Ttl":              "600",
		"Urgency":          "high",
		"Topic":            topic,
	} {
		if got := gotHdr.Get(k); got != want {
			t.Errorf("%s = %q, quería %q", k, got, want)
		}
	}
	if len(gotHdr.Get("Topic")) > 32 {
		t.Error("Topic > 32 chars")
	}

	// El navegador (clave privada del test) recupera el payload original.
	padded := browserDecrypt(t, gotBody, rg.uaPriv, rg.auth)
	if !bytes.Equal(padded, append(append([]byte(nil), payload...), 0x02)) {
		t.Errorf("descifrado = %q", padded)
	}
}

func TestSendGoneRemovesSubscription(t *testing.T) {
	for _, status := range []int{http.StatusGone, http.StatusNotFound} {
		rg := newRig(t, respond(status))
		res := rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
		if res.Outcome != Gone || res.Status != status {
			t.Fatalf("%d: resultado = %+v", status, res)
		}
		// Send solo informa; el borrado lo hace el worker vía OnGone.
		rg.sender.Start(t.Context())
		rg.sender.Enqueue(rg.sub, []byte("x"), "")
		waitFor(t, func() bool { return len(rg.subs.All()) == 0 })
	}
}

func TestSendRetryAfterPerHost(t *testing.T) {
	rg := newRig(t, respond(http.StatusTooManyRequests, "Retry-After", "120"))
	start := rg.clock.Now()

	res := rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
	if res.Outcome != RetryAfter || res.Status != 429 || !res.RetryAt.Equal(start.Add(120*time.Second)) {
		t.Fatalf("resultado = %+v", res)
	}
	if rg.hits.Load() != 1 {
		t.Fatalf("hits = %d", rg.hits.Load())
	}

	// A los 119 s sigue en pausa: no llega al servidor.
	rg.clock.Advance(119 * time.Second)
	res = rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
	if res.Outcome != RetryAfter || res.Status != 0 {
		t.Fatalf("en pausa: %+v", res)
	}
	if rg.hits.Load() != 1 {
		t.Fatalf("envió durante la pausa: hits = %d", rg.hits.Load())
	}

	// Cumplidos los 120 s, vuelve a enviar.
	rg.clock.Advance(time.Second)
	rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
	if rg.hits.Load() != 2 {
		t.Fatalf("no reanudó: hits = %d", rg.hits.Load())
	}
}

func TestSendServerErrorKeepsSubscription(t *testing.T) {
	rg := newRig(t, respond(http.StatusInternalServerError))
	rg.sender.Start(t.Context())
	rg.sender.Enqueue(rg.sub, []byte("x"), "")
	waitFor(t, func() bool { return rg.hits.Load() == 1 })
	time.Sleep(50 * time.Millisecond)
	if len(rg.subs.All()) != 1 {
		t.Fatal("un 500 borró la suscripción")
	}
	res := rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
	if res.Outcome != RetryAfter {
		t.Errorf("tras un 500 el host tiene que quedar en pausa: %+v", res)
	}
}

func TestSendOtherStatusIsErr(t *testing.T) {
	rg := newRig(t, respond(http.StatusRequestEntityTooLarge))
	res := rg.sender.Send(context.Background(), rg.sub, []byte("x"), "")
	if res.Outcome != Err || res.Err == nil || res.Status != 413 {
		t.Fatalf("resultado = %+v", res)
	}
	if len(rg.subs.All()) != 1 {
		t.Error("un 413 borró la suscripción")
	}
}

func TestSendRejectsBadInput(t *testing.T) {
	rg := newRig(t, respond(201))
	ctx := context.Background()
	cases := map[string]struct {
		sub     Subscription
		payload []byte
		topic   string
	}{
		"topic largo":    {rg.sub, []byte("x"), strings.Repeat("a", 33)},
		"topic inválido": {rg.sub, []byte("x"), "a b"},
		"payload grande": {rg.sub, make([]byte, MaxPlaintext+1), ""},
		"endpoint http":  {Subscription{Endpoint: "http://push.ejemplo.test/x", P256dh: rg.sub.P256dh, Auth: rg.sub.Auth}, []byte("x"), ""},
		"p256dh basura":  {Subscription{Endpoint: testEndpoint, P256dh: "AAAA", Auth: rg.sub.Auth}, []byte("x"), ""},
	}
	for name, c := range cases {
		if res := rg.sender.Send(ctx, c.sub, c.payload, c.topic); res.Outcome != Err {
			t.Errorf("%s: resultado = %+v", name, res)
		}
	}
	if rg.hits.Load() != 0 {
		t.Errorf("entradas inválidas llegaron al servidor: %d", rg.hits.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in   string
		want time.Duration
	}{
		{"120", 120 * time.Second},
		{"0", 0},
		{"", defaultRetry},
		{"basura", defaultRetry},
		{"-5", defaultRetry},
		{"99999999", maxRetryAfter},
		{now.Add(30 * time.Second).UTC().Format(http.TimeFormat), 30 * time.Second},
		{now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
	} {
		if got := parseRetryAfter(c.in, now); got != c.want {
			t.Errorf("%q → %v, quería %v", c.in, got, c.want)
		}
	}
}

func TestEnqueueNeverBlocksWhenFull(t *testing.T) {
	// Sin worker: la cola se llena y el aviso 65 se descarta sin bloquear.
	s := &Sender{}
	sub := Subscription{Endpoint: testEndpoint}
	done := make(chan []bool)
	go func() {
		var got []bool
		for i := 0; i < QueueSize+10; i++ {
			got = append(got, s.Enqueue(sub, []byte("x"), ""))
		}
		done <- got
	}()
	select {
	case got := <-done:
		for i, ok := range got {
			if want := i < QueueSize; ok != want {
				t.Fatalf("Enqueue #%d = %v, quería %v", i, ok, want)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Enqueue bloqueó con la cola llena")
	}
}

func TestEnqueueConcurrentWithWorkerRace(t *testing.T) {
	rg := newRig(t, respond(201))
	ctx, cancel := context.WithCancel(context.Background())
	rg.sender.Start(ctx)
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				rg.sender.Enqueue(rg.sub, []byte("x"), "")
			}
		}()
	}
	wg.Wait()
	waitFor(t, func() bool { return rg.hits.Load() > 0 })
	cancel()
	rg.sender.Wait()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condición no alcanzada a tiempo")
}

func TestSendDoesNotFollowRedirect(t *testing.T) {
	var target atomic.Int32
	tgt := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { target.Add(1) }))
	defer tgt.Close()
	rg := newRig(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		http.Redirect(w, r, tgt.URL+"/x", http.StatusTemporaryRedirect)
	})
	res := rg.sender.Send(context.Background(), rg.sub, []byte("hola"), "")
	if res.Outcome != Err || res.Status != http.StatusTemporaryRedirect {
		t.Fatalf("res = %+v, quiero Err con 307", res)
	}
	if target.Load() != 0 {
		t.Fatal("se siguió la redirección")
	}
}

func TestIsPublicAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1": false, "::1": false, "10.1.2.3": false, "172.16.0.1": false,
		"192.168.1.1": false, "169.254.1.1": false, "100.64.0.1": false,
		"100.127.255.254": false, "fd00::1": false, "::ffff:127.0.0.1": false,
		"0.0.0.0": false, "224.0.0.1": false, "fe80::1": false, "ff02::1": false,
		"8.8.8.8": true, "142.250.0.1": true, "2001:4860::1": true, "100.128.0.1": true,
	}
	for in, want := range cases {
		if got := isPublicAddr(netip.MustParseAddr(in)); got != want {
			t.Errorf("isPublicAddr(%s) = %v, quiero %v", in, got, want)
		}
	}
}

func TestDefaultSenderBlocksLoopback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	_, p256dh, auth := newSubscription(t)
	s := &Sender{Keys: newKeysT(t)}
	sub := Subscription{Endpoint: srv.URL + "/push", P256dh: b64url.EncodeToString(p256dh), Auth: b64url.EncodeToString(auth)}
	res := s.Send(context.Background(), sub, []byte("hola"), "")
	if res.Outcome != Err || !errors.Is(res.Err, errBlockedAddr) {
		t.Fatalf("res = %+v, quiero Err por destino bloqueado", res)
	}
	if hits.Load() != 0 {
		t.Fatal("el servidor loopback recibió la conexión")
	}
}
