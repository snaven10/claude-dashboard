package webpush

// VAPID (RFC 8292) con ES256 solo con la stdlib. Decisión: DD-51 de PLAN-011.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// jwtLifetime es la vigencia del JWT (RFC 8292 §2: ≤ 24 h).
	jwtLifetime = 12 * time.Hour
	// jwtReuseMargin: un JWT cacheado se reusa hasta 1 h antes de vencer.
	jwtReuseMargin = time.Hour

	vapidFileMode os.FileMode = 0o600
	vapidDirMode  os.FileMode = 0o700
)

var b64url = base64.RawURLEncoding

// vapidFile es el formato de vapid.json (DD-45).
type vapidFile struct {
	PrivateKey string `json:"privateKey"` // 32 bytes, b64url
	PublicKey  string `json:"publicKey"`  // 65 bytes sin comprimir, b64url
	CreatedAt  int64  `json:"createdAt"`  // epoch ms
}

type cachedJWT struct {
	token string
	exp   time.Time
}

// Keys es el par VAPID del daemon. Seguro para uso concurrente.
type Keys struct {
	// Subject es el claim "sub": la URL pública (https://…). Se inyecta.
	Subject string

	priv   *ecdsa.PrivateKey
	pubB64 string

	mu    sync.Mutex
	cache map[string]cachedJWT
}

// LoadOrCreateKeys carga vapid.json; si no existe genera un par nuevo y lo
// escribe con 0600. Si existe valida que la pública corresponda a la privada.
func LoadOrCreateKeys(path string) (*Keys, error) {
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		return parseKeys(path, raw)
	case errors.Is(err, fs.ErrNotExist):
		return createKeys(path)
	default:
		return nil, fmt.Errorf("webpush: leyendo %s: %w", path, err)
	}
}

func parseKeys(path string, raw []byte) (*Keys, error) {
	var f vapidFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("webpush: decodificando %s: %w", path, err)
	}
	privBytes, err := b64url.DecodeString(f.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("webpush: privateKey de %s: %w", path, err)
	}
	pubBytes, err := b64url.DecodeString(f.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("webpush: publicKey de %s: %w", path, err)
	}
	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), privBytes)
	if err != nil {
		return nil, fmt.Errorf("webpush: clave privada de %s inválida: %w", path, err)
	}
	pub, err := priv.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("webpush: serializando pública: %w", err)
	}
	if !bytes.Equal(pub, pubBytes) {
		return nil, fmt.Errorf("webpush: %s: la clave pública no corresponde a la privada", path)
	}
	// Corrige permisos abiertos sin abortar (DD-45).
	if st, err := os.Stat(path); err == nil && st.Mode().Perm()&^vapidFileMode != 0 {
		_ = os.Chmod(path, vapidFileMode)
	}
	return newKeys(priv, pub), nil
}

func createKeys(path string) (*Keys, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("webpush: generando clave VAPID: %w", err)
	}
	pub, err := priv.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("webpush: serializando pública: %w", err)
	}
	privBytes, err := priv.Bytes()
	if err != nil {
		return nil, fmt.Errorf("webpush: serializando privada: %w", err)
	}
	data, err := json.Marshal(vapidFile{
		PrivateKey: b64url.EncodeToString(privBytes),
		PublicKey:  b64url.EncodeToString(pub),
		CreatedAt:  time.Now().UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("webpush: codificando vapid.json: %w", err)
	}
	if err := writeFileAtomic(path, data); err != nil {
		return nil, err
	}
	return newKeys(priv, pub), nil
}

func newKeys(priv *ecdsa.PrivateKey, pub []byte) *Keys {
	return &Keys{priv: priv, pubB64: b64url.EncodeToString(pub), cache: map[string]cachedJWT{}}
}

// writeFileAtomic: temp en el mismo dir → fsync → chmod 0600 → rename.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, vapidDirMode); err != nil {
		return fmt.Errorf("webpush: creando %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("webpush: temporal para %s: %w", path, err)
	}
	name := tmp.Name()
	fail := func(err error) error {
		_ = os.Remove(name)
		return fmt.Errorf("webpush: escribiendo %s: %w", path, err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		return fail(err)
	}
	if err := os.Chmod(name, vapidFileMode); err != nil {
		return fail(err)
	}
	if err := os.Rename(name, path); err != nil {
		return fail(err)
	}
	return nil
}

// PublicKeyB64 es la pública sin comprimir (65 bytes) en b64url: el
// applicationServerKey del navegador y el "k=" de la cabecera.
func (k *Keys) PublicKeyB64() string { return k.pubB64 }

// JWT devuelve el JWT ES256 para el origen aud (scheme://host). Cachea por aud
// hasta 1 h antes de que venza.
func (k *Keys) JWT(aud string, now time.Time) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if c, ok := k.cache[aud]; ok && now.Before(c.exp.Add(-jwtReuseMargin)) {
		return c.token, nil
	}
	exp := now.Add(jwtLifetime)
	claims, err := json.Marshal(map[string]any{"aud": aud, "exp": exp.Unix(), "sub": k.Subject})
	if err != nil {
		return "", fmt.Errorf("webpush: claims: %w", err)
	}
	signing := b64url.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + b64url.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, k.priv, digest[:])
	if err != nil {
		return "", fmt.Errorf("webpush: firmando JWT: %w", err)
	}
	// JWS ES256 = r||s de 32+32 bytes, relleno a la izquierda (NO DER).
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	token := signing + "." + b64url.EncodeToString(sig)
	k.cache[aud] = cachedJWT{token: token, exp: exp}
	return token, nil
}
