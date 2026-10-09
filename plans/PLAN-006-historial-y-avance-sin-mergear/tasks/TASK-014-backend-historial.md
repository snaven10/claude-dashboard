# TASK-014 — Backend: historial de tokens por día

- **Plan:** PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `pending`

---

## Objetivo
`GET /api/history` según el contrato §4, calculado de los transcripts (DD-26, DD-27).

## Contexto verificado
- Leé el master entero, sobre todo §2, §4, DD-26 y DD-27.
- `internal/transcript/usage.go` ya resuelve lectura incremental por bloques (`usageChunk`, `maxLine`), dedup por `message.id` consecutivo y truncado. Seguí ese patrón (podés extraer helpers compartidos si queda limpio, sin romper sus tests).
- Línea de uso real: `{"type":"assistant","timestamp":"2026-10-08T21:04:05.123Z","cwd":"/home/…","sessionId":"…","message":{"id":"msg_…","model":"claude-opus-5-5","usage":{"input_tokens":…,"output_tokens":…,"cache_read_input_tokens":…,"cache_creation_input_tokens":…}}}`.
- Layout real: `~/.claude/projects/<slug>/<sessionId>.jsonl` y `~/.claude/projects/<slug>/<sessionId>/subagents/<x>.jsonl` (el `sessionId` padre es el nombre del directorio).
- Proyecto: `agents.projectName(cwd)` (privado hoy, con cache). Exportalo (`agents.ProjectName`) y usalo; no copies la lógica.
- Día LOCAL: `time.Local` (inyectable en tests con una `*time.Location` fija, ej. UTC-6, para probar que `2026-10-09T03:00Z` cae el 8).
- Rutas en `internal/server/server.go` (`Handler`); la ruta nueva no toca el poller ni el SSE.
- **Fixtures sintéticos** en `t.TempDir()`.

## Archivos
- **Crear:** `internal/history/history.go`, `internal/history/history_test.go`
- **Modificar:** `internal/agents/agents.go` (exportar `ProjectName`), `internal/server/server.go`, `internal/server/server_test.go`, `cmd/claude-dashboard/main.go` (arranque del escáner, flag `-history-interval 60s`, `0` lo desactiva), `README.md`

## Pasos
- [ ] **Paso 1** — Escáner incremental por archivo con aporte por archivo (para poder restar al achicarse o desaparecer) y agregación por (día local, modelo, proyecto, sessionId).
- [ ] **Paso 2** — `Result(now)`: 30 días terminando hoy con ceros, top-5 + `"otros"`, `sessions` distintos.
- [ ] **Paso 3** — Goroutine (primer escaneo en segundo plano, después cada intervalo) y `GET /api/history` con `ready`. Log del tiempo del primer escaneo. README.

## Criterios de aceptación
- [ ] Test: dedup por `message.id` consecutivo; subagente suma al día y cuenta la sesión padre (no una sesión nueva).
- [ ] Test: día local con zona UTC-6 (`03:00Z` del 9 → día 8); 30 entradas con ceros y orden ascendente.
- [ ] Test: top-5 + `"otros"` con 7 modelos; la suma de las filas = total del día.
- [ ] Test: incremental (2º escaneo sin cambios lee 0 bytes); archivo truncado y archivo borrado restan su aporte.
- [ ] Test en el server: `ready:false` antes del primer escaneo y `true` después; la request no dispara un escaneo.
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
