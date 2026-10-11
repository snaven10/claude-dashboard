# TASK-032 — Cifrado aes128gcm de Web Push (RFC 8291 + RFC 8188)

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker — **modelo opus** (justificación en el master §3)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** — (paralela con TASK-030)
- **Estado:** `done`

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
- [x] **Paso 1 — vector.** Copiá del texto oficial del RFC 8291 (rfc-editor.org, §5 "Push Message
  Encryption Example" y Apéndice A "Intermediate Values for Encryption") los valores en b64url:
  plaintext, clave privada y pública del servidor de aplicación, clave pública del navegador,
  `auth`, `salt`, ECDH secret, PRK/IKM, CEK, NONCE y el mensaje cifrado final. **Literal, no de
  memoria.** Guardalos en `testdata/rfc8291.json` con una línea `source` que cite RFC 8291 §5/Apéndice A.
- [x] **Paso 2 — `encrypt` interna** con la firma de DD-50 (salt y clave efímera inyectadas),
  exponiendo las intermedias en una función de test o un struct no exportado para comparar IKM/CEK/NONCE.
- [x] **Paso 3 — `Encrypt(plaintext []byte, p256dh, auth []byte) ([]byte, error)` pública:** salt
  de 16 bytes con `crypto/rand` y `ecdh.P256().GenerateKey(rand.Reader)` NUEVOS en cada llamada.
- [x] **Paso 4 — validaciones.** `p256dh` inválido (no en la curva, largo ≠ 65) → error (vía
  `ecdh.P256().NewPublicKey`); `auth` ≠ 16 bytes → error; plaintext > 3993 → error. Nunca panic.
- [x] **Paso 5 — tests** (abajo). Agregá un comentario de cabecera en `ece.go` con el diagrama de
  DD-50 y las referencias a las secciones de los RFC.

## Criterios de aceptación
- [x] Test vector: `encrypt` con las entradas del RFC produce exactamente el cuerpo del RFC; IKM,
  CEK y NONCE intermedios también coinciden.
- [x] Test ida y vuelta: un `decrypt` de test (solo en `_test.go`, del lado navegador con la clave
  privada del UA generada en el test) recupera el plaintext de `Encrypt`.
- [x] Test: dos `Encrypt` del mismo plaintext a la misma suscripción dan salt distintos, clave
  efímera (keyid) distinta y cuerpos distintos.
- [x] Test: cabecera = `salt(16) || 00 00 10 00 || 41 || as_public(65)`; el último byte del
  plaintext descifrado es el delimitador `0x02`.
- [x] Test: `p256dh` con un punto fuera de la curva, de 64 bytes, o `auth` de 15 bytes → error sin panic.
- [x] Test: plaintext de 3993 bytes ok (cuerpo ≤ 4096), de 3994 → error.
- [x] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados. `go.mod` sin `require`.

## Riesgos
- Errores silenciosos que el vector no ve (reuso de salt/clave, padding en el camino real): por eso
  los tests de unicidad y de ida y vuelta son obligatorios además del vector.

## Resultado
- **Estado final:** `done` (2026-10-11)
- **Resumen:** `internal/webpush` cifra un payload Web Push con aes128gcm (RFC 8291 §3.4 + RFC 8188 §2)
  solo con stdlib (`crypto/ecdh`, `crypto/hkdf`, `crypto/aes`, `crypto/cipher`). `Encrypt(plaintext,
  p256dh, auth)` genera salt y par ECDH efímero nuevos en cada llamada y delega en
  `encrypt(plaintext, uaPublic, auth, salt, asPriv)`; la derivación vive en `deriveKeys`, que devuelve
  el struct no exportado `keys` (as_public, ecdh_secret, IKM, CEK, NONCE). Con las entradas del RFC
  8291 §5 el cuerpo sale idéntico byte a byte (144 bytes) y las intermedias coinciden con el Apéndice A.
  Constante exportada `MaxPlaintext = 3993` para que el notificador (TASK-034, DD-53) trunque contra el
  mismo número.
- **Archivos tocados:** (commit `90720a9` — `feat(webpush): cifrado aes128gcm de Web Push (RFC 8291 + RFC 8188) en stdlib`)
  - `internal/webpush/ece.go` (nuevo, 195 líneas)
  - `internal/webpush/ece_test.go` (nuevo, 614 líneas, 10 tests)
  - `internal/webpush/testdata/rfc8291.json` (nuevo; vector público del RFC)
  - este TASK (`Estado` + `Resultado`), en el commit `docs(plan)` siguiente
- **Verificado por:** corrido en el worktree `feature/plan-11-task-032`, Go 1.27.1, 2026-10-11:
  ```
  $ gofmt -l .
  (vacío, exit 0)
  $ go vet ./...
  (sin salida, exit 0)
  $ go test -race ./...
  ?   	claude-dashboard/cmd/claude-dashboard	[no test files]
  ok  	claude-dashboard/internal/agents	1.092s
  ok  	claude-dashboard/internal/history	1.175s
  ok  	claude-dashboard/internal/orq	1.074s
  ok  	claude-dashboard/internal/plans	1.119s
  ok  	claude-dashboard/internal/server	1.848s
  ok  	claude-dashboard/internal/transcript	10.424s
  ok  	claude-dashboard/internal/webpush	1.297s
  ?   	claude-dashboard/web	[no test files]
  $ cat go.mod
  module claude-dashboard

  go 1.27
  ```
  Criterio → test (todos en `ece_test.go`, todos corridos y en verde):
  - vector + IKM/CEK/NONCE → `TestEncryptMatchesRFC8291Vector` (y `TestRFC8291VectorIsSelfConsistent`,
    que comprueba que el JSON es coherente consigo mismo: body = header || ciphertext, públicas =
    de sus privadas, key_info/cek_info/nonce_info, PRK_key y PRK por HMAC)
  - ida y vuelta → `TestEncryptRoundTrip` (7 casos, incluye vacío y 3993 bytes) con `browserDecrypt`
  - unicidad → `TestEncryptUsesFreshSaltAndEphemeralKey` (2 mensajes + 64 sin salt ni keyid repetido)
  - cabecera y delimitador 0x02 → `TestEncryptHeaderLayout`
  - entradas inválidas sin panic → `TestEncryptRejectsInvalidSubscription` (15 casos, con `errors.Is`
    contra el centinela correcto) + `TestEncryptNeverPanicsOnGarbage`
  - 3993 ok (cuerpo = 4096 exactos) / 3994 error → `TestEncryptPlaintextLimit`
  - extra: `TestEncryptIsBoundToSubscription` (otra clave del UA u otro auth no abren el mensaje) y
    `TestEncryptInternalRejectsBadSaltAndSenderKey`

  **Los tests tienen dientes** (comprobado rompiendo `ece.go` a propósito y restaurándolo; cada
  mutación hizo fallar al menos un test): delimitador `0x01`; `key_info` con as/ua invertidos; salt
  sin aleatorizar en `Encrypt`; `auth` y `salt` cruzados en el HKDF; sin chequeo del largo de `auth`;
  tope en 3994; `rs` = 4097; info del NONCE = info del CEK. El caso "salt fijo" solo lo atrapan los
  tests de unicidad y de cabecera, no el vector: confirma el riesgo que anotaba la task.
- **Desviaciones:**
  - **Firma pública:** la task pide `Encrypt(plaintext []byte, p256dh, auth []byte)`; DD-50 la escribe
    abreviada como `Encrypt(plaintext, sub)` sin definir el tipo `sub`. Se implementó la de la task:
    el tipo de la suscripción nace en TASK-033 (`internal/remote/subscriptions.go`) y este paquete no
    debe importarlo. TASK-033 llama con los dos campos ya decodificados de b64url.
  - **`browserDecrypt` no reusa `deriveKeys`:** sigue el pseudocódigo de RFC 8291 §3.4 con HMAC-SHA-256
    crudo (sin `crypto/hkdf`), a propósito, para que un error simétrico no pase la ida y vuelta. Antes
    de usarlo de oráculo se valida abriendo el cuerpo del propio RFC con la `ua_private` del vector.
  - **Validación más estricta que "largo ≠ 65":** además se exige el prefijo `0x04` antes de
    `ecdh.P256().NewPublicKey`, y la firma interna valida salt (16) y que `asPriv` sea P-256 no nil.
  - **Errores centinela no exportados** (`errInvalidKey`, `errInvalidAuth`, `errInvalidSalt`,
    `errInvalidSenderKey`, `errPlaintextTooLarge`): no amplían la API; si TASK-033/034 necesitan
    distinguirlos, se exportan ahí.
- **Riesgos abiertos / siguiente:**
  - **El §5 del RFC dice `Content-Length: 145`, pero el cuerpo que imprime mide 144 bytes** (192
    caracteres b64url; 86 de cabecera + 41 de plaintext + 1 + 16). El vector se copió literal y el
    test compara los bytes, no ese encabezado. Que TASK-033 no use 145 como referencia.
  - El vector se bajó de `https://www.rfc-editor.org/rfc/rfc8291.txt` el 2026-10-11 y se copió de
    ese texto (solo se quitó el espacio en blanco de presentación, que el Apéndice A permite).
  - No probado contra un servicio de push real ni un navegador real: eso es TASK-033 (envío) y el
    gate de TASK-038. Lo verificado acá es conformidad con el RFC y la ida y vuelta local.
  - `Encrypt` no agrega padding: el servicio de push ve el largo exacto del payload (DD-53 ya lo
    asume: "ve el destino, el tamaño y la hora"). Si algún día importa, es rellenar con ceros después
    del `0x02` hasta un tamaño fijo.
  - TASK-033 tiene que decodificar `p256dh`/`auth` con b64url y conviene que llame a `Encrypt` (o
    valide igual) al suscribir, para rechazar claves inválidas en ese momento y no al primer aviso.
