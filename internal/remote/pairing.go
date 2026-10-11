package remote

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	// PairingTTL es la vida del emparejamiento pendiente (DD-46).
	PairingTTL = 5 * time.Minute
	// MaxPairingFails: canjes fallidos que invalidan el pendiente (DD-46).
	MaxPairingFails = 5

	// Crockford base32: sin I, L, O, U.
	crockford  = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	codeLength = 8
)

var (
	// ErrInvalidPairing: token/código incorrecto, ya usado, invalidado o inexistente.
	ErrInvalidPairing = errors.New("emparejamiento inválido")
	// ErrExpiredPairing: el pendiente venció (TTL de 5 min).
	ErrExpiredPairing = errors.New("emparejamiento vencido")
)

// Pairing guarda UN emparejamiento pendiente, solo en memoria (DD-46): token y
// código comparten un solo uso, vencen juntos y piden-uno-nuevo invalida el
// anterior. Del token y del código solo se conserva el SHA-256.
type Pairing struct {
	// Now es el reloj; nil = time.Now. Inyectable para tests.
	Now func() time.Time

	mu        sync.Mutex
	active    bool
	tokenHash [32]byte
	codeHash  [32]byte
	expires   time.Time
	fails     int
}

func (p *Pairing) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// New genera un emparejamiento nuevo e invalida el pendiente anterior. El
// código se devuelve con el formato de pantalla ABCD-EFGH.
func (p *Pairing) New() (token, code string, expires time.Time, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", time.Time{}, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)

	cb := make([]byte, codeLength)
	if _, err := rand.Read(cb); err != nil {
		return "", "", time.Time{}, err
	}
	chars := make([]byte, codeLength)
	for i, b := range cb {
		chars[i] = crockford[b&31] // 32 símbolos: sin sesgo de módulo
	}
	plain := string(chars)
	code = plain[:4] + "-" + plain[4:]

	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = true
	p.tokenHash = sha256.Sum256([]byte(token))
	p.codeHash = sha256.Sum256([]byte(plain))
	p.expires = p.now().Add(PairingTTL)
	p.fails = 0
	return token, code, p.expires, nil
}

// Redeem canjea el token o el código (exactamente uno). Éxito, vencimiento y
// el 5º fallo dejan el pendiente inservible: es de un solo uso.
func (p *Pairing) Redeem(token, code string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active {
		return ErrInvalidPairing
	}
	if !p.now().Before(p.expires) {
		p.active = false
		return ErrExpiredPairing
	}

	var got, want [32]byte
	switch {
	case token != "" && code == "":
		got, want = sha256.Sum256([]byte(token)), p.tokenHash
	case code != "" && token == "":
		got, want = sha256.Sum256([]byte(normalizeCode(code))), p.codeHash
	default: // ninguno o ambos
		return p.failLocked()
	}
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return p.failLocked()
	}
	p.active = false
	return nil
}

func (p *Pairing) failLocked() error {
	p.fails++
	if p.fails >= MaxPairingFails {
		p.active = false
	}
	return ErrInvalidPairing
}

// normalizeCode acepta minúsculas, guiones/espacios y las confusiones clásicas
// de Crockford (O→0, I/L→1).
func normalizeCode(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch r {
		case '-', ' ':
			continue
		case 'O':
			r = '0'
		case 'I', 'L':
			r = '1'
		}
		b.WriteRune(r)
	}
	return b.String()
}
