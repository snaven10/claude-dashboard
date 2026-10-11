package webpush

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newKeysT(t *testing.T) *Keys {
	t.Helper()
	k, err := LoadOrCreateKeys(filepath.Join(t.TempDir(), "vapid.json"))
	if err != nil {
		t.Fatal(err)
	}
	k.Subject = "https://mi-pc.tailnet-ejemplo.ts.net"
	return k
}

func decodeJSONPart(t *testing.T, part string) map[string]any {
	t.Helper()
	raw, err := b64url.DecodeString(part)
	if err != nil {
		t.Fatalf("parte no b64url: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parte no JSON: %v", err)
	}
	return m
}

func TestJWTClaimsAndSignature(t *testing.T) {
	k := newKeysT(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	aud := "https://push.ejemplo.test"
	tok, err := k.JWT(aud, now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT con %d partes", len(parts))
	}

	hdr := decodeJSONPart(t, parts[0])
	if len(hdr) != 2 || hdr["typ"] != "JWT" || hdr["alg"] != "ES256" {
		t.Errorf("header = %v", hdr)
	}
	cl := decodeJSONPart(t, parts[1])
	if len(cl) != 3 || cl["aud"] != aud || cl["sub"] != k.Subject {
		t.Errorf("claims = %v", cl)
	}
	if exp, _ := cl["exp"].(float64); int64(exp) != now.Add(12*time.Hour).Unix() {
		t.Errorf("exp = %v, quería %d", cl["exp"], now.Add(12*time.Hour).Unix())
	}

	sig, err := b64url.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != 64 {
		t.Fatalf("firma de %d bytes, quería 64 (r||s, no DER)", len(sig))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&k.priv.PublicKey, digest[:], r, s) {
		t.Fatal("la firma no verifica con la pública")
	}
	// La firma tiene que fallar si cambia el contenido.
	other := sha256.Sum256([]byte(parts[0] + "." + parts[1] + "x"))
	if ecdsa.Verify(&k.priv.PublicKey, other[:], r, s) {
		t.Error("la firma verificó sobre otro contenido")
	}
	// Y la pública del header Authorization es la de la clave.
	pub, _ := k.priv.PublicKey.Bytes()
	if k.PublicKeyB64() != b64url.EncodeToString(pub) || len(pub) != 65 {
		t.Errorf("PublicKeyB64 inconsistente (%d bytes)", len(pub))
	}
	// Una firma DER (ASN.1) NO pasa el parser r||s: mide ≠ 64 bytes.
	der, err := ecdsa.SignASN1(rand.Reader, k.priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(der) == 64 {
		t.Error("una firma DER no debería medir 64 bytes")
	}
}

func TestJWTCachePerAudience(t *testing.T) {
	k := newKeysT(t)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	a1, _ := k.JWT("https://a.ejemplo.test", now)
	a2, _ := k.JWT("https://a.ejemplo.test", now.Add(10*time.Hour)) // exp-1h = +11h → cache
	if a1 != a2 {
		t.Error("no reusó el JWT dentro de la ventana")
	}
	a3, _ := k.JWT("https://a.ejemplo.test", now.Add(11*time.Hour+time.Minute)) // pasó exp-1h
	if a3 == a1 {
		t.Error("reusó un JWT a menos de 1 h de vencer")
	}
	b, _ := k.JWT("https://b.ejemplo.test", now)
	if b == a1 {
		t.Error("compartió JWT entre audiencias")
	}
	if cl := decodeJSONPart(t, strings.Split(b, ".")[1]); cl["aud"] != "https://b.ejemplo.test" {
		t.Errorf("aud = %v", cl["aud"])
	}
}

func TestLoadOrCreateKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "vapid.json")
	k1, err := LoadOrCreateKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("modo %v, quería 0600", st.Mode().Perm())
	}
	k2, err := LoadOrCreateKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	if k1.PublicKeyB64() != k2.PublicKeyB64() || !k1.priv.Equal(k2.priv) {
		t.Error("la segunda carga devolvió otra clave")
	}
}

func TestLoadKeysRejectsMismatchedPublic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vapid.json")
	if _, err := LoadOrCreateKeys(path); err != nil {
		t.Fatal(err)
	}
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _ := other.PublicKey.Bytes()
	raw, _ := os.ReadFile(path)
	var f vapidFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	f.PublicKey = b64url.EncodeToString(otherPub)
	data, _ := json.Marshal(f)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateKeys(path); err == nil {
		t.Fatal("aceptó una pública que no corresponde a la privada")
	}
}

func TestLoadKeysRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vapid.json")
	for _, content := range []string{"", "no json", `{"privateKey":"!!","publicKey":"!!"}`, `{"privateKey":"AAAA","publicKey":"AAAA"}`} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadOrCreateKeys(path); err == nil {
			t.Errorf("aceptó %q", content)
		}
	}
}
