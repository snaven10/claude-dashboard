# TASK-018 — Backend: datos nuevos por sesión

- **Plan:** PLAN-008 — Datos nuevos por sesión
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `done`

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
- **Estado:** done
- **Commit:** d7b042d (rama `feature/plan-8-task-018`)
- **Archivos creados:** `internal/transcript/extras.go`, `internal/transcript/subagents.go`, `internal/transcript/subagents_test.go`
- **Archivos modificados:** `internal/transcript/transcript.go`, `transcript_test.go`, `usage.go`, `usage_test.go`, `internal/agents/agents.go`, `internal/server/poller.go`, `server.go`, `server_test.go`, `README.md`, `cmd/claude-dashboard/main.go` (ver desvío)
- **Qué hace:** `Detail.Away`/`TurnStartedAt` en `parse` (cola); `prs`/`artifacts`/`lines`/`permissionMode` en el acumulador incremental con filtro `bytes.Contains` previo a cualquier `Unmarshal` (`extraKind`); `ExtrasFor` (misma pasada que Usage/Errors) + `Subagents(dir, now)`; el poller los vuelca en `Session` (listas `[]` sin dato, escalares `null`). `extraLine` no declara `totalCostUSD`, así que el costo no puede salir.
- **Desvío:** se tocó `cmd/claude-dashboard/main.go` (1 línea: `Extras: transcript.ExtrasFor`), que no estaba en la lista de la task; sin eso el daemon real no pide los campos.
- **Decisiones menores:** `permissionMode` raro como último valor da `null` (no arrastra el anterior); `away` sin prompt real en la ventana de lectura no se muestra.
- **Criterios:** todos cubiertos por tests corridos (away, prs, artifacts, lines sin costo, permissionMode, subagents 119 s/121 s/sin dir, incremental 0 bytes, server con los campos y sin emitir si `active` no cambia).
- **Verificado por:** `gofmt -l .` → vacío; `go vet ./...` → sin salida; `go test -race ./...` → ok en agents, history, orq, plans, server, transcript.
- **No verificado:** contra transcripts reales (solo fixtures sintéticos); rendimiento sobre 1,6 GB al arrancar.
