# TASK-008 — Backend: contexto y ritmo por sesión

- **Plan:** PLAN-003 — Contexto, gasto por plan, orden y ritmo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-003-design.md`
- **Estado:** `pending`

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
- [ ] **Paso 1** — En el acumulador: último contexto, conteo y último `preTokens`/timestamp de `compact_boundary`, y cola `(ts, output)` por `message.id` único de los últimos 10 min.
- [ ] **Paso 2** — `rate` contra `now` inyectable (ventana 5 min); `high` por umbrales. `context.warn` por umbral.
- [ ] **Paso 3** — Cablear en el poller y en `Session`; flags en `main.go`; README.
- [ ] **Paso 4** — `stale` y `rate` cambian con el tiempo: que el hash del snapshot no emita en cada ciclo si solo envejeció la ventana sin cambiar `high` ni los números (mirá cómo se resolvió `stale` en PLAN-002).

## Criterios de aceptación
- [ ] Test: dos `compact_boundary` → `compactions=2`, `lastPreTokens` del último; sin compactaciones → `0` y `null`.
- [ ] Test: `context.current` es el del ÚLTIMO mensaje (no la suma) y respeta la dedup por `message.id`.
- [ ] Test: `rate` cuenta solo lo de los últimos 5 min según `now` inyectado; `high` true por output y por llamadas, false debajo.
- [ ] Test en el server: el snapshot trae `context` y `rate` (y falla si el poller deja de pedirlos).
- [ ] La lectura sigue siendo incremental: el 2º ciclo sin cambios lee 0 bytes.
- [ ] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
