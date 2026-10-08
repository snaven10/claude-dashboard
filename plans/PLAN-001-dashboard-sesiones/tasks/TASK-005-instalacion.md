# TASK-005 — Instalación: Makefile, systemd --user, README

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

## Pasos
- [ ] **Paso 1 — Makefile.** `install` copia binario + unidad a `~/.config/systemd/user/` y hace `systemctl --user daemon-reload && enable --now`.
- [ ] **Paso 2 — unidad** con `Restart=on-failure`, PATH explícito.
- [ ] **Paso 3 — README**: qué muestra, de dónde saca los datos, limitaciones (formatos internos F2/F3).

## Criterios de aceptación
- [ ] Tras `make install`, `curl -s localhost:7777/api/sessions` responde sin abrir terminal extra.
- [ ] `make uninstall` deja el sistema como estaba.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

