# TASK-012 — Backend: rama de git (`branch`) y avance del plan (`plan`)

- **Plan:** PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `pending`

---

## Objetivo
Cada sesión trae `branch` (DD-24) y `plan` (DD-23) según el contrato §4 del master.

## Contexto verificado
- Leé el master entero, sobre todo §2, §4, DD-23 y DD-24.
- `internal/agents/agents.go` → `projectName`/`resolveProject` ya leen `.git` y siguen el `gitdir:` de un worktree hasta el repo principal. Reusá esa lógica (refactor chico) para: (a) leer `HEAD` del gitdir de la sesión → `branch`; (b) obtener la raíz del repo principal → punto de partida para buscar `plans/`. **Sin ejecutar `git`.** OJO: en un worktree el `HEAD` está en el gitdir del worktree (`<repo>/.git/worktrees/<x>/HEAD`), no en el del repo principal.
- `projectName` cachea por cwd para siempre; `HEAD` y los `Estado:` cambian → esos NO se cachean para siempre (cache por `(mtime,size)` o releer, son archivos chicos).
- Paquete nuevo `internal/plans`: dado `(número, raízRepo, home)` devuelve el avance. Formatos reales de `**Estado:**` que el parser debe tolerar (todos vistos hoy): `` `pending` ``, `done` (sin backticks), `Pendiente`, `pendiente`, `⬜ PENDIENTE · **Depende de:** …`, `🟢 \`pending\` — **DESBLOQUEADA…**`, `` `done` — cerrada 2026-08-30… ``, `sigue \`in_progress\`.`, `` `skipped` — DIFERIDO… ``.
- Número de plan: `orq.Job.PlanTask()` (`internal/orq/orq.go`) si el key lo trae; si no, de la rama con `(?i)plan-?(\d+)`. Ej. reales de rama: `feat-plan-042-latido-front`, `plan-4-task-010`.
- Layouts reales: `~/personal/claude-dashboard/plans/PLAN-004-abrir-en-telefono/tasks/TASK-010-….md` (plan en la raíz del repo) y `~/mi-empresa/plans/PLAN-042-…/tasks/` donde el repo es `~/mi-empresa/web-frontend` (plan en un PADRE del repo). Nunca subir más allá de `$HOME` (inyectable en tests).
- El cableado va en `internal/server/poller.go` (`enrich`); hay un test por campo que falla si el poller deja de pedirlo (`TestSessionsCarryTokens`): hacé lo mismo.
- **Fixtures sintéticos** en `t.TempDir()`: armá repos falsos con `.git/HEAD`, worktree con `.git` archivo `gitdir: …`, y `plans/` falsos.

## Archivos
- **Crear:** `internal/plans/plans.go`, `internal/plans/plans_test.go`
- **Modificar:** `internal/agents/agents.go` (campos `Branch`, `Plan` + tipo `Plan`; lectura de HEAD), `internal/agents/agents_test.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `README.md`

## Pasos
- [ ] **Paso 1** — Rama: gitdir de la sesión → `HEAD` → nombre de rama, sha corto si está desacoplado, `null` sin `.git`; validación de DD-24.
- [ ] **Paso 2** — `internal/plans`: buscar `plans/PLAN-<N>-*` subiendo desde la raíz del repo principal hasta `$HOME`; N como entero; parsear `Estado:` (DD-23) de cada `tasks/*.md`; contar por estado; cache por archivo.
- [ ] **Paso 3** — Número de plan (orq key → rama) y cableado en `enrich`; tests del server. README.

## Criterios de aceptación
- [ ] Test rama: repo normal, worktree (`.git` archivo, HEAD del worktree ≠ HEAD del principal), HEAD desacoplado, sin `.git`, HEAD con basura → `null`.
- [ ] Test parser: TODAS las variantes de `Estado:` listadas en Contexto dan el estado correcto; una línea sin vocabulario → `unknown`; solo cuenta la PRIMERA línea `**Estado:**`.
- [ ] Test búsqueda: plan en la raíz del repo; plan en un padre; `PLAN-003` matchea N=3; N=13 NO matchea `PLAN-042`; no sale de `$HOME`; sin plan → `null`.
- [ ] Test: un `Estado:` que cambia en disco se refleja en la siguiente llamada.
- [ ] Test en el server: el snapshot trae `branch` y `plan` y falla si el poller deja de pedirlos.
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
