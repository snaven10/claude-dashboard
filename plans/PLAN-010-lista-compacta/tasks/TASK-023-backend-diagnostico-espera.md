# TASK-023 — Backend: log de diagnóstico cuando una sesión espera sin `ask`

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-023` desde `main`
- **Depende de:** — (en paralelo con TASK-022 y con cualquier task del frontend)
- **Estado:** `pending`

---

## Objetivo
Cuando una sesión `waiting`/`blocked` queda sin `ask`, el log del servicio dice por qué
(`sin-transcript` o `sin-texto-en-turno`), una sola vez por cambio, sin datos del usuario. Sirve
para confirmar en vivo cuál de las dos ramas del master §2.1 produce el "input needed".

## Contexto verificado
- `internal/server/poller.go:56-58`: `s.Summary(ss.Cwd, ss.SessionID)`; error → `s.logf` y sin detalle.
- `internal/server/poller.go:224-245` (`applyDetail`): `ss.Ask` solo se llena si `d.Ask != nil`.
- `internal/transcript/transcript.go:131-160` (`Summary`): transcript no encontrado (`findPath`
  devuelve `""`) → `Detail{}` y `nil`. Hoy no hay forma de distinguir "no hay transcript" de "hay
  transcript pero no dejó nada abierto".
- `internal/transcript/transcript.go:240-260` (`askScan`): sin texto ni `tool_use` abierto desde el
  último prompt real → `result()` nil. Un permiso pendiente no se escribe en el transcript.
- Constantes en `internal/agents/agents.go:25-29`: `agents.StatusWaiting`, `agents.StatusBlocked`
  (y `StatusBusy`, `StatusIdle`, `StatusDone`). Log con `s.logf` (`internal/server/server.go:64`).
- El poller corre cada 3 s: el log **no** puede repetirse en cada ciclo.

## Archivos
- **Modificar:** `internal/server/poller.go`, `internal/server/server_test.go`
- **Modificar (solo si hace falta para distinguir "sin transcript"):** `internal/transcript/transcript.go`
  + `internal/transcript/transcript_test.go` — p. ej. un campo `Found bool` (`json:"-"`) en `Detail`
  que `Summary` pone en `true` cuando encontró el archivo. **No** cambia el JSON de `/api/sessions`.

## Pasos
- [ ] **Paso 1 — Distinguir el motivo.** Tras `applyDetail`, si `ss.Status` es waiting/blocked y
  `ss.Ask == nil`: motivo `sin-transcript` (Summary no encontró archivo) o `sin-texto-en-turno`.
- [ ] **Paso 2 — Log deduplicado.** Mapa `sessionId → motivo` protegido por mutex en el `Server`;
  loguear solo cuando el motivo de esa sesión cambia (y olvidar la sesión cuando deja de esperar o
  desaparece). Formato: `waiting sin ask: <primeros 8 del sessionId> waitingFor=<valor F1> motivo=<m>`.
  Nada de títulos, prompts ni rutas.
- [ ] **Paso 3 — Tests** con `httptest`/`Refresh` y `Summary` falso (como `server_test.go:631`):
  (a) waiting + detalle sin ask + archivo encontrado → un log `sin-texto-en-turno`; (b) waiting sin
  transcript → `sin-transcript`; (c) dos `Refresh` seguidos iguales → un solo log; (d) idle sin ask →
  nada; (e) el JSON del snapshot no cambia (mismas claves que `server_test.go:127`).

## Criterios de aceptación
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde (pegados en `Verificado por`).
- [ ] El contrato de `GET /api/sessions` no cambia (test de claves en verde, `docs/API.md` intacto).
- [ ] El log no contiene texto de la sesión: revisado en el test (la línea solo tiene id corto,
  `waitingFor` y motivo).

## Riesgos
- Si `Found` se agrega a `Detail`, el cache de `Summary` guarda `Detail` por `(path,size,mtime)`:
  el caso "sin archivo" no pasa por el cache; verificar que no se cachee un `Found` viejo.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
