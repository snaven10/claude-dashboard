# TASK-024 — Fila de una línea, lista al 60 %, encabezado en dos renglones y timeline alta

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-024` desde `main` (con TASK-022 mergeada)
- **Depende de:** TASK-022
- **Estado:** `pending`

---

## Objetivo
Cada sesión de la lista ocupa un renglón de ≤ 28 px, la lista se lleva ~60 % del ancho, el
encabezado de la página ocupa dos renglones y la línea de tiempo del detalle muestra ≥ 10 eventos
a 1440×900. Implementa DD-37 del master (leelo entero antes de empezar).

## Contexto verificado
- `web/app.js:901-940` (`row`): hoy arma `.row-main` con `.row-title`, `.row-sub` (away / ask corto
  de idle / proyecto · rama) y `.chips`; `.row-side` con texto de estado, hora y mini barra de
  contexto (`.bar.ctx`).
- `web/app.js:876-899` (`chipsFor`): errores, ritmo, contexto (solo fuera del detalle), sin
  actividad y `sessionChips` (subagentes, PR, líneas, turno, permisos — `app.js:423-441`).
- `web/app.js:400-404` (`awayLine`), `askView` (`app.js:103-114`), `plainText` (`app.js:345-356`).
- `web/app.css:164` `.split { grid-template-columns: minmax(0, 5fr) minmax(0, 6fr) }`;
  `web/app.css:182-211` estilos de `.row`, `.dot`, `.row-main`, `.row-sub`, `.row-side`, `.ctx`;
  `web/app.css:275` `.tl-wrap .events { max-height: 220px; overflow-y: auto; }`;
  `web/app.css:236-249` `.detail` sticky con `max-height: calc(100vh - 24px)`;
  `web/app.css:329-340` teléfono `max-width: 760px`.
- `web/index.html:15-43`: encabezado en tres bloques (`.top-row`, `.counters`, `.filters`).
- `filters.project` (`app.js:495-510`) dice si el filtro de proyecto está en "Todos" (`''`).
- La firma del detalle (`renderDetail`, `app.js:1334-1355`) evita reconstruirlo en cada snapshot:
  no la rompas.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`

## Pasos
- [ ] **Paso 1 — Lógica pura + tests.** `rowView(s, { now, projectFilter })` →
  `{ title, project /* null si projectFilter !== '' */, chips /* máx. 2 */, extra /* n restantes */,
  extraTitle, when, tooltip, ariaLabel }`. Prioridad de chips: errores (`high`) > contexto `warn` >
  ritmo `high` > `stale` > turno ≥ 15 min > PR > subagentes > líneas > permisos. `tooltip` = resumen
  que hoy va en `.row-sub` (away, o ask corto de idle, o proyecto · rama), con `plainText`.
  `ariaLabel` incluye título + estado en texto + hora. Exportar y testear en `web/app.test.js`:
  orden y tope de chips, "+N", proyecto oculto con filtro, texto hostil intacto como texto.
- [ ] **Paso 2 — `row()` de un renglón.** punto · título (elipsis) · etiqueta de proyecto (muted,
  `max-width: 14ch`, elipsis) · chips · hora (tabular-nums, derecha). Quitar `.row-sub`, el texto
  de estado y la mini barra de contexto. `title` = `tooltip`; `aria-label` = `ariaLabel`.
- [ ] **Paso 3 — CSS.** `.row` en una línea (`grid-template-columns: 10px minmax(0,1fr) auto auto auto`
  o flex), `padding` vertical ~3-4 px, `align-items: center`, alto ≤ 28 px. `.split` a
  `minmax(0, 3fr) minmax(0, 2fr)`. Limpiar CSS muerto (`.row-sub`, `.row-st`, `.st-*` si queda sin
  uso, `.ctx`).
- [ ] **Paso 4 — Encabezado en dos renglones.** Renglón 1: título, conexión, contadores, "hoy …".
  Renglón 2: filtros, botones, ayuda de teclado. A 1440 px no debe envolver a un tercer renglón;
  en teléfono puede envolver.
- [ ] **Paso 5 — Bandeja más baja.** Tarjeta de dos renglones: (1) título · proyecto · hora +
  acciones a la derecha; (2) motivo en **una** línea con elipsis y texto completo en `title`. (El
  contenido del motivo lo cambia TASK-027; acá solo el alto.)
- [ ] **Paso 6 — Timeline alta.** Sacar el tope de 220 px: la lista de eventos crece y scrollea el
  detalle (que ya es sticky con `max-height`). Si el scroll anidado molesta, `max-height: min(60vh, 640px)`.
  Con 20 eventos del fixture deben verse ≥ 10 en el detalle a 1440×900 sin scrollear la página.
- [ ] **Paso 7 — Teléfono.** A 390 px la fila sigue en un renglón (la etiqueta de proyecto puede
  ocultarse; chips máx. 1 + "+N"); `body.phone-detail` y "← volver" sin cambios.

## Criterios de aceptación
- [ ] `node --check web/app.js`, `node --test web/` en verde con los tests nuevos de `rowView`;
  `rg -n innerHTML web/app.js` vacío. Pegar salida en `Verificado por`.
- [ ] `gofmt -l .`, `go vet ./...`, `go test -race ./...` en verde.
- [ ] Medición estándar (master §3) con `?fixture=lista`: alto de `.row` ≤ 28 px
  (`getBoundingClientRect().height`); filas visibles a 1440×900 **≥ 20** (la meta de 25 se cierra en
  TASK-025, cuando las esperando salen de la lista); timeline con ≥ 10 eventos visibles; sin scroll
  horizontal a 1440 y 390 px; claro y oscuro. Pegar los números junto a la línea base de TASK-022.
- [ ] j/k, Enter, 1/2/3, `/`, Esc siguen funcionando (probado a mano o declarado como no probado).
- [ ] Texto hostil del fixture inerte (sin alerta, se ve como texto).

## Riesgos
- Quitar `.row-sub` saca el "mientras no estabas" de la vista de lista: queda en `title` y en el
  detalle. Si el usuario lo extraña, se puede volver como chip; no lo agregues por tu cuenta.
- `text-overflow: ellipsis` necesita `min-width: 0` en cada nivel del grid.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
