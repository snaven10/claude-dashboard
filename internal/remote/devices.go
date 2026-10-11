package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// DevicesFile es el archivo del store (DD-45).
	DevicesFile = "devices.json"
	// CookieName y CookieMaxAge: cookie de dispositivo de DD-46 (90 días).
	CookieName   = "cd_device"
	CookieMaxAge = 7776000

	maxNameRunes  = 40
	maxLoginRunes = 100
	touchEvery    = time.Minute
)

// Device es un teléfono emparejado. CookieHash es el SHA-256 (hex) de la
// cookie; la cookie en claro nunca se guarda. Las marcas son epoch ms.
type Device struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Login      string `json:"login"`
	CookieHash string `json:"cookieHash,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
}

// Devices es el store de dispositivos sobre devices.json.
type Devices struct {
	// Now es el reloj; nil = time.Now.
	Now func() time.Time

	store *Store
	mu    sync.Mutex
	list  []Device
	// persisted: último lastSeenAt escrito a disco por dispositivo (Touch ≤ 1/min).
	persisted map[string]time.Time
}

// NewDevices carga devices.json (inexistente = vacío).
func NewDevices(store *Store) (*Devices, error) {
	d := &Devices{store: store, persisted: map[string]time.Time{}}
	if err := store.Read(DevicesFile, &d.list); err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", DevicesFile, err)
	}
	return d, nil
}

func (d *Devices) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func hashCookie(cookie string) string {
	sum := sha256.Sum256([]byte(cookie))
	return hex.EncodeToString(sum[:])
}

// sanitize quita caracteres de control, recorta y limita a max runas.
func sanitize(s string, max int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// Create registra un dispositivo y devuelve la cookie en claro (única vez que
// existe fuera del teléfono).
func (d *Devices) Create(name, login string) (string, Device, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", Device{}, fmt.Errorf("generando cookie: %w", err)
	}
	idb := make([]byte, 8)
	if _, err := rand.Read(idb); err != nil {
		return "", Device{}, fmt.Errorf("generando id: %w", err)
	}
	cookie := base64.RawURLEncoding.EncodeToString(raw)
	name = sanitize(name, maxNameRunes)
	if name == "" {
		name = "dispositivo"
	}
	now := d.now()
	dev := Device{
		ID:         hex.EncodeToString(idb),
		Name:       name,
		Login:      sanitize(login, maxLoginRunes),
		CookieHash: hashCookie(cookie),
		CreatedAt:  now.UnixMilli(),
		LastSeenAt: now.UnixMilli(),
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	next := append(append([]Device(nil), d.list...), dev)
	if err := d.store.Write(DevicesFile, next); err != nil {
		return "", Device{}, err
	}
	d.list = next
	d.persisted[dev.ID] = now
	dev.CookieHash = ""
	return cookie, dev, nil
}

// Lookup busca por cookie en tiempo constante (compara todos los hashes).
func (d *Devices) Lookup(cookie string) (Device, bool) {
	if cookie == "" {
		return Device{}, false
	}
	want := []byte(hashCookie(cookie))
	d.mu.Lock()
	defer d.mu.Unlock()
	var found Device
	ok := false
	for _, dev := range d.list {
		if subtle.ConstantTimeCompare(want, []byte(dev.CookieHash)) == 1 {
			found, ok = dev, true
		}
	}
	found.CookieHash = ""
	return found, ok
}

// Has dice si el dispositivo sigue emparejado.
func (d *Devices) Has(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, dev := range d.list {
		if dev.ID == id {
			return true
		}
	}
	return false
}

// Touch actualiza lastSeenAt; escribe a disco a lo sumo una vez por minuto por
// dispositivo para no reescribir el archivo en cada SSE.
func (d *Devices) Touch(id string) {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.list {
		if d.list[i].ID != id {
			continue
		}
		d.list[i].LastSeenAt = now.UnixMilli()
		if last, ok := d.persisted[id]; ok && now.Sub(last) < touchEvery {
			return
		}
		if err := d.store.Write(DevicesFile, d.list); err == nil {
			d.persisted[id] = now
		}
		return
	}
}

// List devuelve los dispositivos sin cookieHash.
func (d *Devices) List() []Device {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Device, len(d.list))
	for i, dev := range d.list {
		dev.CookieHash = ""
		out[i] = dev
	}
	return out
}

// Revoke borra el dispositivo; false si no existía.
func (d *Devices) Revoke(id string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	idx := -1
	for i := range d.list {
		if d.list[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false, nil
	}
	next := make([]Device, 0, len(d.list)-1)
	next = append(next, d.list[:idx]...)
	next = append(next, d.list[idx+1:]...)
	if err := d.store.Write(DevicesFile, next); err != nil {
		return false, err
	}
	d.list = next
	delete(d.persisted, id)
	return true, nil
}
