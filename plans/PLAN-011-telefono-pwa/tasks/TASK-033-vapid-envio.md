# TASK-033 — Claves VAPID, JWT ES256, envío al servicio de push y store de suscripciones

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-030 (store), TASK-032 (`webpush.Encrypt`)
- **Estado:** `pending`

---

## Objetivo
`internal/webpush` sabe generar/cargar las claves VAPID, firmar el JWT ES256 y mandar un push
cifrado a una suscripción manejando las respuestas del servicio; las suscripciones se persisten
en `subscriptions.json` ligadas a un dispositivo.

## Contexto verificado
- Design **DD-51** (claves, JWT, cabeceras, respuestas, validación del endpoint/SSRF), DD-45 (archivos).
- Tras TASK-030: `readJSON`/`writeJSON` en `internal/remote`. Tras TASK-032: `webpush.Encrypt`.
- Go 1.27: `ecdsa.GenerateKey`, `ecdsa.Sign` (r, s *big.Int) — se serializan a 32+32 bytes con
  padding a la izquierda —, `PublicKey.Bytes()` (sin comprimir, 65 bytes), `ecdsa.ParseRawPrivateKey`
  y `ParseUncompressedPublicKey` (1.25) para cargar desde `vapid.json`.

## Archivos
- **Crear:** `internal/webpush/vapid.go`, `internal/webpush/vapid_test.go`, `internal/webpush/send.go`, `internal/webpush/send_test.go`, `internal/remote/subscriptions.go`, `internal/remote/subscriptions_test.go`

## Pasos
- [ ] **Paso 1 — `vapid.go`.** `LoadOrCreateKeys(path)`: si no existe, genera y escribe `vapid.json`
  (0600); si existe, la carga y valida que la pública corresponda a la privada. `PublicKeyB64()`.
  `JWT(aud string, now time.Time)` con los claims de DD-51 (`sub` = PublicURL inyectada), cache por
  `aud` hasta 1 h antes de `exp`.
- [ ] **Paso 2 — `subscriptions.go`.** Store sobre `subscriptions.json`: `Add(deviceId, endpoint,
  p256dh, auth)` (upsert por endpoint), `Remove(endpoint)`, `RemoveDevice(deviceId)`, `All()`.
  `ValidateEndpoint` de DD-51: `https`, host no vacío y no IP literal ni `localhost`, ≤ 1024.
  `p256dh` 65 bytes y `auth` 16 bytes decodificados de b64url.
- [ ] **Paso 3 — `send.go`.** `Sender{Client *http.Client, Keys, Now}` con `Send(ctx, sub, payload, topic)`
  → resultado tipado (`OK`, `Gone`, `RetryAfter(t)`, `Err`). Cabeceras exactas de DD-51. Cola
  acotada (64) con un worker; `Enqueue` nunca bloquea (cola llena → log y descarte). `Gone`
  (404/410) → callback que borra la suscripción. `Retry-After` por host.
- [ ] **Paso 4 — tests** con `httptest.NewTLSServer` como servicio de push falso (el cliente del
  test confía en su cert) y endpoints `https://push.ejemplo.test/...` donde haga falta un host.

## Criterios de aceptación
- [ ] Test JWT: header y claims decodificados == los esperados (`aud` = origen del endpoint,
  `exp` = now+12h, `sub` = PublicURL); la firma tiene **64 bytes** y verifica con `ecdsa.Verify`
  usando la pública (r = primeros 32, s = últimos 32); una firma DER no pasa el parser del test.
- [ ] Test: `LoadOrCreateKeys` crea `vapid.json` en `0600`, la segunda llamada devuelve la misma
  clave; un `vapid.json` con pública que no corresponde → error.
- [ ] Test envío: el servidor falso recibe `Authorization: vapid t=…, k=<pública>`,
  `Content-Encoding: aes128gcm`, `TTL`, `Urgency: high`, `Topic` de ≤ 32 chars b64url, y un cuerpo
  que el test descifra (con la clave privada del UA generada en el test) al payload original.
- [ ] Test: 410 y 404 → la suscripción desaparece del store; 429 con `Retry-After: 120` → no se
  envía a ese host antes de 120 s (reloj falso); 500 → log, la suscripción se conserva.
- [ ] Test: `ValidateEndpoint` rechaza `http://…`, `https://127.0.0.1/…`, `https://[::1]/…`,
  `https://localhost/…`, `https://192.0.2.5/…`, host vacío y > 1024 chars.
- [ ] Test: `Enqueue` con la cola llena no bloquea (test con timeout) y `-race` limpio.
- [ ] `subscriptions.json` en `0600`. `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
