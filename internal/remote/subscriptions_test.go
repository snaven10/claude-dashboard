package remote

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	goodP256dh = base64.RawURLEncoding.EncodeToString(append([]byte{0x04}, make([]byte, 64)...))
	goodAuth   = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
)

const goodEndpoint = "https://push.ejemplo.test/wpush/v2/abc"

func newSubs(t *testing.T) (*Subscriptions, *Store, string) {
	t.Helper()
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSubscriptions(st)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	s.Now = c.Now
	return s, st, st.Path(SubscriptionsFile)
}

func TestSubscriptionsAddUpsertRemove(t *testing.T) {
	s, st, path := newSubs(t)
	if err := s.Add("dev1", goodEndpoint, goodP256dh, goodAuth); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("dev1", goodEndpoint+"2", goodP256dh, goodAuth); err != nil {
		t.Fatal(err)
	}
	if err := s.Add("dev2", goodEndpoint+"3", goodP256dh, goodAuth); err != nil {
		t.Fatal(err)
	}
	// upsert por endpoint: otro dispositivo se queda con la misma
	if err := s.Add("dev2", goodEndpoint, goodP256dh, goodAuth); err != nil {
		t.Fatal(err)
	}
	all := s.All()
	if len(all) != 3 {
		t.Fatalf("len = %d, quería 3", len(all))
	}
	for _, x := range all {
		if x.Endpoint == goodEndpoint && x.DeviceID != "dev2" {
			t.Errorf("upsert no reasignó: %+v", x)
		}
	}

	st2, err := NewSubscriptions(st)
	if err != nil {
		t.Fatal(err)
	}
	if len(st2.All()) != 3 {
		t.Error("no persistió")
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("subscriptions.json: %v %v", fi, err)
	}

	if ok, err := s.Remove(goodEndpoint + "2"); err != nil || !ok {
		t.Fatalf("Remove = %v %v", ok, err)
	}
	if ok, _ := s.Remove("https://no.ejemplo.test/x"); ok {
		t.Error("Remove de inexistente devolvió true")
	}
	if ok, err := s.RemoveDevice("dev2"); err != nil || !ok {
		t.Fatalf("RemoveDevice = %v %v", ok, err)
	}
	if got := s.All(); len(got) != 0 {
		t.Errorf("quedaron %+v", got)
	}
	if ok, _ := s.RemoveDevice("dev2"); ok {
		t.Error("RemoveDevice repetido devolvió true")
	}
}

func TestValidateEndpoint(t *testing.T) {
	bad := map[string]string{
		"http":          "http://push.ejemplo.test/x",
		"ipv4 loop":     "https://127.0.0.1/x",
		"ipv6 loop":     "https://[::1]/x",
		"localhost":     "https://localhost/x",
		"sub-localhost": "https://app.localhost/x",
		"ip doc":        "https://192.0.2.5/x",
		"ip con puerto": "https://10.0.0.1:8443/x",
		"host vacío":    "https:///x",
		"sin scheme":    "push.ejemplo.test/x",
		"credenciales":  "https://u:p@push.ejemplo.test/x",
		"largo":         "https://push.ejemplo.test/" + strings.Repeat("a", 1024),
		"vacío":         "",
	}
	for name, ep := range bad {
		if err := ValidateEndpoint(ep); err == nil {
			t.Errorf("%s: aceptó %q", name, ep)
		}
	}
	good := "https://push.ejemplo.test/" + strings.Repeat("a", 1024-len("https://push.ejemplo.test/"))
	if len(good) != 1024 {
		t.Fatalf("len good = %d", len(good))
	}
	for _, ep := range []string{goodEndpoint, good, "https://push.ejemplo.test:443/x"} {
		if err := ValidateEndpoint(ep); err != nil {
			t.Errorf("rechazó %q: %v", ep, err)
		}
	}
}

func TestSubscriptionsAddValidatesKeys(t *testing.T) {
	s, _, _ := newSubs(t)
	short := base64.RawURLEncoding.EncodeToString(make([]byte, 10))
	for name, args := range map[string][4]string{
		"p256dh corto":  {"dev1", goodEndpoint, short, goodAuth},
		"auth corto":    {"dev1", goodEndpoint, goodP256dh, short},
		"p256dh no b64": {"dev1", goodEndpoint, "***", goodAuth},
		"device vacío":  {"", goodEndpoint, goodP256dh, goodAuth},
		"endpoint http": {"dev1", "http://push.ejemplo.test/x", goodP256dh, goodAuth},
	} {
		if err := s.Add(args[0], args[1], args[2], args[3]); err == nil {
			t.Errorf("%s: aceptado", name)
		}
	}
	if len(s.All()) != 0 {
		t.Error("una suscripción inválida quedó guardada")
	}
}
