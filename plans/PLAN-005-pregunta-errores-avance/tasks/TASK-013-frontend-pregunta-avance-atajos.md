# TASK-013 — Frontend: pregunta, errores, avance del plan, favicon, contadores y rama

- **Plan:** PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master (no código: el backend va en paralelo)
- **Estado:** `pending`

---

## Objetivo
La UI muestra los campos nuevos y los atajos de DD-25.

## Contexto verificado
- Leé el master entero, sobre todo §4 y DD-25.
- `web/app.js`: `card`, `render`, `groupByPlan` (hoy agrupa por `orq.plan`), `planTotals`, `matchesQuery`, `loadFilters`/`saveFilters`, `visible`; las funciones puras se exportan para `web/app.test.js` (bloque con `groupByPlan: groupByPlan, …`). Seguí ese patrón: lógica pura exportada y testeada con `node --test`.
- El favicon actual está inline en `web/index.html` (círculo `#d97757`). Los contadores están en `<span class="counter c-waiting"><b id="n-waiting">`.
- **Solo `textContent`/`createElement`. Prohibido `innerHTML`.** Todo texto de `ask`, `branch` y `plan.name` es hostil: probalo con `<img src=x onerror=alert(1)>`.
- `?fixture` carga `web/fixture.json`: agregá sesiones con cada caso (pregunta con `more`, permiso `tool` en `waiting`, `text` en `idle`, `errors.high`, `errors` sin `high`, dos workers del mismo `plan` con avance, sesión con `plan` sin `orq`, rama larga, rama hostil). IDs y textos sintéticos.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/fixture.json`

## Pasos
- [ ] **Paso 1** — `ask` en la tarjeta según DD-25 (waiting/blocked destacado; idle chico; busy nada; `tool` en waiting = "pide permiso: …"; "+N preguntas").
- [ ] **Paso 2** — Insignia de errores (`high`) y tooltip con el conteo.
- [ ] **Paso 3** — Grupo por `plan.path` (fallback `orq.plan`), encabezado con avance y barra de progreso accesible (`role="progressbar"` + `aria-valuenow/max`).
- [ ] **Paso 4** — Favicon con número (canvas → `toDataURL`, "9+" desde 10, original con 0) y solo se regenera si el número cambió.
- [ ] **Paso 5** — Contadores como `<button>` que filtran por estado (toggle, `aria-pressed`, persistido con los filtros).
- [ ] **Paso 6** — Rama en la tarjeta (monoespaciada, truncada con `title`) y en la búsqueda.

## Criterios de aceptación
- [ ] `node --test web/`: texto de `ask` por estado/kind; agrupación por `plan.path` con fallback; texto del avance; etiqueta del favicon (0, 3, 10+); filtro por estado; la búsqueda encuentra por rama.
- [ ] `grep -n innerHTML web/app.js` vacío.
- [ ] Con `?fixture`: claro y oscuro, 1280 y 360 px, sin scroll horizontal; texto hostil inerte. (Si no podés abrir un navegador, decilo en el Resultado: el orquestador lo verifica.)
- [ ] `node --check web/app.js`, `node --test web/`, `go test ./...` (el embed compila) en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
