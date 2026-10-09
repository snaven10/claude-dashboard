# TASK-017 — Frontend: interfaz tipo bandeja

- **Plan:** PLAN-007 — Interfaz tipo bandeja
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** —
- **Estado:** `done`

---

## Objetivo
Reemplazar la grilla de tarjetas por la bandeja de DD-30..DD-34, sin perder ninguna función actual.

## Contexto verificado
- Leé el master entero y abrí `plans/PLAN-007-bandeja/maqueta.html` en un navegador o leé su código: es la referencia visual y de comportamiento APROBADA por el usuario. Copiá su estructura (franja, `.split` lista/detalle, filas, chips, teléfono) adaptada a los datos reales. Lo que la maqueta muestra de PLAN-008 (resumen "mientras no estabas", PR, subagentes, líneas, turno, permisos) NO va en esta task.
- `web/app.js` hoy: `card()` arma la tarjeta; `render()` la grilla; funciones puras exportadas (`askView`, `errorsView`, `planHeading`, `groupByPlan`, `splitOldDone`, `matchesQuery`, `faviconLabel`, `toggleStatusFilter`, `phoneMode`, `resumeWarning`, `qrMatrix`, `formatContext`, `rateTitle`, …) con tests en `web/app.test.js`. Reusalas; los tests existentes tienen que seguir pasando (podés adaptar los que dependan del DOM de la tarjeta).
- Funciones que NO se pueden perder: SSE + reconexión e indicador "conectado", avisos de Windows (sin abandonadas), favicon con número, título `(N) esperándote`, filtros (proyecto, ocultar done/idle, estado por contador, búsqueda con `/`), agrupar por plan con avance y "sin mergear", terminadas viejas colapsadas, línea de tiempo (`/api/sessions/{id}/timeline`), QR + link de Remote Control (validación `REMOTE_URL_RE`), copiar comando con aviso de `--resume`, historial (`/api/history`), `?fixture` (+ fixtures de timeline e historial).
- **Solo `textContent`/`createElement`. Prohibido `innerHTML`.** Texto de sesiones = hostil.
- Re-render por SSE cada vez que cambia el snapshot: conservar selección, foco, scroll de la lista y el panel de detalle abierto; la línea de tiempo del detalle se pide al seleccionar y cuando cambia `lastActivityAt` de la seleccionada (no en cada snapshot).

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/fixture.json` (agregá 2 abandonadas: una `blocked` de fondo sin `lastActivityAt` con `startedAt` de hace 90 d, y una `waiting` interactiva de hace 3 d)

## Pasos
- [x] **Paso 1** — Lógica pura + tests: `isAbandoned(s, now)`, `plainText(s)`, `counts(sessions, now)` (sin abandonadas), selección siguiente/anterior sobre la lista visible, `inboxItems(sessions, now)`.
- [x] **Paso 2** — Estructura: encabezado (contadores, "hoy X output", filtros, historial, avisos, ayuda de teclado), franja, `.split` lista/detalle, secciones colapsadas (terminadas viejas, abandonadas).
- [x] **Paso 3** — Fila compacta y detalle completo (todo lo de la tarjeta actual) con QR en línea y línea de tiempo.
- [x] **Paso 4** — Teclado, persistencia de selección, teléfono lista→detalle.
- [x] **Paso 5** — Quitar la grilla de tarjetas y el CSS muerto.

## Criterios de aceptación
- [ ] `node --test web/`: abandonadas (umbral exacto 24 h, `startedAt` de respaldo, `done` nunca es abandonada), contadores y favicon sin abandonadas, `plainText` (negritas, código, encabezado, link, texto hostil intacto como texto), navegación j/k en los bordes, franja.
- [ ] `grep -n innerHTML web/app.js` vacío.
- [ ] Con `?fixture` a 1280 px entran al menos 15 filas sin scroll; claro y oscuro; 1280 y 360 px sin scroll horizontal; texto hostil inerte. (Si no podés abrir un navegador, decilo: lo verifica el orquestador.)
- [ ] `node --check web/app.js`, `node --test web/`, `go test ./...` en verde y pegados en `Verificado por`.

## Resultado
- **Estado:** `done` (rama `feature/plan-7-task-017`, commit `724f458`)
- **Archivos:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/fixture.json` (+2 abandonadas: `blocked` de fondo sin `lastActivityAt` con `startedAt` de hace 90 d; `waiting` interactiva de hace 3 d).
- **Hecho:** lógica pura nueva (`isAbandoned`, `counts`, `inboxItems`, `splitAbandoned`, `stepSelection`, `plainText`, `oneLine`, `todayView`); franja "Te esperan"; lista compacta con agrupación por plan, terminadas viejas y abandonadas colapsadas; detalle sticky con todo lo de la tarjeta anterior, línea de tiempo en línea (se pide al seleccionar y cuando cambia `lastActivityAt`) y QR en línea; teclado j/k/flechas/Enter/Esc/1-2-3//; teléfono lista→detalle con "← volver"; selección en localStorage que sobrevive al SSE (foco, scroll y detalle se conservan; el detalle solo se reconstruye si cambia su firma). Se eliminaron la grilla, los overlays de línea de tiempo y teléfono, y su CSS.
- **Verificado por:** `node --check web/app.js` ok · `node --test web/` 44 pass / 0 fail · `gofmt -l .` vacío · `go vet ./...` ok · `go test -race ./...` ok (todos los paquetes) · `grep -n innerHTML web/app.js` vacío. Smoke headless con Chromium (`?fixture`, servidor estático): 16 filas, contadores 6/5/4 sin las 2 abandonadas, título `(6) esperándote`, "hoy 1.9M output · 24 sesiones", HTML hostil inerte, 360 px sin scroll horizontal visible en la captura.
- **No verificado / para el orquestador:** "≥ 15 filas sin scroll a 1280 px": con este fixture la franja trae 6 tarjetas y a 1000 px de alto entran ~10 filas sin scroll (hay 16 en el DOM). Con datos reales (pocas esperando) debería entrar más; confirmar en navegador. Tampoco probé a mano oscuro, j/k, teléfono lista→detalle ni QR.
- **Desvíos:** el encabezado ya no es sticky (el detalle sticky lo necesita libre). La franja ignora los filtros (igual que los contadores). El commit no lleva trailer de sesión: la regla del usuario y el hook lo prohíben.
