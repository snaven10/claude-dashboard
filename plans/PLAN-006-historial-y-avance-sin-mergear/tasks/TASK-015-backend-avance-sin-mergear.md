# TASK-015 — Backend: avance del plan con worktrees sin mergear

- **Plan:** PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `pending`

---

## Objetivo
`plan.unmerged` y contadores que incluyen las tasks cerradas en worktrees sin mergear (DD-28).

## Contexto verificado
- Leé el master entero, sobre todo §2, §4 y DD-28.
- `internal/plans/plans.go`: `Find(number, repoRoot, home)`, `inDir`, `progress`, `taskState` (cache por `mtime`+`size`). `Progress` y `agents.Plan` tienen los mismos campos (el poller convierte directo): agregá `Unmerged` a los DOS.
- Un worktree se registra en `<repo>/.git/worktrees/<x>/gitdir`, cuyo contenido es la ruta absoluta al archivo `.git` del worktree (ej. `/home/u/.orq/trees/claude-dashboard/claude-dashboard/plan-5-task-011/.git`). La raíz del worktree es el directorio de esa ruta. Puede apuntar a algo que ya no existe (worktree borrado sin `prune`): ignoralo.
- El plan "está dentro del repo" si su ruta empieza con `<repoRoot>/plans/`. En MI-EMPRESA el plan está en un padre del repo → no aplica.
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
- [ ] Test: plan en un padre del repo (layout MI-EMPRESA) → sin overlay.
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
