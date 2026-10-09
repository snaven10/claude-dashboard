# TASK-012 — Backend: rama de git (`branch`) y avance del plan (`plan`)

- **Plan:** PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `done`

---

## Objetivo
Cada sesión trae `branch` (DD-24) y `plan` (DD-23) según el contrato §4 del master.

## Contexto verificado
- Leé el master entero, sobre todo §2, §4, DD-23 y DD-24.
- `internal/agents/agents.go` → `projectName`/`resolveProject` ya leen `.git` y siguen el `gitdir:` de un worktree hasta el repo principal. Reusá esa lógica (refactor chico) para: (a) leer `HEAD` del gitdir de la sesión → `branch`; (b) obtener la raíz del repo principal → punto de partida para buscar `plans/`. **Sin ejecutar `git`.** OJO: en un worktree el `HEAD` está en el gitdir del worktree (`<repo>/.git/worktrees/<x>/HEAD`), no en el del repo principal.
- `projectName` cachea por cwd para siempre; `HEAD` y los `Estado:` cambian → esos NO se cachean para siempre (cache por `(mtime,size)` o releer, son archivos chicos).
- Paquete nuevo `internal/plans`: dado `(número, raízRepo, home)` devuelve el avance. Formatos reales de `**Estado:**` que el parser debe tolerar (todos vistos hoy): `` `pending` ``, `done` (sin backticks), `Pendiente`, `pendiente`, `⬜ PENDIENTE · **Depende de:** …`, `🟢 \`pending\` — **DESBLOQUEADA…**`, `` `done` — cerrada 2026-08-30… ``, `sigue \`in_progress\`.`, `` `skipped` — DIFERIDO… ``.
- Número de plan: `orq.Job.PlanTask()` (`internal/orq/orq.go`) si el key lo trae; si no, de la rama con `(?i)plan-?(\d+)`. Ej. reales de rama: `feat-plan-042-latido-front`, `plan-4-task-010`.
- Layouts reales: `~/personal/claude-dashboard/plans/PLAN-004-abrir-en-telefono/tasks/TASK-010-….md` (plan en la raíz del repo) y `~/proyectos/mi-empresa/plans/PLAN-042-…/tasks/` donde el repo es `~/proyectos/mi-empresa/web-frontend` (plan en un PADRE del repo). Nunca subir más allá de `$HOME` (inyectable en tests).
- El cableado va en `internal/server/poller.go` (`enrich`); hay un test por campo que falla si el poller deja de pedirlo (`TestSessionsCarryTokens`): hacé lo mismo.
- **Fixtures sintéticos** en `t.TempDir()`: armá repos falsos con `.git/HEAD`, worktree con `.git` archivo `gitdir: …`, y `plans/` falsos.

## Archivos
- **Crear:** `internal/plans/plans.go`, `internal/plans/plans_test.go`
- **Modificar:** `internal/agents/agents.go` (campos `Branch`, `Plan` + tipo `Plan`; lectura de HEAD), `internal/agents/agents_test.go`, `internal/server/poller.go`, `internal/server/server_test.go`, `README.md`

## Pasos
- [x] **Paso 1** — Rama: gitdir de la sesión → `HEAD` → nombre de rama, sha corto si está desacoplado, `null` sin `.git`; validación de DD-24.
- [x] **Paso 2** — `internal/plans`: buscar `plans/PLAN-<N>-*` subiendo desde la raíz del repo principal hasta `$HOME`; N como entero; parsear `Estado:` (DD-23) de cada `tasks/*.md`; contar por estado; cache por archivo.
- [x] **Paso 3** — Número de plan (orq key → rama) y cableado en `enrich`; tests del server. README.

## Criterios de aceptación
- [x] Test rama: repo normal, worktree (`.git` archivo, HEAD del worktree ≠ HEAD del principal), HEAD desacoplado, sin `.git`, HEAD con basura → `null`.
- [x] Test parser: TODAS las variantes de `Estado:` listadas en Contexto dan el estado correcto; una línea sin vocabulario → `unknown`; solo cuenta la PRIMERA línea `**Estado:**`.
- [x] Test búsqueda: plan en la raíz del repo; plan en un padre; `PLAN-003` matchea N=3; N=13 NO matchea `PLAN-042`; no sale de `$HOME`; sin plan → `null`.
- [x] Test: un `Estado:` que cambia en disco se refleja en la siguiente llamada.
- [x] Test en el server: el snapshot trae `branch` y `plan` y falla si el poller deja de pedirlos.
- [x] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
- **Commit de código:** `a02f582` (rama `feature/plan-5-task-012`).
- **Creados:** `internal/plans/plans.go`, `internal/plans/plans_test.go`, `internal/agents/git.go` (`findGit`, `Branch`, `RepoRoot`).
- **Modificados:** `internal/agents/agents.go` (`Session.Branch`, `Session.Plan`, tipo `Plan`), `internal/agents/agents_test.go`, `internal/server/poller.go` (`enrich` → `agents.Branch`, `findPlan`, `planNumber`), `internal/server/server_test.go`, `README.md`.
- **Criterios cubiertos por tests que corrieron:** rama (repo normal, subdirectorio, worktree con HEAD ≠ principal, desacoplado, sin `.git`, basura, >200 chars, HEAD que cambia); parser (las 9 variantes reales, sin vocabulario → `unknown`, solo la primera línea `**Estado:**`, más allá de la línea 40 se ignora); búsqueda (raíz, padre, `PLAN-003`=3, 13 no matchea `PLAN-042`, no sale de `$HOME`, sin plan → nil); `Estado:` que cambia se refleja; server (`TestSessionsCarryBranchAndPlan`: rama del worktree, número por key de orq gana sobre la rama, `null` con clave presente).
- **Desvíos:** (1) `internal/agents/git.go` es archivo nuevo, no listado; `resolveProject` NO se refactorizó (tiene cache y su propio contrato), `findGit` duplica el recorrido a propósito para no tocar `projectName`. (2) No agregué campos inyectables en `Server` (`server.go` no estaba en la lista): el poller llama directo a `agents.Branch`/`plans.Find` y el test usa un `HOME` temporal. (3) Sin trailer `Claude-Session` en el commit: la regla del usuario prohíbe atribución de AI y el hook la bloquea.
- **Para el orquestador:** `Session` y `poller.go`/README se cruzan con TASK-011; no se tocó `main.go`.
- **Verificado por** (`gofmt -l .` vacío; `go vet ./...` limpio; `go test -race ./...`): `ok` en agents, orq, plans, server, transcript.
