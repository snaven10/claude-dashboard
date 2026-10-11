# TASK-031 — Emparejamiento, cookie de dispositivo, gate remoto y revocación

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-030
- **Estado:** `pending`

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
- [ ] **Paso 1 — `pairing.go`.** `Pairing` en memoria con reloj inyectable (`now func() time.Time`):
  `New() (token, code string, expires time.Time)` invalida el pendiente anterior; `Redeem(token, code)`
  valida (uno de los dos), un solo uso compartido, TTL 5 min, 5 fallos → invalida. Comparación en
  tiempo constante sobre SHA-256. Código Crockford base32 de 8 chars, se acepta con o sin guion y en
  minúsculas.
- [ ] **Paso 2 — `devices.go`.** Store sobre `devices.json`: `Create(name, login) (cookie string, Device)`,
  `Lookup(cookie) (Device, bool)` (por SHA-256, tiempo constante), `Touch(id)` (≤ 1 escritura/min),
  `List()`, `Revoke(id)`. `name` saneado: ≤ 40 runas, sin control chars.
- [ ] **Paso 3 — middleware en `server/remote.go`.** Según la tabla de DD-47: en remoto, rutas de
  datos y `/api/push/*` exigen cookie `cd_device` válida (401 JSON `{"error":"unpaired"}`);
  `/api/pair/new` y `/api/devices*` → 404 en remoto; `/api/pair/redeem` → 404 en local. Anti-CSRF
  de DD-48 en todo POST/DELETE nuevo.
- [ ] **Paso 4 — rutas.** `GET /api/remote` → `{enabled, local, paired}`. `POST /api/pair/new` →
  `{url: PublicURL+"/#pair="+token, code, expiresAt}`. `POST /api/pair/redeem` `{token|code, name}`
  → `Set-Cookie` de DD-46 + `{deviceId}`; guarda `Tailscale-User-Login` si viene. `GET /api/devices`
  → lista sin `cookieHash`. `DELETE /api/devices/{id}` → revoca y cierra sus SSE (DD-49); 404 si no existe.
  Sin `PublicURL`, ninguna de estas rutas se registra (DD-44).
- [ ] **Paso 5 — tests** (todos con reloj falso y `t.TempDir()` como state dir).

## Criterios de aceptación
- [ ] Test: token válido → cookie con `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/`, `Max-Age` de DD-46.
- [ ] Test: **token expirado** (reloj + 5 min 1 s) → 401/410 y no crea dispositivo.
- [ ] Test: **token reusado** → el segundo canje falla; canjear el código después del token también falla.
- [ ] Test: generar un emparejamiento nuevo invalida el anterior.
- [ ] Test: 5 canjes fallidos invalidan el pendiente aunque el sexto sea correcto.
- [ ] Test: **cookie inválida** (aleatoria, vacía, de dispositivo revocado) en el remoto → 401 en
  `/api/sessions`, `/api/events`, timeline, history.
- [ ] Test: **petición loopback sin cookie** al listener local → 200 en `/api/sessions` (como hoy).
- [ ] Test: estáticos y `GET /api/remote` responden en remoto sin cookie.
- [ ] Test: `/api/pair/new`, `/api/devices` → 404 por el remoto; `/api/pair/redeem` → 404 por el local.
- [ ] Test: POST sin `Content-Type: application/json`, o con `Origin` ajeno, o `Sec-Fetch-Site: cross-site` → 403.
- [ ] Test: revocar cierra un SSE abierto de ese dispositivo (el body del stream termina) y no toca otros.
- [ ] Test: `devices.json` no contiene la cookie en claro (grep del contenido) y queda en `0600`.
- [ ] Test: sin `PublicURL`, `/api/pair/new` → 404 por el local.
- [ ] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Riesgos
- Cerrar el SSE de un dispositivo revocado sin carreras con `broadcast` (que también cierra canales):
  un solo dueño del `close`, bajo `s.mu`.

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
