# TASK-006 — Backend: stale, orq, tokens y timeline

- **Plan:** PLAN-002 — Alertas, workers de orq, tokens y línea de tiempo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-002-design.md`
- **Estado:** `done`

---

## Objetivo
`/api/sessions` y el SSE traen `stale`, `orq` y `tokens`; existe `GET /api/sessions/{sessionId}/timeline`.

## Contexto verificado
- Contrato y decisiones DD-8..DD-11 en el design. Leelo entero antes de empezar.
- Usage del asistente: `message.usage.{input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens}`, `message.model`, `message.id`. Las líneas del mismo `message.id` son consecutivas y repiten el `usage` (verificado: 596 líneas = 276 mensajes).
- `~/.orq/jobs.json`: objeto `jid → {key, ws, specialist, bgname, modo, host, …}`. `bgname` = `name` de F1.

## Archivos
- **Crear:** `internal/orq/orq.go`, `internal/orq/orq_test.go`, `internal/transcript/usage.go`, `internal/transcript/usage_test.go`
- **Modificar:** `internal/agents/agents.go` (campos nuevos en `Session`), `internal/transcript/transcript.go` (eventos para timeline), `internal/server/*.go`, `cmd/claude-dashboard/main.go` (flag `-stale`), `README.md`

## Pasos
- [x] **Paso 1 — `internal/orq`**: `Load(path) (map[bgname]Job, error)` con cache por mtime; regex de plan/task; archivo ausente o roto → mapa vacío.
- [x] **Paso 2 — tokens (DD-9)**: acumulador incremental por path con dedup por `message.id`, reset si el archivo se achica, líneas incompletas para el próximo ciclo.
- [x] **Paso 3 — stale (DD-8)**: flag `-stale` (default 10m); sin emitir SSE en cada ciclo por el paso del tiempo.
- [x] **Paso 4 — timeline**: últimos 20 eventos de la cola (prompt real / tool / text), ruta validada contra el snapshot (404 si el id no está).
- [x] **Paso 5 — README**: documentar los campos nuevos, el flag y la ruta.

## Criterios de aceptación
- [x] Test: un mensaje partido en 3 líneas con el mismo `message.id` suma su `usage` UNA vez.
- [x] Test: el acumulador lee solo lo nuevo en el 2º ciclo (contador de bytes) y se resetea si el archivo se trunca.
- [x] Test: `orq` une por `bgname`, saca plan/task del key, y un `jobs.json` roto no rompe `/api/sessions`.
- [x] Test: `stale` true para busy viejo, false para idle viejo y para busy reciente; el hash no cambia entre dos ciclos sin cambios reales.
- [x] Test: `/timeline` con id fuera del snapshot → 404; con id válido → ≤ 20 eventos ordenados, sin mensajes del harness.
- [x] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** done
- **Resumen:** `/api/sessions` y el SSE traen `stale` (DD-8, flag `-stale`, default 10m; 0 lo desactiva), `orq` (DD-10, unión por `bgname`, plan/task por regex del key) y `tokens` (DD-9, acumulador incremental por path con dedup por `message.id`, reset si el archivo se achica, línea incompleta para el próximo ciclo). Nueva ruta `GET /api/sessions/{sessionId}/timeline` (últimos 20 eventos de la cola, 404 si el id no está en el último snapshot). `stale` no mete ruido al hash porque depende de `lastActivityAt`, no de un contador: el hash solo cambia cuando el valor flipea.
- **Archivos tocados:** crear `internal/orq/orq.go`, `internal/orq/orq_test.go`, `internal/transcript/usage.go`, `internal/transcript/usage_test.go`; modificar `internal/agents/agents.go`, `internal/transcript/transcript.go`, `internal/server/server.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `cmd/claude-dashboard/main.go`, `README.md`. Commit código: `f943ae4`.
- **Verificado por:** en el worktree, `gofmt -l .` → vacío; `go vet ./...` → sin salida; `go test -race ./...` → `ok` en agents, orq, server y transcript (cmd y web sin tests). Tests: message.id partido en 3 líneas suma una vez; 2º ciclo lee 0 bytes sin cambios y solo lo nuevo con cambios (contador `usageBytesRead`), reset al truncar; orq une por bgname y jobs.json roto no rompe `/api/sessions`; stale true/false/false y hash estable entre ciclos sin cambios reales; `/timeline` 404 fuera del snapshot (incluye `..%2F`), ≤20 eventos ordenados sin harness. No verificado: ejecución contra el `claude` real y `~/.orq/jobs.json` real (no se levantó el server).
- **QA del orquestador (2026-10-08), rama de integración con TASK-007, contra 29-30 sesiones reales:**
  - **Bug: el poller nunca llamaba a `Usage`** (el acumulador estaba testeado pero nadie lo invocaba): 0 de 29 sesiones con tokens. Además, con `Summary` nil y `Usage` no, la goroutine hacía panic. Corregido en `1eb8457` + `TestSessionsCarryTokens` (prueba de mutación: sin la llamada falla con "con tokens 0, sin tokens 2").
  - **Bug: la primera lectura cargaba el transcript entero en RAM** (`make([]byte, size)`; el más grande pesa 112 MB). Ahora por bloques de 4 MB (`d5eb269`) + `TestUsageChunkBoundaries` con bloques de 7 bytes (mutación: sin arrastrar el resto entre bloques, falla).
  - Verificado: dedup por "último `message.id`" = dedup por id único en 60 transcripts reales; tokens de las 26 sesiones con transcript cuadran con un conteo independiente; RSS 24 MB tras leer 24 transcripts enteros; primer ciclo 1,06 s, siguientes 0,5-1,4 s con load 22 (la CLI `claude agents` sola tarda ~440 ms); los 5 workers de orq salen con plan/task/specialist; `/timeline` sirve 20 eventos.
  - NO verificado con datos reales: `stale` (ninguna sesión colgada en ese momento; cubierto por test).
- **Desvíos / gotchas:** `Load` de orq devuelve error solo la primera vez que lee una versión rota (queda cacheada vacía) para no loguear cada ciclo. `plan`/`task` son `*string` (null si el key no matchea). Se agregaron campos opcionales a `server.Server` (`Usage`, `Timeline`, `Orq`, `StaleAfter`, `Now`); los nil degradan sin romper tests previos. Commit sin el trailer `Claude-Session` por la regla dura de no atribución de AI.
