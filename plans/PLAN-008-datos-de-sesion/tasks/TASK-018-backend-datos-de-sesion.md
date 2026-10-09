# TASK-018 — Backend: datos nuevos por sesión

- **Plan:** PLAN-008 — Datos nuevos por sesión
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `pending`

---

## Objetivo
Cada sesión trae `away`, `prs`, `artifacts`, `lines`, `permissionMode`, `subagents` y `turnStartedAt` (contrato §4, DD-35).

## Contexto verificado
- Leé el master entero, sobre todo §2, §4 y DD-35.
- Formas reales (solo claves):
  - `{"type":"system","subtype":"away_summary","content":"…","timestamp":"…Z",…}`
  - `{"type":"pr-link","prNumber":87,"prRepository":"grupo/repo","prUrl":"https://…","sessionId":"…","timestamp":"…"}`
  - `{"type":"frame-link","frameUrl":"https://claude.ai/…" | "","title":"…","artifactCount":…,"path":"…","timestamp":"…"}`
  - `{"type":"cost-state","totalLinesAdded":412,"totalLinesRemoved":96,"totalCostUSD":…,…}` (NO exponer el costo)
  - `{"type":"permission-mode","permissionMode":"auto","sessionId":"…"}`
  - subagentes: `~/.claude/projects/<slug>/<sessionId>/subagents/<x>.jsonl`
- `internal/transcript/transcript.go` (`parse`, `realUserText`, `harnessText`, `askScan`) y `internal/transcript/usage.go` (`consume`, `result`, acumulador incremental; el filtro barato por `bytes.Contains` antes del `json.Unmarshal`). Extendé, no reemplaces. El filtro barato es importante: `consume` corre sobre 1,6 GB al arrancar.
- `internal/server/poller.go`: hay un test por campo que falla si el poller deja de pedirlo (`TestSessionsCarry…`). Hacé lo mismo.
- **Fixtures sintéticos.** Nunca texto real de transcripts.

## Archivos
- **Modificar:** `internal/transcript/transcript.go`, `internal/transcript/transcript_test.go`, `internal/transcript/usage.go`, `internal/transcript/usage_test.go`, `internal/agents/agents.go` (campos + tipos), `internal/server/poller.go`, `internal/server/server_test.go`, `README.md`
- **Crear (si conviene):** `internal/transcript/subagents.go` + test

## Pasos
- [ ] **Paso 1** — `away` y `turnStartedAt` en `parse` (cola).
- [ ] **Paso 2** — `prs`, `artifacts`, `lines`, `permissionMode` en el acumulador, con validaciones de DD-35.
- [ ] **Paso 3** — `subagents` con `now` inyectable.
- [ ] **Paso 4** — Cableado en el poller y en `Session`; hash: `subagents.active` cambia con el tiempo, que no emita en cada ciclo si el número no cambió. README.

## Criterios de aceptación
- [ ] Test `away`: posterior al último prompt → sale; anterior → `null`; truncado a 400 runas.
- [ ] Test `prs`: dedup por URL, máximo 5 (los últimos), descarta `http://`, `javascript:`, número 0.
- [ ] Test `artifacts`: descarta `frameUrl` vacío y hosts que no sean `https://claude.ai/`.
- [ ] Test: `lines` del ÚLTIMO `cost-state`; el JSON del snapshot NO contiene `totalCostUSD` ni `cost`.
- [ ] Test `permissionMode`: último valor; valor raro (`<x>`, 40 letras) → `null`.
- [ ] Test `subagents`: activo dentro de 120 s, inactivo afuera, sin directorio → `null`.
- [ ] Test: lectura incremental intacta (2º ciclo sin cambios lee 0 bytes) y server trae los 7 campos (falla si el poller deja de pedirlos).
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
