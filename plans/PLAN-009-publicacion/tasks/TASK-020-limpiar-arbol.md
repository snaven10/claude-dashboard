# TASK-020 — Limpiar el árbol para publicar

- **Plan:** PLAN-009 — publicación
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), worktree de orq
- **Depende de:** — (primera del plan)
- **Estado:** `pending`

---

## Objetivo
Ningún archivo versionado nombra al cliente del usuario, sus repos, sus planes, sus máquinas ni
rutas de su HOME. El repo trae `LICENSE` MIT y un README que sirve a un desconocido.

## Contexto verificado
Ver PLAN-009 §2 (lista de archivos con línea). Comando de inventario:
`git grep -n -i -E 'mi-empresa|org|remota|snaven10|_FrontEnd|_BackEnd|plan-042|PLAN-042'`.

## Pasos
- [ ] **Paso 1 — datos de prueba.** `mi-empresa` → `demo-multi` en `web/fixture-history.json`;
  `PLAN-042-*`/`plan-042-*` → `PLAN-042-*`/`plan-042-*` en fixtures y tests (Go y node). Los tests
  que comparan textos se ajustan al mismo reemplazo, sin cambiar lo que prueban.
- [ ] **Paso 2 — comentarios.** "layout MI-EMPRESA" → "plan en un directorio padre del repo (workspace multi-repo)".
- [ ] **Paso 3 — agente.** "tienen datos de MI-EMPRESA" → "tienen datos privados del usuario".
- [ ] **Paso 4 — planes.** Rutas `/home/usuario/mi-empresa` → `~/proyectos/mi-empresa`; el nombre real
  del plan del cliente → `PLAN-042-ejemplo`; `web-frontend` → `web-frontend`; `remota` → "otra máquina".
  Mantener el sentido técnico de cada hallazgo.
- [ ] **Paso 5 — `LICENSE`.** MIT, 2026, titular `snaven10`.
- [ ] **Paso 6 — README.** Sección de instalación para terceros: requisitos (Go, Claude Code,
  systemd `--user`), `make install`, que es solo lectura y solo escucha en 127.0.0.1, y la nota de
  que los SHAs citados en `plans/` son de antes de la publicación.

## Criterios de aceptación
- [ ] El `git grep` de inventario devuelve vacío (salvo `LICENSE` con el titular).
- [ ] `go vet ./...`, `go test -race ./...` y los tests de node (`web/app.test.js`) pasan.
- [ ] Mismo número de tests que antes (ninguno borrado para que pase el grep).

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
