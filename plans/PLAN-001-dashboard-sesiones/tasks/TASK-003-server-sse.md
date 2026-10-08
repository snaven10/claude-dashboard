# TASK-003 — Server HTTP: API, SSE, poller y embed

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-001, TASK-002
- **Estado:** `pending`

---

## Objetivo
`claude-dashboard -addr 127.0.0.1:7777` sirve el frontend embebido, `GET /api/sessions` y `GET /api/events` (SSE).

## Contexto verificado
- Contrato JSON en el design. DD-4 (un solo poller, 3 s, emitir solo si cambió). DD-7 (bind 127.0.0.1).

## Archivos
- **Crear:** `internal/server/server.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `web/embed.go` (`//go:embed`)
- **Modificar:** `cmd/claude-dashboard/main.go`

## Pasos
- [ ] **Paso 1 — poller**: `agents.List` + `transcript.Summary` por sesión (en paralelo, límite 8) → snapshot + hash.
- [ ] **Paso 2 — broadcaster SSE**: suscriptores con canal buffered; cliente lento se descarta, no bloquea.
- [ ] **Paso 3 — rutas** `/`, `/api/sessions`, `/api/events` (evento `snapshot` al conectar y en cada cambio; heartbeat 15 s).
- [ ] **Paso 4 — flags** `-addr`, `-interval`; rechazar `-addr` que no sea loopback salvo `-allow-remote`.
- [ ] **Paso 5 — shutdown** limpio con `signal.NotifyContext`.

## Criterios de aceptación
- [ ] `httptest`: `/api/sessions` responde el contrato con un `agents.List` falso.
- [ ] Dos clientes SSE reciben el mismo snapshot; uno colgado no frena al otro.
- [ ] Con 20 sesiones el poller completa un ciclo < 1.5 s (log de duración).

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

