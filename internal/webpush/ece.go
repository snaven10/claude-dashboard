// Package webpush implementa, solo con la stdlib, lo que hace falta para mandar
// un aviso Web Push. Este archivo es el cifrado del payload: "aes128gcm" según
// RFC 8291 (Message Encryption for Web Push) sobre RFC 8188 (Encrypted
// Content-Encoding for HTTP). Decisión de diseño: DD-50 de PLAN-011.
//
// Algoritmo (RFC 8291 §3.4 + RFC 8188 §2):
//
//	ua_public   = p256dh de la suscripción (65 bytes, punto P-256 sin comprimir)
//	auth        = auth de la suscripción (16 bytes)                      RFC 8291 §3.2
//	as_priv     = ecdh.P256().GenerateKey(rand)     # NUEVO por mensaje  RFC 8291 §3.1
//	salt        = 16 bytes de crypto/rand           # NUEVO por mensaje  RFC 8188 §2.1
//	ecdh_secret = ECDH(as_priv, ua_public)                               RFC 8291 §3.1
//	key_info    = "WebPush: info" || 0x00 || ua_public || as_public      RFC 8291 §3.3
//	IKM         = HKDF(salt=auth, ikm=ecdh_secret, info=key_info, L=32)  RFC 8291 §3.3
//	CEK         = HKDF(salt=salt, ikm=IKM,
//	                   info="Content-Encoding: aes128gcm" || 0x00, L=16) RFC 8188 §2.2
//	NONCE       = HKDF(salt=salt, ikm=IKM,
//	                   info="Content-Encoding: nonce" || 0x00, L=12)     RFC 8188 §2.3
//	record      = AES-128-GCM(CEK, NONCE, plaintext || 0x02)             RFC 8188 §2
//	body        = salt(16) || rs(4, BE = 4096) || idlen(1 = 65)
//	              || as_public(65) || record                             RFC 8188 §2.1
//
// Un solo registro (RFC 8291 §4): por eso el nonce no se combina con el número
// de secuencia (el del primer registro es 0) y el delimitador de padding es
// 0x02, el del ÚLTIMO registro (RFC 8188 §2), no 0x01.
//
// Lo que el vector del RFC no puede probar y este código tiene que garantizar:
//
//   - salt y clave efímera NUEVOS por mensaje. CEK y NONCE salen de
//     (salt, as_priv, suscripción): repetirlos repite el par clave/nonce de
//     AES-GCM, y eso rompe confidencialidad y autenticidad. Por eso Encrypt no
//     acepta ninguno de los dos como parámetro.
//   - la clave pública del navegador se valida (está en la curva) antes del ECDH.
package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	saltLen      = 16 // RFC 8188 §2.1
	authLen      = 16 // RFC 8291 §3.2
	publicKeyLen = 65 // punto P-256 sin comprimir: 0x04 || X(32) || Y(32)
	tagLen       = 16 // expansión de AEAD_AES_128_GCM
	cekLen       = 16
	nonceLen     = 12

	// recordSize es el "rs" de la cabecera. RFC 8291 §4: tiene que ser mayor
	// que plaintext + delimitador + tag; con el tope de MaxPlaintext siempre lo es.
	recordSize = 4096

	// headerLen = salt(16) + rs(4) + idlen(1) + keyid(65) = 86.
	headerLen = saltLen + 4 + 1 + publicKeyLen

	// MaxPlaintext es el plaintext más largo que entra en un cuerpo de 4096
	// bytes, lo máximo que un servicio de push está obligado a aceptar
	// (RFC 8030 §7.2; cuenta de RFC 8291 §4): 4096 − 86 de cabecera − 16 de
	// tag − 1 de delimitador = 3993.
	MaxPlaintext = recordSize - headerLen - tagLen - 1

	// paddingDelimiter cierra el último (y único) registro. RFC 8188 §2.
	paddingDelimiter = 0x02

	keyInfoPrefix = "WebPush: info\x00"
	cekInfo       = "Content-Encoding: aes128gcm\x00"
	nonceInfo     = "Content-Encoding: nonce\x00"
)

// Errores centinela, no exportados: los tests los usan para comprobar que una
// entrada inválida se rechaza por el motivo correcto y no por casualidad.
var (
	errInvalidKey        = errors.New("p256dh inválido")
	errInvalidAuth       = errors.New("auth inválido")
	errInvalidSalt       = errors.New("salt inválido")
	errInvalidSenderKey  = errors.New("clave efímera inválida")
	errPlaintextTooLarge = errors.New("plaintext demasiado grande")
)

// Encrypt cifra plaintext para una suscripción Web Push y devuelve el cuerpo
// de la petición al servicio de push (Content-Encoding: aes128gcm).
//
// p256dh y auth son los campos de la suscripción ya decodificados de b64url:
// 65 y 16 bytes. Cada llamada genera un salt y un par ECDH efímero nuevos, así
// que dos llamadas con las mismas entradas dan cuerpos distintos.
//
// Devuelve error, nunca panic, si p256dh no es un punto válido de P-256, si
// auth no mide 16 bytes o si plaintext supera MaxPlaintext.
func Encrypt(plaintext, p256dh, auth []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("webpush: generar salt: %w", err)
	}
	asPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("webpush: generar clave efímera: %w", err)
	}
	return encrypt(plaintext, p256dh, auth, salt, asPriv)
}

// encrypt es Encrypt con el salt y la clave efímera inyectados. Existe para
// poder reproducir el vector del RFC 8291; fuera de los tests solo la llama
// Encrypt, con valores recién generados. NO reusar salt ni asPriv.
func encrypt(plaintext, uaPublic, auth, salt []byte, asPriv *ecdh.PrivateKey) ([]byte, error) {
	if len(plaintext) > MaxPlaintext {
		return nil, fmt.Errorf("webpush: %w: %d bytes, máximo %d", errPlaintextTooLarge, len(plaintext), MaxPlaintext)
	}
	k, err := deriveKeys(uaPublic, auth, salt, asPriv)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(k.cek)
	if err != nil {
		return nil, fmt.Errorf("webpush: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("webpush: gcm: %w", err)
	}

	// Copia propia: el plaintext del llamador no se toca.
	padded := make([]byte, len(plaintext)+1)
	copy(padded, plaintext)
	padded[len(plaintext)] = paddingDelimiter

	body := make([]byte, 0, headerLen+len(padded)+tagLen)
	body = append(body, salt...)
	body = binary.BigEndian.AppendUint32(body, recordSize)
	body = append(body, publicKeyLen)
	body = append(body, k.asPublic...)
	return gcm.Seal(body, k.nonce, padded, nil), nil
}

// keys son los valores intermedios de la derivación, con los nombres del
// Apéndice A del RFC 8291. El struct no se exporta: sirve para que el test del
// vector compare cada paso y no solo el resultado final.
type keys struct {
	asPublic   []byte // clave pública efímera sin comprimir (el "keyid")
	ecdhSecret []byte
	ikm        []byte
	cek        []byte
	nonce      []byte
}

// deriveKeys valida las entradas y hace la derivación de RFC 8291 §3.4.
func deriveKeys(uaPublic, auth, salt []byte, asPriv *ecdh.PrivateKey) (keys, error) {
	if len(auth) != authLen {
		return keys{}, fmt.Errorf("webpush: %w: %d bytes, se esperaban %d", errInvalidAuth, len(auth), authLen)
	}
	if len(salt) != saltLen {
		return keys{}, fmt.Errorf("webpush: %w: %d bytes, se esperaban %d", errInvalidSalt, len(salt), saltLen)
	}
	if asPriv == nil || asPriv.Curve() != ecdh.P256() {
		return keys{}, fmt.Errorf("webpush: %w: tiene que ser P-256", errInvalidSenderKey)
	}
	// El largo se mira acá para que el error sea claro y para no depender de
	// qué codificaciones acepte NewPublicKey: RFC 8291 §4 exige los 65 bytes
	// sin comprimir. NewPublicKey comprueba que el punto esté en la curva.
	if len(uaPublic) != publicKeyLen || uaPublic[0] != 0x04 {
		return keys{}, fmt.Errorf("webpush: %w: se esperaban %d bytes sin comprimir (0x04 || X || Y)", errInvalidKey, publicKeyLen)
	}
	uaKey, err := ecdh.P256().NewPublicKey(uaPublic)
	if err != nil {
		return keys{}, fmt.Errorf("webpush: %w: %w", errInvalidKey, err)
	}

	secret, err := asPriv.ECDH(uaKey)
	if err != nil {
		return keys{}, fmt.Errorf("webpush: ecdh: %w", err)
	}
	asPublic := asPriv.PublicKey().Bytes()

	keyInfo := keyInfoPrefix + string(uaPublic) + string(asPublic)
	ikm, err := hkdf.Key(sha256.New, secret, auth, keyInfo, sha256.Size)
	if err != nil {
		return keys{}, fmt.Errorf("webpush: derivar IKM: %w", err)
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, cekInfo, cekLen)
	if err != nil {
		return keys{}, fmt.Errorf("webpush: derivar CEK: %w", err)
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, nonceInfo, nonceLen)
	if err != nil {
		return keys{}, fmt.Errorf("webpush: derivar NONCE: %w", err)
	}
	return keys{asPublic: asPublic, ecdhSecret: secret, ikm: ikm, cek: cek, nonce: nonce}, nil
}
