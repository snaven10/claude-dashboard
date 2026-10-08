# TASK-006 — Backend: stale, orq, tokens y timeline

- **Plan:** PLAN-002 — Alertas, workers de orq, tokens y línea de tiempo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-002-design.md`
- **Estado:** `pending`

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
- [ ] **Paso 1 — `internal/orq`**: `Load(path) (map[bgname]Job, error)` con cache por mtime; regex de plan/task; archivo ausente o roto → mapa vacío.
- [ ] **Paso 2 — tokens (DD-9)**: acumulador incremental por path con dedup por `message.id`, reset si el archivo se achica, líneas incompletas para el próximo ciclo.
- [ ] **Paso 3 — stale (DD-8)**: flag `-stale` (default 10m); sin emitir SSE en cada ciclo por el paso del tiempo.
- [ ] **Paso 4 — timeline**: últimos 20 eventos de la cola (prompt real / tool / text), ruta validada contra el snapshot (404 si el id no está).
- [ ] **Paso 5 — README**: documentar los campos nuevos, el flag y la ruta.

## Criterios de aceptación
- [ ] Test: un mensaje partido en 3 líneas con el mismo `message.id` suma su `usage` UNA vez.
- [ ] Test: el acumulador lee solo lo nuevo en el 2º ciclo (contador de bytes) y se resetea si el archivo se trunca.
- [ ] Test: `orq` une por `bgname`, saca plan/task del key, y un `jobs.json` roto no rompe `/api/sessions`.
- [ ] Test: `stale` true para busy viejo, false para idle viejo y para busy reciente; el hash no cambia entre dos ciclos sin cambios reales.
- [ ] Test: `/timeline` con id fuera del snapshot → 404; con id válido → ≤ 20 eventos ordenados, sin mensajes del harness.
- [ ] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
