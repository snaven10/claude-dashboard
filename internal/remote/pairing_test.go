package remote

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newPairing() (*Pairing, *clock) {
	c := &clock{t: time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)}
	return &Pairing{Now: c.Now}, c
}

func TestPairingTokenRedeemOnce(t *testing.T) {
	p, _ := newPairing()
	tok, code, exp, err := p.New()
	if err != nil {
		t.Fatal(err)
	}
	if exp.Sub(time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)) != 5*time.Minute {
		t.Errorf("TTL = %v", exp)
	}
	if len(code) != 9 || code[4] != '-' {
		t.Errorf("formato del código: %q", code)
	}
	if err := p.Redeem(tok, ""); err != nil {
		t.Fatalf("canje válido: %v", err)
	}
	if err := p.Redeem(tok, ""); !errors.Is(err, ErrInvalidPairing) {
		t.Errorf("reuso del token: %v", err)
	}
	if err := p.Redeem("", code); !errors.Is(err, ErrInvalidPairing) {
		t.Errorf("código tras el token: %v", err)
	}
}

func TestPairingCodeVariants(t *testing.T) {
	for _, mut := range []func(string) string{
		func(s string) string { return s },
		func(s string) string { return strings.ReplaceAll(s, "-", "") },
		func(s string) string { return strings.ToLower(s) },
		func(s string) string { return " " + strings.ToLower(strings.ReplaceAll(s, "-", " ")) + " " },
	} {
		p, _ := newPairing()
		_, code, _, _ := p.New()
		if err := p.Redeem("", mut(code)); err != nil {
			t.Errorf("variante %q de %q: %v", mut(code), code, err)
		}
	}
}

func TestPairingCodeThenTokenFails(t *testing.T) {
	p, _ := newPairing()
	tok, code, _, _ := p.New()
	if err := p.Redeem("", code); err != nil {
		t.Fatal(err)
	}
	if err := p.Redeem(tok, ""); !errors.Is(err, ErrInvalidPairing) {
		t.Errorf("token tras el código: %v", err)
	}
}

func TestPairingExpired(t *testing.T) {
	p, c := newPairing()
	tok, _, _, _ := p.New()
	c.t = c.t.Add(5*time.Minute + time.Second)
	if err := p.Redeem(tok, ""); !errors.Is(err, ErrExpiredPairing) {
		t.Errorf("vencido: %v", err)
	}
	// Justo en el límite (5 min exactos) ya no vale.
	p2, c2 := newPairing()
	tok2, _, _, _ := p2.New()
	c2.t = c2.t.Add(5 * time.Minute)
	if err := p2.Redeem(tok2, ""); err == nil {
		t.Error("el límite exacto no debería valer")
	}
	// Un segundo antes, sí.
	p3, c3 := newPairing()
	tok3, _, _, _ := p3.New()
	c3.t = c3.t.Add(5*time.Minute - time.Second)
	if err := p3.Redeem(tok3, ""); err != nil {
		t.Errorf("a 4:59: %v", err)
	}
}

func TestPairingNewInvalidatesPrevious(t *testing.T) {
	p, _ := newPairing()
	t1, _, _, _ := p.New()
	t2, _, _, _ := p.New()
	if err := p.Redeem(t1, ""); err == nil {
		t.Error("el token anterior sigue valiendo")
	}
	p3, _ := newPairing()
	_, c1b, _, _ := p3.New()
	_, _, _, _ = p3.New()
	if err := p3.Redeem("", c1b); err == nil {
		t.Error("el código anterior sigue valiendo")
	}
	if err := p.Redeem(t2, ""); err != nil {
		t.Errorf("el nuevo: %v", err)
	}
}

func TestPairingFiveFailuresInvalidate(t *testing.T) {
	p, _ := newPairing()
	tok, _, _, _ := p.New()
	for i := 0; i < MaxPairingFails; i++ {
		if err := p.Redeem("nope", ""); err == nil {
			t.Fatal("canje inválido aceptado")
		}
	}
	if err := p.Redeem(tok, ""); err == nil {
		t.Error("el sexto (correcto) debería fallar")
	}
	// Con 4 fallos el correcto todavía entra.
	p2, _ := newPairing()
	tok2, _, _, _ := p2.New()
	for i := 0; i < MaxPairingFails-1; i++ {
		_ = p2.Redeem("nope", "")
	}
	if err := p2.Redeem(tok2, ""); err != nil {
		t.Errorf("tras 4 fallos: %v", err)
	}
}

func TestPairingRejectsEmptyAndBoth(t *testing.T) {
	p, _ := newPairing()
	tok, code, _, _ := p.New()
	if err := p.Redeem("", ""); err == nil {
		t.Error("vacío aceptado")
	}
	if err := p.Redeem(tok, code); err == nil {
		t.Error("token y código juntos aceptados")
	}
	var zero Pairing
	if err := zero.Redeem("x", ""); err == nil {
		t.Error("sin pendiente aceptado")
	}
}

func TestPairingTokensAreUnique(t *testing.T) {
	p, _ := newPairing()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		tok, _, _, _ := p.New()
		if seen[tok] || len(tok) < 40 {
			t.Fatalf("token repetido o corto: %q", tok)
		}
		seen[tok] = true
	}
}
