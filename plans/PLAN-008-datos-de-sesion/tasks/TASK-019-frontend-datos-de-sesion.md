# TASK-019 — Frontend: datos nuevos en la bandeja

- **Plan:** PLAN-008 — Datos nuevos por sesión
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-017 (PLAN-007) mergeada en `main`; contrato §4
- **Estado:** `done`

---

## Objetivo
Mostrar en filas y detalle de la bandeja los campos de §4 según DD-36 y la maqueta (`../../PLAN-007-bandeja/maqueta.html`).

## Contexto verificado
- Leé el master entero (§4, DD-36) y la maqueta: chips de la fila, bloque "Mientras no estabas", stats "turno actual" y "líneas", sección "Links".
- La bandeja ya está en `web/app.js` (TASK-017): fila compacta y panel de detalle; funciones puras exportadas y testeadas en `web/app.test.js`.
- URLs de PR son de GitLab interno: validá en el cliente solo `https://` (sin espacios ni comillas) antes de usarlas en `href`; artifacts solo `https://claude.ai/`.
- **Solo `textContent`/`createElement`; prohibido `innerHTML`.**
- `web/fixture.json`: agregá a varias sesiones los 7 campos con datos sintéticos (incluí un PR con URL hostil `javascript:alert(1)` y un `away.text` hostil) para probar que no se usan.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json`

## Pasos
- [x] **Paso 1** — Lógica pura + tests: chips de la fila, minutos de turno, validación de URLs, línea secundaria con `away`.
- [x] **Paso 2** — Fila y detalle según DD-36.
- [x] **Paso 3** — Búsqueda por PR y repo.

## Criterios de aceptación
- [x] `node --test web/`: chips (subagentes solo activos > 0, turno solo busy ≥ 15 min, permisos ocultos en `default`), URL hostil descartada, `away` como línea secundaria solo fuera de `busy`.
- [x] `grep -n innerHTML web/app.js` vacío.
- [ ] (NO verificado: sin navegador) Con `?fixture`: claro/oscuro, 1280 y 360 px sin scroll horizontal; hostil inerte. (Si no podés abrir navegador, decilo.)
- [x] `node --check web/app.js`, `node --test web/`, `go test ./...` en verde y pegados en `Verificado por`.

## Resultado
- **Estado:** done (un criterio sin verificar, ver abajo)
- **Commit:** `e95a35c` en rama `feature/plan-8-task-019`
- **Archivos:** `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json`
- **Hecho:** lógica pura exportada (`validPrUrl`, `validArtifactUrl`, `validPrs`, `validArtifacts`, `turnMinutes`, `awayLine`, `sessionChips`); chips en fila y detalle (subagentes activos > 0, último PR, `+A −R`, turno busy ≥ 15 min, permisos ≠ `default`); `away.text` como línea secundaria fuera de `busy` (tiene prioridad sobre el `ask` de idle); en el detalle, bloque "Mientras no estabas", stats "turno actual"/"líneas", sección "Links" y `permisos: X` en la meta; búsqueda por `PR #N`, `N` y repo. Fixture con los 7 campos en varias sesiones, incluida la hostil (`javascript:` en PR y artifact, `away` con HTML, `permissionMode` inválido).
- **Decisión:** un PR con URL inválida se descarta entero (no sale chip ni link ni se busca por él).
- **Verificado por:** `node --check web/app.js` OK; `node --test web/` 53/53 pass; `grep innerHTML web/app.js` vacío; `gofmt -l .` vacío; `go vet ./...` OK; `go test -race ./...` OK (todos los paquetes).
- **NO verificado:** render con `?fixture` en claro/oscuro, 1280 y 360 px sin scroll horizontal (no hay navegador en este entorno). El CSS usa `min-width:0`, `overflow-wrap:anywhere` y `flex-wrap`, pero hay que mirarlo a ojo.
