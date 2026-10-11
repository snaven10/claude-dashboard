# TASK-032 — Cifrado aes128gcm de Web Push (RFC 8291 + RFC 8188)

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker — **modelo opus** (justificación en el master §3)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** — (paralela con TASK-030)
- **Estado:** `pending`

---

## Objetivo
`internal/webpush` cifra un payload para una suscripción Web Push con aes128gcm según RFC 8291,
solo con stdlib, y el resultado coincide byte a byte con el ejemplo del RFC.

## Contexto verificado
- Design **DD-50** (algoritmo, layout del cuerpo, firma interna testeable, tope de tamaño).
- `go.mod`: `go 1.27` → `crypto/ecdh` (1.20), `crypto/hkdf` (1.24: `hkdf.Key(sha256.New, secret, salt, info, n)`),
  `crypto/aes` + `crypto/cipher` (GCM). Cero dependencias.
- No existe todavía `internal/webpush/`.

## Archivos
- **Crear:** `internal/webpush/ece.go`, `internal/webpush/ece_test.go`, `internal/webpush/testdata/rfc8291.json` (vector público del RFC)

## Pasos
- [ ] **Paso 1 — vector.** Copiá del texto oficial del RFC 8291 (rfc-editor.org, §5 "Push Message
  Encryption Example" y Apéndice A "Intermediate Values for Encryption") los valores en b64url:
  plaintext, clave privada y pública del servidor de aplicación, clave pública del navegador,
  `auth`, `salt`, ECDH secret, PRK/IKM, CEK, NONCE y el mensaje cifrado final. **Literal, no de
  memoria.** Guardalos en `testdata/rfc8291.json` con una línea `source` que cite RFC 8291 §5/Apéndice A.
- [ ] **Paso 2 — `encrypt` interna** con la firma de DD-50 (salt y clave efímera inyectadas),
  exponiendo las intermedias en una función de test o un struct no exportado para comparar IKM/CEK/NONCE.
- [ ] **Paso 3 — `Encrypt(plaintext []byte, p256dh, auth []byte) ([]byte, error)` pública:** salt
  de 16 bytes con `crypto/rand` y `ecdh.P256().GenerateKey(rand.Reader)` NUEVOS en cada llamada.
- [ ] **Paso 4 — validaciones.** `p256dh` inválido (no en la curva, largo ≠ 65) → error (vía
  `ecdh.P256().NewPublicKey`); `auth` ≠ 16 bytes → error; plaintext > 3993 → error. Nunca panic.
- [ ] **Paso 5 — tests** (abajo). Agregá un comentario de cabecera en `ece.go` con el diagrama de
  DD-50 y las referencias a las secciones de los RFC.

## Criterios de aceptación
- [ ] Test vector: `encrypt` con las entradas del RFC produce exactamente el cuerpo del RFC; IKM,
  CEK y NONCE intermedios también coinciden.
- [ ] Test ida y vuelta: un `decrypt` de test (solo en `_test.go`, del lado navegador con la clave
  privada del UA generada en el test) recupera el plaintext de `Encrypt`.
- [ ] Test: dos `Encrypt` del mismo plaintext a la misma suscripción dan salt distintos, clave
  efímera (keyid) distinta y cuerpos distintos.
- [ ] Test: cabecera = `salt(16) || 00 00 10 00 || 41 || as_public(65)`; el último byte del
  plaintext descifrado es el delimitador `0x02`.
- [ ] Test: `p256dh` con un punto fuera de la curva, de 64 bytes, o `auth` de 15 bytes → error sin panic.
- [ ] Test: plaintext de 3993 bytes ok (cuerpo ≤ 4096), de 3994 → error.
- [ ] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados. `go.mod` sin `require`.

## Riesgos
- Errores silenciosos que el vector no ve (reuso de salt/clave, padding en el camino real): por eso
  los tests de unicidad y de ida y vuelta son obligatorios además del vector.

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
