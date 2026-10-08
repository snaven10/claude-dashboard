# TASK-003 — Server HTTP: API, SSE, poller y embed

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-001, TASK-002
- **Estado:** `done`

---

## Objetivo
`claude-dashboard -addr 127.0.0.1:7777` sirve el frontend embebido, `GET /api/sessions` y `GET /api/events` (SSE).

## Contexto verificado
- Contrato JSON en el design. DD-4 (un solo poller, 3 s, emitir solo si cambió). DD-7 (bind 127.0.0.1).

## Archivos
- **Crear:** `internal/server/server.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `web/embed.go` (`//go:embed`)
- **Modificar:** `cmd/claude-dashboard/main.go`

## Pasos
- [x] **Paso 1 — poller**: `agents.List` + `transcript.Summary` por sesión (en paralelo, límite 8) → snapshot + hash.
- [x] **Paso 2 — broadcaster SSE**: suscriptores con canal buffered; cliente lento se descarta, no bloquea.
- [x] **Paso 3 — rutas** `/`, `/api/sessions`, `/api/events` (evento `snapshot` al conectar y en cada cambio; heartbeat 15 s).
- [x] **Paso 4 — flags** `-addr`, `-interval`; rechazar `-addr` que no sea loopback salvo `-allow-remote`.
- [x] **Paso 5 — shutdown** limpio con `signal.NotifyContext`.

## Criterios de aceptación
- [x] `httptest`: `/api/sessions` responde el contrato con un `agents.List` falso.
- [x] Dos clientes SSE reciben el mismo snapshot; uno colgado no frena al otro.
- [x] Con 20 sesiones el poller completa un ciclo < 1.5 s (log de duración).

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** done
- **Resumen:** Server con un único poller (`Poll`/`Refresh`, hash sha256 de las sesiones sin `generatedAt`; emite solo si cambió; ante error de F1 conserva el último snapshot), broadcaster con canales buffered (4) que descarta al suscriptor lento sin bloquear, rutas `/`, `GET /api/sessions` (503 si F1 falla sin snapshot previo) y `GET /api/events` (snapshot al conectar, `event: snapshot`, heartbeat 15 s configurable). Resúmenes F3 en paralelo con límite 8; un fallo de F3 degrada solo esa sesión. `ValidateAddr` rechaza no-loopback (incluye `:7777`) salvo `-allow-remote`. `main.go` con flags `-addr`, `-interval`, `-allow-remote` y shutdown con `signal.NotifyContext`. Sin desvíos del design.
- **Archivos tocados:** `internal/server/server.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `web/embed.go`, `cmd/claude-dashboard/main.go`. Commit `9135ff8` en la rama `feature/plan-1-task-003` (worktree `plan-1-task-003`). Sin push.
- **Verificado por:** en el worktree: `gofmt -l .` → vacío; `go vet ./...` → limpio; `go test -race -count=1 ./...` → ok (agents, server, transcript). Criterios: httptest del contrato con `agents.List` falso (`TestSessionsContract`); dos clientes SSE con el mismo snapshot y un suscriptor colgado que no frena (`TestSSETwoClientsAndSlowOneDoesNotBlock`); ciclo con 20 sesiones = 304 ms con Summary simulado de 100 ms, pico de concurrencia 8 (`TestPollerCycleWith20Sessions`). NO verificado: arranque real del binario contra `claude` y tiempo real de F1/F3 con sesiones reales (no se levantó el server, por regla); la duración del ciclo se midió con fakes. QA del orquestador (2026-10-08) contra datos reales en `127.0.0.1:7788`: 25 sesiones con el contrato completo, ciclo del poller 403-475 ms, SSE emite `snapshot` al conectar, `-addr 0.0.0.0` rechazado; Playwright: 25 tarjetas en orden de urgencia, indicador `conectado`, título `(5) esperándote`, consola sin errores. Hallazgo menor: `project` = basename(cwd) muestra el nombre del worktree de orq (`plan-1-task-00X`) en vez del repo.
