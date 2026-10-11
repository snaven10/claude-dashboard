package remote

import (
	"os"
	"strings"
	"testing"
	"time"
)

func newDevices(t *testing.T) (*Devices, *clock, string) {
	t.Helper()
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	d, err := NewDevices(st)
	if err != nil {
		t.Fatal(err)
	}
	d.Now = c.Now
	return d, c, st.Path(DevicesFile)
}

func TestDevicesCreateLookupRevoke(t *testing.T) {
	d, _, path := newDevices(t)
	cookie, dev, err := d.Create("Mi tel", "ana@ejemplo.test")
	if err != nil {
		t.Fatal(err)
	}
	if dev.CookieHash != "" {
		t.Error("Create devolvió el hash")
	}
	got, ok := d.Lookup(cookie)
	if !ok || got.ID != dev.ID || got.CookieHash != "" {
		t.Fatalf("lookup: %+v %v", got, ok)
	}
	if _, ok := d.Lookup("otra"); ok {
		t.Error("cookie ajena aceptada")
	}
	if _, ok := d.Lookup(""); ok {
		t.Error("cookie vacía aceptada")
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), cookie) {
		t.Error("cookie en claro en disco")
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Errorf("modo %v", st.Mode().Perm())
	}
	for _, x := range d.List() {
		if x.CookieHash != "" {
			t.Error("List expone cookieHash")
		}
	}

	ok, err = d.Revoke(dev.ID)
	if err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}
	if _, found := d.Lookup(cookie); found {
		t.Error("cookie revocada sigue valiendo")
	}
	if ok, _ := d.Revoke(dev.ID); ok {
		t.Error("segunda revocación = true")
	}
}

func TestDevicesPersistAcrossReload(t *testing.T) {
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := NewDevices(st)
	cookie, dev, _ := d1.Create("a", "")
	d2, err := NewDevices(st)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := d2.Lookup(cookie); !ok || got.ID != dev.ID {
		t.Errorf("tras recargar: %+v %v", got, ok)
	}
}

func TestDevicesSanitizeName(t *testing.T) {
	d, _, _ := newDevices(t)
	long := strings.Repeat("é", 100)
	_, dev, _ := d.Create(long, "x")
	if n := len([]rune(dev.Name)); n != 40 {
		t.Errorf("runas = %d", n)
	}
	_, dev, _ = d.Create("a\x00b\x1b[31m\nc\t", "l\r\nogin")
	if strings.ContainsAny(dev.Name+dev.Login, "\x00\x1b\n\r\t") {
		t.Errorf("control chars: %q %q", dev.Name, dev.Login)
	}
	_, dev, _ = d.Create("  \x00 ", "")
	if dev.Name == "" {
		t.Error("nombre vacío sin default")
	}
}

func TestDevicesTouchWritesAtMostOncePerMinute(t *testing.T) {
	d, c, path := newDevices(t)
	_, dev, _ := d.Create("a", "")
	readSeen := func() int64 {
		var list []Device
		st, _ := NewStore(strings.TrimSuffix(path, "/"+DevicesFile))
		if err := st.Read(DevicesFile, &list); err != nil {
			t.Fatal(err)
		}
		return list[0].LastSeenAt
	}
	created := readSeen()

	c.t = c.t.Add(10 * time.Second)
	d.Touch(dev.ID)
	if got := readSeen(); got != created {
		t.Errorf("Touch a los 10 s escribió: %d != %d", got, created)
	}
	if l := d.List()[0].LastSeenAt; l != c.t.UnixMilli() {
		t.Errorf("en memoria no se actualizó: %d", l)
	}
	c.t = c.t.Add(61 * time.Second)
	d.Touch(dev.ID)
	if got := readSeen(); got != c.t.UnixMilli() {
		t.Errorf("Touch tras 1 min no escribió: %d", got)
	}
	d.Touch("no-existe") // no debe entrar en pánico
}
