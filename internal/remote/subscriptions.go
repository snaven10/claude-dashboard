package remote

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// SubscriptionsFile es el archivo del store (DD-45).
	SubscriptionsFile = "subscriptions.json"

	maxEndpointLen = 1024
	p256dhLen      = 65
	authLen        = 16
)

// Subscription es una suscripción Web Push ligada a un dispositivo.
// P256dh y Auth van en b64url sin padding, como los entrega el navegador.
type Subscription struct {
	DeviceID  string `json:"deviceId"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	CreatedAt int64  `json:"createdAt"`
}

// Subscriptions es el store de suscripciones sobre subscriptions.json.
type Subscriptions struct {
	// Now es el reloj; nil = time.Now.
	Now func() time.Time

	store *Store
	mu    sync.Mutex
	list  []Subscription
}

// NewSubscriptions carga subscriptions.json (inexistente = vacío).
func NewSubscriptions(store *Store) (*Subscriptions, error) {
	s := &Subscriptions{store: store}
	if err := store.Read(SubscriptionsFile, &s.list); err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", SubscriptionsFile, err)
	}
	return s, nil
}

func (s *Subscriptions) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// ValidateEndpoint aplica DD-51: https, host no vacío, ni IP literal ni
// loopback/localhost (evita usar el daemon contra la red local), ≤ 1024.
func ValidateEndpoint(endpoint string) error {
	if len(endpoint) == 0 || len(endpoint) > maxEndpointLen {
		return fmt.Errorf("endpoint: longitud inválida (1..%d)", maxEndpointLen)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint: URL inválida: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("endpoint: tiene que ser https")
	}
	if u.User != nil {
		return errors.New("endpoint: no admite credenciales en la URL")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return errors.New("endpoint: host vacío")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return errors.New("endpoint: host local no permitido")
	}
	// Zona IPv6 (fe80::1%eth0) o IP literal en cualquier forma.
	bare, _, _ := strings.Cut(host, "%")
	if net.ParseIP(bare) != nil {
		return errors.New("endpoint: IP literal no permitida")
	}
	return nil
}

func validateKey(name, b64 string, want int) error {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(b64, "="))
	if err != nil {
		return fmt.Errorf("%s: b64url inválido: %w", name, err)
	}
	if len(raw) != want {
		return fmt.Errorf("%s: %d bytes, se esperaban %d", name, len(raw), want)
	}
	return nil
}

// Add registra o actualiza (upsert por endpoint) una suscripción. Valida el
// endpoint y el largo de las claves antes de guardar.
func (s *Subscriptions) Add(deviceID, endpoint, p256dh, auth string) error {
	if deviceID == "" {
		return errors.New("deviceId vacío")
	}
	if err := ValidateEndpoint(endpoint); err != nil {
		return err
	}
	if err := validateKey("p256dh", p256dh, p256dhLen); err != nil {
		return err
	}
	if err := validateKey("auth", auth, authLen); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]Subscription(nil), s.list...)
	sub := Subscription{
		DeviceID: deviceID, Endpoint: endpoint,
		P256dh: strings.TrimRight(p256dh, "="), Auth: strings.TrimRight(auth, "="),
		CreatedAt: s.now().UnixMilli(),
	}
	replaced := false
	for i := range next {
		if next[i].Endpoint == endpoint {
			next[i] = sub
			replaced = true
			break
		}
	}
	if !replaced {
		next = append(next, sub)
	}
	return s.commit(next)
}

// Remove borra la suscripción de ese endpoint; devuelve si existía.
func (s *Subscriptions) Remove(endpoint string) (bool, error) {
	return s.removeWhere(func(x Subscription) bool { return x.Endpoint == endpoint })
}

// RemoveDevice borra todas las suscripciones de un dispositivo.
func (s *Subscriptions) RemoveDevice(deviceID string) (bool, error) {
	return s.removeWhere(func(x Subscription) bool { return x.DeviceID == deviceID })
}

func (s *Subscriptions) removeWhere(match func(Subscription) bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]Subscription, 0, len(s.list))
	for _, x := range s.list {
		if !match(x) {
			next = append(next, x)
		}
	}
	if len(next) == len(s.list) {
		return false, nil
	}
	return true, s.commit(next)
}

// commit persiste next y, solo si escribió bien, reemplaza la lista en memoria.
// Se llama con s.mu tomado.
func (s *Subscriptions) commit(next []Subscription) error {
	if next == nil {
		next = []Subscription{}
	}
	if err := s.store.Write(SubscriptionsFile, next); err != nil {
		return fmt.Errorf("guardando %s: %w", SubscriptionsFile, err)
	}
	s.list = next
	return nil
}

// All devuelve una copia de las suscripciones.
func (s *Subscriptions) All() []Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Subscription(nil), s.list...)
}
