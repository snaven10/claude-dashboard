# TASK-020 — Limpiar el árbol para publicar

- **Plan:** PLAN-009 — publicación
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), worktree de orq
- **Depende de:** — (primera del plan)
- **Estado:** `done`

---

## Objetivo
Ningún archivo versionado nombra al cliente del usuario, sus repos, sus planes, sus máquinas ni
rutas de su HOME. El repo trae `LICENSE` MIT y un README que sirve a un desconocido.

## Contexto verificado
Ver PLAN-009 §2 (lista de archivos con línea). Comando de inventario:
`git grep` con los nombres del cliente, sus repos y planes, la otra máquina y el usuario del HOME (lista completa en `reemplazos.txt`, fuera del repo).

## Pasos
- [x] **Paso 1 — datos de prueba.** nombre del workspace del cliente → `demo-multi` en `web/fixture-history.json`;
  el plan real → `PLAN-042-*`/`plan-042-*` en fixtures y tests (Go y node). Los tests
  que comparan textos se ajustan al mismo reemplazo, sin cambiar lo que prueban.
- [x] **Paso 2 — comentarios.** el comentario que nombraba al cliente → "plan en un directorio padre del repo (workspace multi-repo)".
- [x] **Paso 3 — agente.** la mención al cliente → "tienen datos privados del usuario".
- [x] **Paso 4 — planes.** Rutas del HOME y del workspace del cliente → `~/proyectos/mi-empresa`; el nombre real
  del plan del cliente → `PLAN-042-ejemplo`; el repo del cliente → `web-frontend`; la otra máquina → "otra máquina".
  Mantener el sentido técnico de cada hallazgo.
- [x] **Paso 5 — `LICENSE`.** MIT, 2026, titular el autor.
- [x] **Paso 6 — README.** Sección de instalación para terceros: requisitos (Go, Claude Code,
  systemd `--user`), `make install`, que es solo lectura y solo escucha en 127.0.0.1, y la nota de
  que los SHAs citados en `plans/` son de antes de la publicación.

## Criterios de aceptación
- [x] El `git grep` de inventario devuelve vacío (salvo `LICENSE` con el titular).
- [x] `go vet ./...`, `go test -race ./...` y los tests de node (`web/app.test.js`) pasan.
- [x] Mismo número de tests que antes (ninguno borrado para que pase el grep).

## Resultado
- **Estado final:** `done`
- **Resumen:** Reemplazos aplicados según pasos 1-4 (workspace del cliente→`demo-multi`, plan real→`PLAN-042`; en el test de Go el caso "un número que es prefijo de otro no matchea" pasó a "4 no matchea 042" para conservar el caso de prefijo). `LICENSE` MIT 2026 `snaven10`; README con requisitos, `make install`, solo lectura/127.0.0.1 y nota de SHAs previos a la publicación. Commit código: `03fb8f0`.
- **Archivos tocados:** `LICENSE` (nuevo), `README.md`, `.claude/agents/claude-dashboard-worker.md`, `internal/plans/plans.go`, `internal/plans/plans_test.go`, `web/app.js`, `web/app.test.js`, `web/fixture.json`, `web/fixture-history.json`, planes PLAN-001/002/005/006 (7 .md).
- **Verificado por:** `gofmt -l .` vacío; `go vet ./...` ok; `go test -race ./...` ok en todos los paquetes; `node --test web/app.test.js` 53/53 (mismo total que antes, ninguno borrado). `git grep` de inventario vacío salvo `LICENSE` y los propios `plans/PLAN-009-*`, que citan los términos para describir el trabajo (los maneja TASK-021 con `reemplazos.txt`/historial). Desvío: los planes quedaron con nombres genéricos (`~/proyectos/mi-empresa`, `PLAN-042-ejemplo`, `web-frontend`).
