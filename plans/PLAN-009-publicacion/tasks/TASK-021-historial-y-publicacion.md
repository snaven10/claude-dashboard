# TASK-021 — Reescribir el historial y publicar

- **Plan:** PLAN-009 — publicación
- **Especialista:** orquestador
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`)
- **Depende de:** TASK-020; se ejecuta junto con PLAN-002 TASK-012 de orq
- **Estado:** `pending`

---

## Objetivo
Repo público en GitHub cuyo historial completo no contiene nada del cliente ni de las máquinas del usuario.

## Decisiones del usuario
- Licencia: **MIT** (confirmado 2026-10-09).
- Correo de autor: **pendiente** (propio vs `noreply` de GitHub).

## Pasos
- [ ] **Paso 1** — respaldo `git clone --mirror` a `~/respaldos/claude-dashboard-pre-publicacion.git`.
- [ ] **Paso 2** — `filter-repo --replace-text reemplazos.txt --replace-message reemplazos.txt`
  (+ `--mailmap` si aplica) en un clone fresco, con la lista compartida con orq.
- [ ] **Paso 3** — `git log --all -p | rg -i <patrón completo>` vacío; `go test ./...` y tests de
  node en el HEAD reescrito.
- [ ] **Paso 4 — GATE.** Mostrar al usuario reemplazos, conteos y resultado. **Sin su OK no se pushea.**
- [ ] **Paso 5** — crear el repo público, push, comprobar README y LICENSE en la web. Reemplazar el
  repo local por el reescrito; `make install` y verificar que el servicio sigue sirviendo.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
