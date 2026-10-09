# TASK-021 — Reescribir el historial y publicar

- **Plan:** PLAN-009 — publicación
- **Especialista:** orquestador
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`)
- **Depende de:** TASK-020; se ejecuta junto con PLAN-002 TASK-012 de orq
- **Estado:** `done`

---

## Objetivo
Repo público en GitHub cuyo historial completo no contiene nada del cliente ni de las máquinas del usuario.

## Decisiones del usuario
- Licencia: **MIT** (confirmado 2026-10-09).
- Correo de autor: **el propio** (`funes@samuelfunes.com`, confirmado 2026-10-09). Sin `--mailmap`.

## Pasos
- [ ] **Paso 1** — respaldo `git clone --mirror` a `~/respaldos/claude-dashboard-pre-publicacion.git`.
- [ ] **Paso 2** — `filter-repo --replace-text reemplazos.txt --replace-message reemplazos.txt`
  en un clone fresco, con la lista compartida con orq.
- [ ] **Paso 3** — `git log --all -p | rg -i <patrón completo>` vacío; `go test ./...` y tests de
  node en el HEAD reescrito.
- [ ] **Paso 4 — GATE.** Mostrar al usuario reemplazos, conteos y resultado. **Sin su OK no se pushea.**
- [ ] **Paso 5** — crear el repo público, push, comprobar README y LICENSE en la web. Reemplazar el
  repo local por el reescrito; `make install` y verificar que el servicio sigue sirviendo.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** `done`
- **Resumen:** historial reescrito con `git-filter-repo` 2.47.0 (`--replace-text` + `--replace-message`, lista compartida con orq), README nuevo con capturas de un fixture de demo y referencia técnica en `docs/API.md`, publicado en https://github.com/snaven10/claude-dashboard.
- **Archivos tocados:** todo el historial (108 commits reescritos); `README.md`, `docs/API.md`, `docs/img/*`.
- **Verificado por:** `git log --all -p` (código, diffs, mensajes, autor) del repo reescrito contra la lista privada de identificadores: 0 coincidencias en los dos repos. Árbol final comparado contra el original: solo cambian textos de docs (máquinas y ramas de ejemplo). Tests en el repo reescrito (dashboard: Go en verde, node 53/53; orq: `py_compile`). Objetos viejos eliminados del repo local (`reflog expire` + `gc --prune=now`; un SHA viejo ya no resuelve). Repos creados por la API de GitHub con la credencial personal, públicos, licencia MIT detectada, topics cargados, imagen del README servida (HTTP 200).
- **Desviaciones:** la primera pasada de reemplazos cambiaba la palabra común "partida(s)" y rompía comentarios legítimos; se acotó a las frases del dominio y se rehízo. README creativo nuevo pedido por el usuario al publicar.
- **Riesgos abiertos / siguiente:** los SHAs citados en `plans/` son de antes de la publicación (nota en el README). Respaldos completos pre-publicación en `~/respaldos/` (locales, nunca se suben).
