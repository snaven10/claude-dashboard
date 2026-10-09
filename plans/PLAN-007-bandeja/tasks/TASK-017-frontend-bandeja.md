# TASK-017 — Frontend: interfaz tipo bandeja

- **Plan:** PLAN-007 — Interfaz tipo bandeja
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** —
- **Estado:** `pending`

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
- [ ] **Paso 1** — Lógica pura + tests: `isAbandoned(s, now)`, `plainText(s)`, `counts(sessions, now)` (sin abandonadas), selección siguiente/anterior sobre la lista visible, `inboxItems(sessions, now)`.
- [ ] **Paso 2** — Estructura: encabezado (contadores, "hoy X output", filtros, historial, avisos, ayuda de teclado), franja, `.split` lista/detalle, secciones colapsadas (terminadas viejas, abandonadas).
- [ ] **Paso 3** — Fila compacta y detalle completo (todo lo de la tarjeta actual) con QR en línea y línea de tiempo.
- [ ] **Paso 4** — Teclado, persistencia de selección, teléfono lista→detalle.
- [ ] **Paso 5** — Quitar la grilla de tarjetas y el CSS muerto.

## Criterios de aceptación
- [ ] `node --test web/`: abandonadas (umbral exacto 24 h, `startedAt` de respaldo, `done` nunca es abandonada), contadores y favicon sin abandonadas, `plainText` (negritas, código, encabezado, link, texto hostil intacto como texto), navegación j/k en los bordes, franja.
- [ ] `grep -n innerHTML web/app.js` vacío.
- [ ] Con `?fixture` a 1280 px entran al menos 15 filas sin scroll; claro y oscuro; 1280 y 360 px sin scroll horizontal; texto hostil inerte. (Si no podés abrir un navegador, decilo: lo verifica el orquestador.)
- [ ] `node --check web/app.js`, `node --test web/`, `go test ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
