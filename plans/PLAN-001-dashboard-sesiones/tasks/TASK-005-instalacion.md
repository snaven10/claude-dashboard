# TASK-005 — Instalación: Makefile, systemd --user, README + `project` por repo

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-003
- **Estado:** `done`

---

## Objetivo
`make install` deja el binario en `~/.local/bin` y el panel arrancando solo con la sesión de usuario.

## Contexto verificado
- WSL2 con systemd como PID 1; `systemctl --user` en estado `running`.
- El servicio necesita `claude` en PATH → la unidad debe fijar `Environment=PATH=%h/.local/bin:/usr/bin:/bin`.

## Archivos
- **Crear:** `Makefile` (`build`, `test`, `install`, `uninstall`), `deploy/claude-dashboard.service`, `README.md`
- **Modificar:** `internal/agents/agents.go` (derivación de `project`), `internal/agents/agents_test.go`

## Pasos
- [ ] **Paso 1 — Makefile.** `install` copia binario + unidad a `~/.config/systemd/user/` y hace `systemctl --user daemon-reload && enable --now`.
- [ ] **Paso 2 — unidad** con `Restart=on-failure`, PATH explícito.
- [ ] **Paso 3 — README**: qué muestra, de dónde saca los datos, limitaciones (formatos internos F2/F3).
- [ ] **Paso 4 — `project` = repo, no worktree** (agregado 2026-10-08 tras la QA del lote 3). Hoy `project = filepath.Base(cwd)` (`agents.go:197`) y las sesiones de `orq` corren en worktrees (`~/.orq/trees/<ws>/<repo>/plan-1-task-003`), así que el filtro muestra `plan-1-task-003` en vez de `claude-dashboard`. Regla: subir desde `cwd` hasta encontrar `.git`; si es **directorio** → `project` = basename de ese directorio; si es **archivo** (`gitdir: <ruta>`) y la ruta contiene `/.git/worktrees/` → `project` = basename de lo que está antes de `/.git/worktrees/`; si no hay `.git` o algo falla → basename(cwd) como hoy. **Sin ejecutar `git`** (el poller corre cada 3 s por sesión): solo `os.Stat`/`os.ReadFile`, con cache por `cwd`. Tests con un repo y un worktree sintéticos en `t.TempDir()` (un `.git` dir y un `.git` archivo), más el caso sin repo.

## Criterios de aceptación
- [ ] Tras `make install`, `curl -s localhost:7777/api/sessions` responde sin abrir terminal extra.
- [ ] `make uninstall` deja el sistema como estaba.
- [ ] Sesión con cwd en un worktree → `project` es el nombre del repo principal; en un subdirectorio del repo → el repo; fuera de un repo → basename(cwd). Cubierto por test.

> **No corras `make install` ni `systemctl`** desde la sesión del worker: instala un servicio persistente en la máquina del usuario. Escribí el Makefile y la unidad; la instalación real la verifica el orquestador.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** done
- **Resumen:** Makefile (build/test/install/uninstall), unidad systemd --user (PATH explícito, Restart=on-failure), README, y `project` = repo (no worktree) vía `projectName` en agents.go: sube buscando `.git`; dir → basename; archivo `gitdir:` con `/.git/worktrees/` → basename del repo; si falla → basename(cwd). Sin ejecutar git, cache por cwd (sync.Map).
- **Archivos tocados:** `Makefile`, `deploy/claude-dashboard.service`, `README.md`, `internal/agents/agents.go`, `internal/agents/agents_test.go`. Commits `3b369a2` y `2a34be3` en rama feature/plan-1-task-005 (worktree plan-1-task-005).
- **Verificado por:** `gofmt -l .` vacío; `go vet ./...` limpio; `go test -race ./...` ok (agents, server, transcript) incl. `TestProjectName` (repo, subdir, worktree, subdir de worktree, .git ajeno, sin repo, cwd vacío). `make -n install uninstall` revisado. QA del orquestador (2026-10-08): la unidad traía `After=default.target` + `WantedBy=default.target` (ciclo de orden; systemd lo rompe borrando un job) → corregido en `2a34be3`. `make install` real: servicio `active`+`enabled`, `curl localhost:7777/api/sessions` responde 27 sesiones sin terminal extra, ciclo 413-442 ms, `systemd-analyze --user verify` limpio; el worker vivo en worktree sale como `claude-dashboard`. `make uninstall`: sin binario, unidad, symlink en `default.target.wants`, puerto ni proceso — estado idéntico al previo. Límite conocido: sesiones cuyo worktree ya se borró caen a basename(cwd).
