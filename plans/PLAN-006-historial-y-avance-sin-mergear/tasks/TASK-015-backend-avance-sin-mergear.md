# TASK-015 — Backend: avance del plan con worktrees sin mergear

- **Plan:** PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `done`

---

## Objetivo
`plan.unmerged` y contadores que incluyen las tasks cerradas en worktrees sin mergear (DD-28).

## Contexto verificado
- Leé el master entero, sobre todo §2, §4 y DD-28.
- `internal/plans/plans.go`: `Find(number, repoRoot, home)`, `inDir`, `progress`, `taskState` (cache por `mtime`+`size`). `Progress` y `agents.Plan` tienen los mismos campos (el poller convierte directo): agregá `Unmerged` a los DOS.
- Un worktree se registra en `<repo>/.git/worktrees/<x>/gitdir`, cuyo contenido es la ruta absoluta al archivo `.git` del worktree (ej. `/home/u/.orq/trees/claude-dashboard/claude-dashboard/plan-5-task-011/.git`). La raíz del worktree es el directorio de esa ruta. Puede apuntar a algo que ya no existe (worktree borrado sin `prune`): ignoralo.
- El plan "está dentro del repo" si su ruta empieza con `<repoRoot>/plans/`. En el workspace multi-repo el plan está en un padre del repo → no aplica.
- **Sin ejecutar `git`. Solo lectura. Fixtures sintéticos** en `t.TempDir()`.

## Archivos
- **Modificar:** `internal/plans/plans.go`, `internal/plans/plans_test.go`, `internal/agents/agents.go` (campo `Unmerged` en `Plan`), `internal/server/server_test.go` (si hace falta para el campo), `README.md`

## Pasos
- [ ] **Paso 1** — Listar worktrees del repo desde `.git/worktrees/*/gitdir` (ignorar rotos).
- [ ] **Paso 2** — En `progress`, para planes dentro del repo: por cada task, la copia del worktree gana si existe, tiene estado distinto y `mtime` más nuevo; contar `Unmerged`. Con dos worktrees que la tocan, gana el `mtime` más nuevo.
- [ ] **Paso 3** — README.

## Criterios de aceptación
- [ ] Test: task `pending` en main y `done` en un worktree más nuevo → `done`, `unmerged=1`.
- [ ] Test: worktree con la copia MÁS VIEJA o con el mismo estado → no cambia, `unmerged=0`.
- [ ] Test: task que solo existe en el worktree no suma al total.
- [ ] Test: `gitdir` que apunta a un worktree borrado → ignorado sin error.
- [ ] Test: plan en un padre del repo (plan en un directorio padre del repo) → sin overlay.
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
- **Estado:** done
- **Commit:** `4307ada` (rama `feature/plan-6-task-015`)
- **Archivos:** `internal/plans/plans.go`, `internal/plans/plans_test.go`, `internal/agents/agents.go` (campo `Unmerged`), `README.md`. `server_test.go` no hizo falta.
- **Qué se hizo:** `worktreeRoots` lee `.git/worktrees/*/gitdir` (sin `git`; ignora vacíos, relativos y raíces inexistentes). `overlayState` elige la copia del worktree con estado distinto y `mtime` más nuevo (entre varios, la más nueva). `progress` aplica el overlay solo si el plan está en `<repoRoot>/plans/` (`inDir` pasa `repoRoot` solo cuando `dir == repoRoot`; plan en un padre → sin overlay). Tasks que solo existen en el worktree no suman. `Unmerged` agregado a `Progress` y `agents.Plan` (json `unmerged`).
- **Criterios:** los 5 tests cubiertos y corridos (`TestUnmergedWorktreeWins`, `...OlderOrSameStateIgnored`, `...WorktreeOnlyTaskNotCounted`, `...DeletedWorktreeIgnored`, `...PlanInParentNoOverlay`) + `...NewestWorktreeWins`.
- **Verificado por:** `gofmt -l .` vacío; `go vet ./...` limpio; `go test -race ./...` → ok en agents, orq, plans, server, transcript.
- **Para el orquestador:** el frontend (TASK-016) lee `plan.unmerged`. No se tocó `server.go`/`main.go`.
- **Gotcha:** el mtime de la copia del worktree se compara contra el de `main`; si el merge copia el archivo con mtime nuevo y mismo estado, `unmerged` vuelve a 0 solo (estado igual).
