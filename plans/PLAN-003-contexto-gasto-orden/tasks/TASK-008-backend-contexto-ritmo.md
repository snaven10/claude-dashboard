# TASK-008 — Backend: contexto y ritmo por sesión

- **Plan:** PLAN-003 — Contexto, gasto por plan, orden y ritmo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-003-design.md`
- **Estado:** `done`

---

## Objetivo
Cada sesión trae `context` y `rate` (contrato del design), calculados en la MISMA pasada incremental del acumulador de tokens.

## Contexto verificado
- Leé `PLAN-003-design.md` (DD-13, DD-14) y `internal/transcript/usage.go` (acumulador incremental por bloques de 4 MB, dedup por `message.id`). NO lo reemplaces: extendelo.
- Línea de compactación real: `{"type":"system","subtype":"compact_boundary","timestamp":"…","compactMetadata":{"trigger":"auto","preTokens":967001,"postTokens":12450,…}}`.
- Contexto de un mensaje = `input_tokens + cache_read_input_tokens + cache_creation_input_tokens` de su `usage`.
- El poller llama a `Usage` en `internal/server/poller.go`; hay un test (`TestSessionsCarryTokens`) que falla si se deja de llamar: hacé lo mismo para `context`/`rate`.

## Archivos
- **Modificar:** `internal/transcript/usage.go`, `internal/transcript/usage_test.go`, `internal/agents/agents.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `cmd/claude-dashboard/main.go` (flags `-context-warn`, `-rate-output`, `-rate-calls`), `README.md`

## Pasos
- [x] **Paso 1** — En el acumulador: último contexto, conteo y último `preTokens`/timestamp de `compact_boundary`, y cola `(ts, output)` por `message.id` único de los últimos 10 min.
- [x] **Paso 2** — `rate` contra `now` inyectable (ventana 5 min); `high` por umbrales. `context.warn` por umbral.
- [x] **Paso 3** — Cablear en el poller y en `Session`; flags en `main.go`; README.
- [x] **Paso 4** — `stale` y `rate` cambian con el tiempo: que el hash del snapshot no emita en cada ciclo si solo envejeció la ventana sin cambiar `high` ni los números (mirá cómo se resolvió `stale` en PLAN-002).

## Criterios de aceptación
- [x] Test: dos `compact_boundary` → `compactions=2`, `lastPreTokens` del último; sin compactaciones → `0` y `null`.
- [x] Test: `context.current` es el del ÚLTIMO mensaje (no la suma) y respeta la dedup por `message.id`.
- [x] Test: `rate` cuenta solo lo de los últimos 5 min según `now` inyectado; `high` true por output y por llamadas, false debajo.
- [x] Test en el server: el snapshot trae `context` y `rate` (y falla si el poller deja de pedirlos).
- [x] La lectura sigue siendo incremental: el 2º ciclo sin cambios lee 0 bytes.
- [x] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** done
- **Resumen:** `transcript.Activity(cwd, id, now)` devuelve `Context` y `Rate` desde el mismo acumulador incremental que `Usage` (comparten `scan`; ciclo sin cambios lee 0 bytes). Contexto = último mensaje (con dedup por `message.id`), compactaciones y último `preTokens`/timestamp de `compact_boundary`, cola de 10 min podada contra la entrada más nueva y ventana de 5 min contra `now`. El poller (`Server.Activity`) aplica `warn`/`high` con `-context-warn` 800000, `-rate-output` 20000, `-rate-calls` 25 (0 desactiva). El hash no emite si solo envejece la ventana sin cambiar números ni flags. Desvío: se tocó también `internal/server/server.go` (no listado en la task) para agregar los campos `Activity`/umbrales al `Server`.
- **Archivos tocados:** `internal/transcript/usage.go`, `internal/transcript/usage_test.go`, `internal/agents/agents.go`, `internal/server/poller.go`, `internal/server/server.go`, `internal/server/server_test.go`, `cmd/claude-dashboard/main.go`, `README.md`. Commit código: `c0c998a`.
- **Verificado por:** en el worktree: `gofmt -l .` → vacío; `go vet ./...` → ok; `go test -race ./...` → ok en agents, orq, server, transcript. Tests nuevos: `TestActivityCompactions`, `TestActivityContextIsLastMessageAndDedup`, `TestActivityRateWindow`, `TestActivityIncremental`, `TestSessionsCarryContextAndRate`. No verificado: ejecución contra el daemon y transcripts reales.
