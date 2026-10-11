# TASK-025 — Grupos por estado con contador; la sesión que espera sale de la lista

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-025` desde `main` (con TASK-024 mergeada)
- **Depende de:** TASK-024
- **Estado:** `pending`

---

## Objetivo
La lista se lee por grupos `Trabajando (N)` / `Idle (N)` / `Terminadas (N)` / `Abandonadas (N)`;
terminadas y abandonadas tienen contador visible también en el encabezado de la página; las
`waiting`/`blocked` no abandonadas aparecen solo en la bandeja, que entra primero en j/k.
Implementa DD-38 y DD-39 del master.

## Contexto verificado
- `web/app.js:980-1013` (`renderList`): `sorted(visible(sessions))` → `splitAbandoned` →
  `splitOldDone`; con `groupByPlanOn` usa `groupByPlan` + `groupHead`; si no, lista plana. Terminadas
  viejas y abandonadas van en `fold(...)` (`app.js:970-978`) con `showOldOn` / `showAbandonedOn`
  en `localStorage` (`OLD_KEY`, `ABN_KEY`). `navKeys` se llena en el orden de las filas.
- `splitOldDone` separa solo las `done` de **más de 24 h** (`isOldDone`, `app.js:190-202`); las
  `done` recientes hoy se mezclan con el resto. En los grupos nuevos, **todas** las `done` van a
  "Terminadas" (las recientes primero).
- `web/app.js:1015-1056` (`renderInbox`): tarjetas `article.ask-card` sin `data-key`; `inboxItems`
  (`app.js:319-325`) = waiting/blocked no abandonadas, más reciente primero; ignora filtros.
- `web/app.js:1410-1446` (`onKey`): j/k usan `stepSelection(navKeys, selectedKey, ±1)`;
  `render({ focusKey })` enfoca `rowByKey` (`app.js:1357-1361`, busca solo en `#rows .row`).
- `web/app.js:1383-1388`: si no hay selección válida, se elige `navKeys[0]`.
- `counts()` (`app.js:308-316`) no cuenta `done` ni abandonadas: hay que sumar esos dos números
  para el encabezado (función pura nueva, sin cambiar `counts`).
- `web/index.html:20-25`: contadores `c-waiting`, `c-busy`, `c-idle` (`aria-pressed`, filtros 1/2/3).

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`

## Pasos
- [ ] **Paso 1 — Lógica pura + tests.** `statusGroups(list, now)` → array ordenado de
  `{ id: 'busy'|'idle'|'done'|'abandoned', label, sessions }` **sin** las waiting/blocked no
  abandonadas, sin grupos vacíos; `done` recientes antes que viejas. `listTotals(list, now)` →
  `{ done, abandoned }`. Tests: una waiting no abandonada no aparece en ningún grupo; una waiting de
  hace 3 d sí (abandonadas); `done` reciente y vieja en "Terminadas" en ese orden; grupos vacíos fuera.
- [ ] **Paso 2 — `renderList` por grupos.** Sin plan: encabezado de grupo por cada `statusGroups`
  (`role="group"`, `aria-label` "Trabajando, 7 sesiones"); Terminadas y Abandonadas plegables con el
  `localStorage` que ya existe; el encabezado plegado es una fila completa con contador
  (`▸ Terminadas (21)`), con `aria-expanded`. Con "agrupar por plan": se mantienen `groupHead` de
  plan; dentro, orden por estado; waiting no abandonadas fuera; terminadas viejas y abandonadas al
  final como hoy pero con el mismo formato de encabezado con contador.
- [ ] **Paso 3 — Contadores en el encabezado.** Agregar dos botones discretos `N terminadas` y
  `N abandonadas` (ocultos con 0) que despliegan su grupo y lo traen a la vista (`scrollIntoView`).
  No son filtros: no usan `aria-pressed` ni teclas.
- [ ] **Paso 4 — Bandeja en la navegación.** Cada `ask-card` lleva `data-key`, `tabindex="0"`,
  `aria-selected`, y la clase/borde de selección; click en la tarjeta (fuera de sus botones)
  selecciona. `navKeys` = claves de la bandeja (en su orden) + claves de las filas. `rowByKey` busca
  en filas **y** tarjetas. La auto‑selección inicial cae en la primera de la bandeja si hay.
- [ ] **Paso 5 — Filtro "esperándote".** Con `filters.status === 'waiting'` y sin abandonadas
  visibles, la lista muestra "Las N sesiones que te esperan están arriba, en Te esperan" (no
  "No hay sesiones para mostrar").

## Criterios de aceptación
- [ ] `node --check web/app.js`, `node --test web/` en verde con los tests de `statusGroups` y
  `listTotals`; `rg -n innerHTML web/app.js` vacío.
- [ ] `gofmt -l .`, `go vet ./...`, `go test -race ./...` en verde.
- [ ] Medición estándar con `?fixture=lista` a 1440×900: **≥ 25 sesiones visibles** (filas +
  tarjetas de la bandeja) sin scroll; los encabezados `Terminadas (21)` y `Abandonadas (4)` (o sus
  botones del encabezado de página) visibles sin scroll. Ninguna sesión de la bandeja aparece
  también en `#rows` (comprobar con un `querySelectorAll` de `data-key`).
- [ ] j/k recorre bandeja → filas sin saltos ni duplicados; Enter en 390 px abre el detalle de una
  tarjeta y "← volver" regresa; 1/2/3 siguen filtrando; con `1` se ve el mensaje del Paso 5.
- [ ] "Agrupar por plan" sigue mostrando avance y "sin mergear" del Plan 42 del fixture.
- [ ] Claro y oscuro; sin scroll horizontal a 1440 y 390 px.

## Riesgos
- La bandeja ignora filtros y la lista no: con filtro de proyecto, una waiting de otro proyecto
  sigue arriba. Es el comportamiento actual (DD-32); no lo cambies.
- El foco al re-renderizar por SSE: `render()` hoy recupera el foco solo si el activo es `.row`;
  extenderlo a `.ask-card` para no perder el foco en cada snapshot.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
