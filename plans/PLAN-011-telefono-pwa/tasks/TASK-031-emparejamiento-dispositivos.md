# TASK-031 — Emparejamiento, cookie de dispositivo, gate remoto y revocación

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-030
- **Estado:** `done`

---

## Objetivo
Un teléfono solo ve datos del panel por el listener remoto si canjeó un token (o código) de un
solo uso por una cookie de dispositivo. Desde el panel local se generan emparejamientos, se listan
dispositivos y se revocan. El listener local sigue sin pedir nada.

## Contexto verificado
- Master + design: **DD-46, DD-47, DD-48, DD-49** (son el contrato de esta task), DD-45 (`devices.json`).
- Tras TASK-030: `isRemote(r)`, `Server.PublicURL`, helpers `readJSON`/`writeJSON` en `internal/remote`.
- SSE: `handleEvents` registra suscriptores con `subscribe()`; para DD-49 hace falta asociar
  `deviceId` al suscriptor remoto y poder cerrarlo.

## Archivos
- **Crear:** `internal/remote/pairing.go`, `internal/remote/pairing_test.go`, `internal/remote/devices.go`, `internal/remote/devices_test.go`, `internal/server/remote.go`, `internal/server/remote_test.go`
- **Modificar:** `internal/server/server.go` (registro de rutas y middleware, suscriptor SSE con `deviceId`), `cmd/claude-dashboard/main.go` (cablear el store)

## Pasos
- [x] **Paso 1 — `pairing.go`.** `Pairing` en memoria con reloj inyectable (`now func() time.Time`):
  `New() (token, code string, expires time.Time)` invalida el pendiente anterior; `Redeem(token, code)`
  valida (uno de los dos), un solo uso compartido, TTL 5 min, 5 fallos → invalida. Comparación en
  tiempo constante sobre SHA-256. Código Crockford base32 de 8 chars, se acepta con o sin guion y en
  minúsculas.
- [x] **Paso 2 — `devices.go`.** Store sobre `devices.json`: `Create(name, login) (cookie string, Device)`,
  `Lookup(cookie) (Device, bool)` (por SHA-256, tiempo constante), `Touch(id)` (≤ 1 escritura/min),
  `List()`, `Revoke(id)`. `name` saneado: ≤ 40 runas, sin control chars.
- [x] **Paso 3 — middleware en `server/remote.go`.** Según la tabla de DD-47: en remoto, rutas de
  datos y `/api/push/*` exigen cookie `cd_device` válida (401 JSON `{"error":"unpaired"}`);
  `/api/pair/new` y `/api/devices*` → 404 en remoto; `/api/pair/redeem` → 404 en local. Anti-CSRF
  de DD-48 en todo POST/DELETE nuevo.
- [x] **Paso 4 — rutas.** `GET /api/remote` → `{enabled, local, paired}`. `POST /api/pair/new` →
  `{url: PublicURL+"/#pair="+token, code, expiresAt}`. `POST /api/pair/redeem` `{token|code, name}`
  → `Set-Cookie` de DD-46 + `{deviceId}`; guarda `Tailscale-User-Login` si viene. `GET /api/devices`
  → lista sin `cookieHash`. `DELETE /api/devices/{id}` → revoca y cierra sus SSE (DD-49); 404 si no existe.
  Sin `PublicURL`, ninguna de estas rutas se registra (DD-44).
- [x] **Paso 5 — tests** (todos con reloj falso y `t.TempDir()` como state dir).

## Criterios de aceptación
- [x] Test: token válido → cookie con `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, `Max-Age` de DD-46.
- [x] Test: **token expirado** (reloj + 5 min 1 s) → 401/410 y no crea dispositivo.
- [x] Test: **token reusado** → el segundo canje falla; canjear el código después del token también falla.
- [x] Test: generar un emparejamiento nuevo invalida el anterior.
- [x] Test: 5 canjes fallidos invalidan el pendiente aunque el sexto sea correcto.
- [x] Test: **cookie inválida** (aleatoria, vacía, de dispositivo revocado) en el remoto → 401 en
  `/api/sessions`, `/api/events`, timeline, history.
- [x] Test: **petición loopback sin cookie** al listener local → 200 en `/api/sessions` (como hoy).
- [x] Test: estáticos y `GET /api/remote` responden en remoto sin cookie.
- [x] Test: `/api/pair/new`, `/api/devices` → 404 por el remoto; `/api/pair/redeem` → 404 por el local.
- [x] Test: POST sin `Content-Type: application/json`, o con `Origin` ajeno, o `Sec-Fetch-Site: cross-site` → 403.
- [x] Test: revocar cierra un SSE abierto de ese dispositivo (el body del stream termina) y no toca otros.
- [x] Test: `devices.json` no contiene la cookie en claro (grep del contenido) y queda en `0600`.
- [x] Test: sin `PublicURL`, `/api/pair/new` → 404 por el local.
- [x] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Riesgos
- Cerrar el SSE de un dispositivo revocado sin carreras con `broadcast` (que también cierra canales):
  un solo dueño del `close`, bajo `s.mu`.

## Resultado
- **Estado final:** `done`
- **Resumen:** `Pairing` en memoria (token 32 B + código Crockford de 8, TTL 5 min, un solo uso compartido, 5 fallos invalidan, comparación en tiempo constante sobre SHA-256), store `devices.json` (cookie solo como hash, `Touch` ≤ 1 escritura/min) y un `gate` que envuelve el mux de ambos listeners según DD-47. Rutas `/api/remote`, `/api/pair/new|redeem`, `/api/devices`, `DELETE /api/devices/{id}` solo con `-public-url`. Anti-CSRF de DD-48 en todo POST/DELETE bajo `/api/`. Revocar corta los SSE del dispositivo con un único camino de cierre (`dropLocked`, bajo `s.mu`).
- **Archivos tocados:** commit `1ceadf4`. Creados: `internal/remote/{pairing,pairing_test,devices,devices_test}.go`, `internal/server/{remote,remote_test}.go`. Modificados: `internal/server/server.go` (rutas, `gate`, `subDev`/`dropLocked`/`subscribeDevice`), `cmd/claude-dashboard/main.go` (cableado), `internal/server/server_test.go` (un test de TASK-030, ver Desviaciones).
- **Verificado por:** `gofmt -l .` → vacío; `go vet ./...` → sin salida; `go test -race ./...` → todos `ok` (agents, history, orq, plans, remote, server, transcript, webpush). Todos los criterios de aceptación tienen test que corrió, en `internal/server/remote_test.go` e `internal/remote/*_test.go`.
- **Desviaciones:**
  - `Pairing.New` devuelve un 4º valor `error` (falla de `crypto/rand`); `Devices.Create` y `Revoke` también devuelven `error` (escritura a disco).
  - `GET /api/devices` responde `{"devices":[...]}` (objeto, no array pelado); `expiresAt` en epoch ms, como el resto de las marcas del repo; `DELETE` responde 204.
  - **Falla cerrado:** si el listener es remoto y el modo remoto no está completo (`PublicURL`+`Pairing`+`Devices`), toda ruta `/api/` responde 404. Por eso se ajustó `TestIsRemoteByListenerAndSameSnapshot` (TASK-030): su parte de "mismo snapshot por los dos listeners" ahora habilita el modo remoto y manda cookie, porque el remoto ya no sirve `/api/sessions` sin ella (es el comportamiento pedido por DD-47).
  - Cualquier `/api/*` desconocido en el remoto exige cookie (denegar por defecto), más estricto que la lista de la tabla.
  - Carrera revocación/SSE: `handleEvents` revalida `Devices.Has` tras registrarse, para no dejar abierto un stream que se registró entre el gate y `closeDevice`.
- **Riesgos abiertos / siguiente:**
  - DD-49 también pide borrar las suscripciones push del dispositivo: `subscriptions.json` no existe aún (TASK-033). TASK-034 debe llamar a su store desde `handleDeviceRevoke` (después de `Revoke`, antes de `closeDevice`).
  - Las rutas `/api/push/*` no están registradas acá; el gate ya les exige cookie en el remoto (todo `/api/` salvo `/api/remote` y `/api/pair/redeem`).
  - Una conexión SSE abierta no se corta al vencer la cookie (90 d), solo al revocar.
