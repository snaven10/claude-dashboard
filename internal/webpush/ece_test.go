package webpush

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// rfcVector es testdata/rfc8291.json: el ejemplo de RFC 8291 §5 y los valores
// intermedios de su Apéndice A, en b64url sin padding.
type rfcVector struct {
	Source          string `json:"source"`
	Plaintext       string `json:"plaintext"`
	ASPublic        string `json:"as_public"`
	ASPrivate       string `json:"as_private"`
	UAPublic        string `json:"ua_public"`
	UAPrivate       string `json:"ua_private"`
	Salt            string `json:"salt"`
	AuthSecret      string `json:"auth_secret"`
	ECDHSecret      string `json:"ecdh_secret"`
	PRKKey          string `json:"prk_key"`
	KeyInfo         string `json:"key_info"`
	IKM             string `json:"ikm"`
	PRK             string `json:"prk"`
	CEKInfo         string `json:"cek_info"`
	CEK             string `json:"cek"`
	NonceInfo       string `json:"nonce_info"`
	Nonce           string `json:"nonce"`
	Header          string `json:"header"`
	PaddedPlaintext string `json:"padded_plaintext"`
	Ciphertext      string `json:"ciphertext"`
	Body            string `json:"body"`
}

func loadVector(t *testing.T) rfcVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "rfc8291.json"))
	if err != nil {
		t.Fatalf("leer vector: %v", err)
	}
	var v rfcVector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parsear vector: %v", err)
	}
	return v
}

func b64(t *testing.T, s string) []byte {
	t.Helper()
	if s == "" {
		t.Fatal("campo vacío en el vector")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("b64url %q: %v", s, err)
	}
	return b
}

func p256Private(t *testing.T, raw []byte) *ecdh.PrivateKey {
	t.Helper()
	k, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		t.Fatalf("clave privada P-256: %v", err)
	}
	return k
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// browserDecrypt hace el papel del navegador: descifra un cuerpo aes128gcm con
// la clave privada del UA y el auth de la suscripción. Es una implementación
// INDEPENDIENTE de ece.go a propósito: sigue el pseudocódigo de RFC 8291 §3.4
// con HMAC-SHA-256 crudo, sin crypto/hkdf y sin llamar a deriveKeys, para que
// un error simétrico (ua/as invertidos en key_info, info mal terminada, salt y
// auth cruzados) no pase el test de ida y vuelta.
//
// Devuelve el registro descifrado ENTERO, con el delimitador de padding.
func browserDecrypt(t *testing.T, body []byte, uaPriv *ecdh.PrivateKey, auth []byte) []byte {
	t.Helper()
	if len(body) < 16+4+1 {
		t.Fatalf("cuerpo de %d bytes: no entra ni la cabecera fija", len(body))
	}
	salt := body[:16]
	idlen := int(body[20])
	if len(body) < 21+idlen+16 {
		t.Fatalf("cuerpo de %d bytes: no entra keyid(%d) + tag", len(body), idlen)
	}
	asPublicRaw := body[21 : 21+idlen]
	record := body[21+idlen:]

	asPublic, err := ecdh.P256().NewPublicKey(asPublicRaw)
	if err != nil {
		t.Fatalf("keyid no es un punto P-256: %v", err)
	}
	secret, err := uaPriv.ECDH(asPublic)
	if err != nil {
		t.Fatalf("ecdh del navegador: %v", err)
	}

	// RFC 8291 §3.4, "For both".
	prkKey := hmacSHA256(auth, secret)
	keyInfo := append([]byte("WebPush: info\x00"), uaPriv.PublicKey().Bytes()...)
	keyInfo = append(keyInfo, asPublicRaw...)
	ikm := hmacSHA256(prkKey, append(keyInfo, 0x01))
	prk := hmacSHA256(salt, ikm)
	cek := hmacSHA256(prk, []byte("Content-Encoding: aes128gcm\x00\x01"))[:16]
	nonce := hmacSHA256(prk, []byte("Content-Encoding: nonce\x00\x01"))[:12]

	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	padded, err := gcm.Open(nil, nonce, record, nil)
	if err != nil {
		t.Fatalf("el navegador no pudo descifrar: %v", err)
	}
	return padded
}

// newSubscription genera lo que generaría un navegador al suscribirse.
func newSubscription(t *testing.T) (uaPriv *ecdh.PrivateKey, p256dh, auth []byte) {
	t.Helper()
	uaPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generar clave del UA: %v", err)
	}
	auth = make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatalf("generar auth: %v", err)
	}
	return uaPriv, uaPriv.PublicKey().Bytes(), auth
}

// El vector tiene que ser coherente consigo mismo antes de usarlo de oráculo:
// si hay un error de copia, que falle acá y diga dónde.
func TestRFC8291VectorIsSelfConsistent(t *testing.T) {
	v := loadVector(t)
	if v.Source == "" {
		t.Error("el vector no cita su fuente")
	}

	if got, want := string(b64(t, v.Plaintext)), "When I grow up, I want to be a watermelon"; got != want {
		t.Errorf("plaintext = %q, want %q", got, want)
	}
	if got, want := b64(t, v.PaddedPlaintext), append(b64(t, v.Plaintext), 0x02); !bytes.Equal(got, want) {
		t.Errorf("padded_plaintext no es plaintext || 0x02")
	}
	if got, want := b64(t, v.Body), append(b64(t, v.Header), b64(t, v.Ciphertext)...); !bytes.Equal(got, want) {
		t.Errorf("body (§5) no es header || ciphertext (Apéndice A)")
	}
	if got := len(b64(t, v.Header)); got != 86 {
		t.Errorf("header = %d bytes, want 86", got)
	}

	// Las claves públicas del vector son las de sus privadas.
	asPriv := p256Private(t, b64(t, v.ASPrivate))
	if !bytes.Equal(asPriv.PublicKey().Bytes(), b64(t, v.ASPublic)) {
		t.Errorf("as_public no corresponde a as_private")
	}
	uaPriv := p256Private(t, b64(t, v.UAPrivate))
	if !bytes.Equal(uaPriv.PublicKey().Bytes(), b64(t, v.UAPublic)) {
		t.Errorf("ua_public no corresponde a ua_private")
	}

	// Las info y los pasos de HMAC del Apéndice A, calculados a mano.
	keyInfo := append([]byte("WebPush: info\x00"), b64(t, v.UAPublic)...)
	keyInfo = append(keyInfo, b64(t, v.ASPublic)...)
	if !bytes.Equal(keyInfo, b64(t, v.KeyInfo)) {
		t.Errorf("key_info no es \"WebPush: info\" || 0x00 || ua_public || as_public")
	}
	if got, want := string(b64(t, v.CEKInfo)), "Content-Encoding: aes128gcm\x00"; got != want {
		t.Errorf("cek_info = %q, want %q", got, want)
	}
	if got, want := string(b64(t, v.NonceInfo)), "Content-Encoding: nonce\x00"; got != want {
		t.Errorf("nonce_info = %q, want %q", got, want)
	}
	if got := hmacSHA256(b64(t, v.AuthSecret), b64(t, v.ECDHSecret)); !bytes.Equal(got, b64(t, v.PRKKey)) {
		t.Errorf("prk_key no es HMAC(auth_secret, ecdh_secret)")
	}
	if got := hmacSHA256(b64(t, v.Salt), b64(t, v.IKM)); !bytes.Equal(got, b64(t, v.PRK)) {
		t.Errorf("prk no es HMAC(salt, ikm)")
	}

	// El decrypt de test se valida contra el RFC: si no abre el cuerpo del
	// RFC, no sirve como oráculo del test de ida y vuelta.
	padded := browserDecrypt(t, b64(t, v.Body), uaPriv, b64(t, v.AuthSecret))
	if !bytes.Equal(padded, b64(t, v.PaddedPlaintext)) {
		t.Errorf("browserDecrypt(body del RFC) = %q, want %q", padded, b64(t, v.PaddedPlaintext))
	}
}

func TestEncryptMatchesRFC8291Vector(t *testing.T) {
	v := loadVector(t)
	plaintext := b64(t, v.Plaintext)
	uaPublic := b64(t, v.UAPublic)
	auth := b64(t, v.AuthSecret)
	salt := b64(t, v.Salt)
	asPriv := p256Private(t, b64(t, v.ASPrivate))

	k, err := deriveKeys(uaPublic, auth, salt, asPriv)
	if err != nil {
		t.Fatalf("deriveKeys: %v", err)
	}
	intermediates := []struct {
		name string
		got  []byte
		want string
	}{
		{"as_public", k.asPublic, v.ASPublic},
		{"ecdh_secret", k.ecdhSecret, v.ECDHSecret},
		{"IKM", k.ikm, v.IKM},
		{"CEK", k.cek, v.CEK},
		{"NONCE", k.nonce, v.Nonce},
	}
	for _, tt := range intermediates {
		t.Run(tt.name, func(t *testing.T) {
			if want := b64(t, tt.want); !bytes.Equal(tt.got, want) {
				t.Errorf("%s = %x, want %x", tt.name, tt.got, want)
			}
		})
	}

	body, err := encrypt(plaintext, uaPublic, auth, salt, asPriv)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	want := b64(t, v.Body)
	if !bytes.Equal(body, want) {
		t.Fatalf("cuerpo distinto al del RFC 8291 §5\n got: %s\nwant: %s",
			base64.RawURLEncoding.EncodeToString(body), base64.RawURLEncoding.EncodeToString(want))
	}
	if !bytes.Equal(body[:headerLen], b64(t, v.Header)) {
		t.Errorf("cabecera distinta a la del Apéndice A")
	}
	if !bytes.Equal(body[headerLen:], b64(t, v.Ciphertext)) {
		t.Errorf("ciphertext distinto al del Apéndice A")
	}
}

func TestEncryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"vacío", []byte{}},
		{"nil", nil},
		{"un byte", []byte{0x41}},
		{"json del aviso", []byte(`{"v":1,"title":"demo — esperándote","body":"proyecto-demo","tag":"cd-abc","url":"/?s=00000000"}`)},
		{"termina en 0x02", []byte{0x10, 0x02}},
		{"termina en ceros", []byte{0x10, 0x00, 0x00}},
		{"tamaño máximo", bytes.Repeat([]byte{0xa5}, MaxPlaintext)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uaPriv, p256dh, auth := newSubscription(t)
			original := bytes.Clone(tt.plaintext)

			body, err := Encrypt(tt.plaintext, p256dh, auth)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if !bytes.Equal(tt.plaintext, original) {
				t.Errorf("Encrypt modificó el plaintext del llamador")
			}
			if got, want := len(body), headerLen+len(tt.plaintext)+1+tagLen; got != want {
				t.Errorf("len(body) = %d, want %d", got, want)
			}

			padded := browserDecrypt(t, body, uaPriv, auth)
			if len(padded) != len(tt.plaintext)+1 {
				t.Fatalf("registro descifrado de %d bytes, want %d (plaintext + delimitador, sin padding)", len(padded), len(tt.plaintext)+1)
			}
			if last := padded[len(padded)-1]; last != 0x02 {
				t.Errorf("delimitador = %#x, want 0x02", last)
			}
			if got := padded[:len(padded)-1]; !bytes.Equal(got, tt.plaintext) {
				t.Errorf("plaintext recuperado distinto del original")
			}
		})
	}
}

// Otra suscripción (otra clave del UA u otro auth) no tiene que poder abrir el
// mensaje: comprueba que auth y la clave del UA de verdad entran en la derivación.
func TestEncryptIsBoundToSubscription(t *testing.T) {
	uaPriv, p256dh, auth := newSubscription(t)
	body, err := Encrypt([]byte("hola"), p256dh, auth)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	open := func(priv *ecdh.PrivateKey, auth []byte) error {
		asPublic, err := ecdh.P256().NewPublicKey(body[21:headerLen])
		if err != nil {
			return err
		}
		secret, err := priv.ECDH(asPublic)
		if err != nil {
			return err
		}
		keyInfo := append([]byte("WebPush: info\x00"), priv.PublicKey().Bytes()...)
		keyInfo = append(keyInfo, body[21:headerLen]...)
		ikm := hmacSHA256(hmacSHA256(auth, secret), append(keyInfo, 0x01))
		prk := hmacSHA256(body[:16], ikm)
		block, err := aes.NewCipher(hmacSHA256(prk, []byte("Content-Encoding: aes128gcm\x00\x01"))[:16])
		if err != nil {
			return err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return err
		}
		_, err = gcm.Open(nil, hmacSHA256(prk, []byte("Content-Encoding: nonce\x00\x01"))[:12], body[headerLen:], nil)
		return err
	}

	if err := open(uaPriv, auth); err != nil {
		t.Fatalf("la suscripción correcta no abre el mensaje: %v", err)
	}
	otherPriv, _, otherAuth := newSubscription(t)
	if err := open(otherPriv, auth); err == nil {
		t.Errorf("otra clave del UA abrió el mensaje")
	}
	if err := open(uaPriv, otherAuth); err == nil {
		t.Errorf("otro auth abrió el mensaje")
	}
}

func TestEncryptUsesFreshSaltAndEphemeralKey(t *testing.T) {
	uaPriv, p256dh, auth := newSubscription(t)
	plaintext := []byte("mismo plaintext, misma suscripción")

	a, err := Encrypt(plaintext, p256dh, auth)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	b, err := Encrypt(plaintext, p256dh, auth)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if bytes.Equal(a[:16], b[:16]) {
		t.Errorf("mismo salt en dos mensajes: %x", a[:16])
	}
	if bytes.Equal(a[21:headerLen], b[21:headerLen]) {
		t.Errorf("misma clave efímera (keyid) en dos mensajes")
	}
	if bytes.Equal(a[headerLen:], b[headerLen:]) {
		t.Errorf("mismo ciphertext en dos mensajes")
	}
	if bytes.Equal(a, b) {
		t.Errorf("mismo cuerpo en dos mensajes")
	}
	for i, body := range [][]byte{a, b} {
		if got := browserDecrypt(t, body, uaPriv, auth); !bytes.Equal(got[:len(got)-1], plaintext) {
			t.Errorf("mensaje %d: plaintext recuperado distinto", i+1)
		}
	}

	// Sobre más mensajes: ni un salt ni una clave efímera repetidos.
	const n = 64
	salts := make(map[string]bool, n)
	keyids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		body, err := Encrypt(plaintext, p256dh, auth)
		if err != nil {
			t.Fatalf("Encrypt %d: %v", i, err)
		}
		salts[string(body[:16])] = true
		keyids[string(body[21:headerLen])] = true
	}
	if len(salts) != n {
		t.Errorf("%d salts distintos en %d mensajes", len(salts), n)
	}
	if len(keyids) != n {
		t.Errorf("%d claves efímeras distintas en %d mensajes", len(keyids), n)
	}
}

func TestEncryptHeaderLayout(t *testing.T) {
	uaPriv, p256dh, auth := newSubscription(t)
	plaintext := []byte("cabecera")

	body, err := Encrypt(plaintext, p256dh, auth)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if headerLen != 86 {
		t.Fatalf("headerLen = %d, want 86", headerLen)
	}
	if len(body) < 86 {
		t.Fatalf("cuerpo de %d bytes, menor que la cabecera", len(body))
	}

	// salt(16) || 00 00 10 00 || 41 || as_public(65)
	if bytes.Equal(body[:16], make([]byte, 16)) {
		t.Errorf("salt en ceros")
	}
	if got, want := body[16:20], []byte{0x00, 0x00, 0x10, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("rs = % x, want % x", got, want)
	}
	if got := binary.BigEndian.Uint32(body[16:20]); got != 4096 {
		t.Errorf("rs = %d, want 4096", got)
	}
	if body[20] != 0x41 {
		t.Errorf("idlen = %#x, want 0x41 (65)", body[20])
	}
	keyid := body[21:86]
	if keyid[0] != 0x04 {
		t.Errorf("keyid empieza con %#x, want 0x04 (punto sin comprimir)", keyid[0])
	}
	if _, err := ecdh.P256().NewPublicKey(keyid); err != nil {
		t.Errorf("keyid no es un punto P-256: %v", err)
	}
	if bytes.Equal(keyid, p256dh) {
		t.Errorf("keyid es la clave del navegador, tenía que ser la efímera del servidor")
	}

	// RFC 8291 §4: rs mayor que plaintext + delimitador + tag.
	if record := len(body) - 86; record >= 4096 {
		t.Errorf("registro de %d bytes, tiene que ser menor que rs", record)
	}

	padded := browserDecrypt(t, body, uaPriv, auth)
	if last := padded[len(padded)-1]; last != 0x02 {
		t.Errorf("último byte del plaintext descifrado = %#x, want 0x02", last)
	}
	if !bytes.Equal(padded, append(bytes.Clone(plaintext), 0x02)) {
		t.Errorf("registro descifrado = %x, want plaintext || 02", padded)
	}
}

func TestEncryptRejectsInvalidSubscription(t *testing.T) {
	v := loadVector(t)
	validKey := b64(t, v.UAPublic)
	validAuth := b64(t, v.AuthSecret)

	offCurve := bytes.Clone(validKey)
	offCurve[len(offCurve)-1] ^= 0x01 // misma X, Y corrida: ya no cumple la ecuación

	compressed := append([]byte{0x02}, validKey[1:33]...) // 33 bytes, forma comprimida
	wrongPrefix := bytes.Clone(validKey)
	wrongPrefix[0] = 0x03
	zeros := make([]byte, 65)
	zeroPoint := append([]byte{0x04}, make([]byte, 64)...) // (0,0) no está en P-256

	tests := []struct {
		name    string
		p256dh  []byte
		auth    []byte
		wantErr error
	}{
		{"punto fuera de la curva", offCurve, validAuth, errInvalidKey},
		{"p256dh de 64 bytes", validKey[:64], validAuth, errInvalidKey},
		{"p256dh de 64 bytes sin prefijo", validKey[1:], validAuth, errInvalidKey},
		{"p256dh de 66 bytes", append(bytes.Clone(validKey), 0x00), validAuth, errInvalidKey},
		{"p256dh comprimido", compressed, validAuth, errInvalidKey},
		{"p256dh con prefijo 0x03", wrongPrefix, validAuth, errInvalidKey},
		{"p256dh todo ceros", zeros, validAuth, errInvalidKey},
		{"p256dh punto (0,0)", zeroPoint, validAuth, errInvalidKey},
		{"p256dh punto en el infinito", []byte{0x00}, validAuth, errInvalidKey},
		{"p256dh vacío", []byte{}, validAuth, errInvalidKey},
		{"p256dh nil", nil, validAuth, errInvalidKey},
		{"auth de 15 bytes", validKey, validAuth[:15], errInvalidAuth},
		{"auth de 17 bytes", validKey, append(bytes.Clone(validAuth), 0x00), errInvalidAuth},
		{"auth vacío", validKey, []byte{}, errInvalidAuth},
		{"auth nil", validKey, nil, errInvalidAuth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			body, err := Encrypt([]byte("hola"), tt.p256dh, tt.auth)
			if err == nil {
				t.Fatalf("Encrypt aceptó la entrada, want error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if body != nil {
				t.Errorf("devolvió %d bytes junto con el error", len(body))
			}
		})
	}

	// Sanidad: la misma entrada sin romper sí cifra (los casos de arriba
	// fallan por lo que rompen, no porque el vector esté mal).
	if _, err := Encrypt([]byte("hola"), validKey, validAuth); err != nil {
		t.Fatalf("Encrypt con la suscripción válida: %v", err)
	}
}

// Basura de cualquier largo como p256dh/auth: error o éxito, nunca panic.
func TestEncryptNeverPanicsOnGarbage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic: %v", r)
		}
	}()
	for n := 0; n <= 130; n++ {
		garbage := make([]byte, n)
		if _, err := rand.Read(garbage); err != nil {
			t.Fatalf("rand: %v", err)
		}
		for _, prefix := range []byte{0x00, 0x02, 0x03, 0x04} {
			key := bytes.Clone(garbage)
			if n > 0 {
				key[0] = prefix
			}
			_, _ = Encrypt([]byte("x"), key, garbage)
			_, _ = Encrypt([]byte("x"), key, make([]byte, 16))
		}
	}
}

func TestEncryptPlaintextLimit(t *testing.T) {
	if MaxPlaintext != 3993 {
		t.Fatalf("MaxPlaintext = %d, want 3993 (RFC 8291 §4)", MaxPlaintext)
	}
	uaPriv, p256dh, auth := newSubscription(t)

	t.Run("3993 bytes entra", func(t *testing.T) {
		plaintext := bytes.Repeat([]byte{0x7a}, 3993)
		if len(plaintext) != 3993 {
			t.Fatalf("el plaintext de prueba mide %d, want 3993", len(plaintext))
		}
		body, err := Encrypt(plaintext, p256dh, auth)
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		if len(body) != 4096 {
			t.Errorf("len(body) = %d, want 4096 exactos", len(body))
		}
		padded := browserDecrypt(t, body, uaPriv, auth)
		if !bytes.Equal(padded[:len(padded)-1], plaintext) || padded[len(padded)-1] != 0x02 {
			t.Errorf("el plaintext máximo no sobrevive la ida y vuelta")
		}
	})

	t.Run("3994 bytes se rechaza", func(t *testing.T) {
		plaintext := bytes.Repeat([]byte{0x7a}, 3994)
		if len(plaintext) != 3994 {
			t.Fatalf("el plaintext de prueba mide %d, want 3994", len(plaintext))
		}
		body, err := Encrypt(plaintext, p256dh, auth)
		if !errors.Is(err, errPlaintextTooLarge) {
			t.Fatalf("error = %v, want %v", err, errPlaintextTooLarge)
		}
		if body != nil {
			t.Errorf("devolvió %d bytes junto con el error", len(body))
		}
	})
}

// La firma interna recibe salt y clave efímera: también los valida.
func TestEncryptInternalRejectsBadSaltAndSenderKey(t *testing.T) {
	_, p256dh, auth := newSubscription(t)
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generar clave: %v", err)
	}
	x25519, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generar clave X25519: %v", err)
	}
	salt := make([]byte, 16)

	tests := []struct {
		name    string
		salt    []byte
		asPriv  *ecdh.PrivateKey
		wantErr error
	}{
		{"salt de 15 bytes", salt[:15], asPriv, errInvalidSalt},
		{"salt de 17 bytes", make([]byte, 17), asPriv, errInvalidSalt},
		{"salt nil", nil, asPriv, errInvalidSalt},
		{"clave efímera nil", salt, nil, errInvalidSenderKey},
		{"clave efímera de otra curva", salt, x25519, errInvalidSenderKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			if _, err := encrypt([]byte("hola"), p256dh, auth, tt.salt, tt.asPriv); !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
