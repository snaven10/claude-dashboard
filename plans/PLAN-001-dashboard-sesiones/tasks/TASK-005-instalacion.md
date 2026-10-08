# TASK-005 — Instalación: Makefile, systemd --user, README + `project` por repo

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-003
- **Estado:** `pending`

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
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

