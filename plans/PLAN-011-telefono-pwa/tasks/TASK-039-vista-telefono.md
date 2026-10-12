# TASK-039 — Vista teléfono legible (encabezado compacto, filas de 2 renglones, timeline apilada)

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `feature/plan-11-task-039` desde `main`
- **Depende de:** TASK-036
- **Estado:** `done`

---

## Objetivo
En pantallas angostas (≤ 760 px, el breakpoint de `isPhone()`) el panel se lee y se toca bien. El
escritorio queda **idéntico** al actual.

## Contexto verificado (gate de TASK-038, 2026-10-11, PWA emparejada a 390×844 con datos reales)
- El encabezado ocupa ~330 px de 844 (≈ 40 %): contadores en 2 renglones, filtros (proyecto,
  ocultar done/idle, agrupar por plan), buscador y tres botones ("activar avisos", "avisos en este
  teléfono", "historial").
- Los títulos de las filas quedan ilegibles ("Coordinació…", "verificar…"): con `maxChips: 1` +
  "+N" + hora, el título se queda con ~12 caracteres (`rowView`, `app.js` ~1301).
- Filas de 28 px: chicas para el dedo (objetivo táctil ≥ 44 px).
- En la PWA emparejada conviven "activar avisos" (notificaciones de la pestaña, `#btn-notify`) y
  "avisos en este teléfono" (push): confunden. En modo `paired` el push reemplaza al otro.
- Detalle: la línea de tiempo usa una columna de hora + tipo que se lleva ~la mitad del ancho; el
  detalle tiene doble margen (card dentro de card).

## Archivos
- **Modificar:** `web/app.css`, `web/app.js`, `web/index.html` (solo si hace falta un contenedor para "Filtros"), `web/app.test.js`

## Pasos
- [x] **Paso 1 — encabezado compacto (≤ 760 px).** Contadores en **un** renglón (chips más chicos;
  si no entran, fila con scroll horizontal propio `overflow-x: auto`, nunca scroll de página).
  Filtros + buscador + "historial" detrás de un botón **"Filtros"** (`aria-expanded`, plegado por
  defecto, estado en `localStorage` con try/catch). `/` sigue enfocando el buscador (lo despliega).
  Meta: encabezado ≤ 120 px plegado.
- [x] **Paso 2 — un solo botón de avisos.** En `remoteMode === 'paired'` se oculta `#btn-notify`
  (queda "avisos en este teléfono"). En local y en escritorio, sin cambios.
- [x] **Paso 3 — filas de 2 renglones en teléfono.** Renglón 1: punto + título a todo el ancho
  (elipsis solo al final del ancho disponible). Renglón 2 (más chico, color secundario): proyecto
  (si aplica la regla actual) · hasta 2 chips · hora. Alto ≥ 44 px. Extender `rowView` con un modo
  teléfono testeable (p. ej. `opts.phone`) en vez de ramas sueltas en el render. Bandeja: misma idea
  (título a todo el ancho, motivo debajo).
- [x] **Paso 4 — timeline apilada en teléfono.** "hora · tipo" en una línea chica arriba y el
  texto a todo el ancho debajo.
- [x] **Paso 5 — detalle sin doble margen** en teléfono (padding/borde del card interno).
- [x] **Paso 6 — tests.** `node --test web/`: modo teléfono de `rowView` (2 chips, sin truncar
  el título por chips), ocultar `#btn-notify` en `paired` (lógica pura si se puede). `go test -race ./...`
  y `gofmt -l .` en verde; `rg innerHTML web/app.js` vacío.
- [x] **Paso 7 — medición.** Chromium headless con `?fixture=lista` y con `?fixture&remote=paired`
  a 390×844 y 360×740: alto del encabezado plegado, filas visibles, alto de fila, `scrollWidth ≤`
  ancho; y a 1440×900 que el escritorio no cambió (mismo conteo que TASK-029: 16/16, fila 28 px).
  Capturas fuera del repo.

## Criterios de aceptación
- [x] Encabezado plegado ≤ 120 px a 390 de ancho; sin scroll horizontal a 360 y 390.
- [x] Ningún título de fila se recorta por los chips en teléfono (los chips van al 2º renglón).
- [x] Filas ≥ 44 px en teléfono; escritorio sin cambios (28 px, 16/16 visibles con el fixture).
- [x] En `paired` hay un solo botón de avisos.
- [x] Teclado (j/k, Enter, 1/2/3, `/`, Esc) sigue funcionando en escritorio.

## Resultado
- **Commit de código:** `72f80cb` (rama `feature/plan-11-task-039`).
- **Archivos:** `web/app.css`, `web/app.js`, `web/index.html`, `web/app.test.js`. Sin Go ni deps nuevas.
- **Qué cambió:**
  - Botón "Filtros" (`#btn-filters`, `aria-expanded`, `aria-controls`, estado en `localStorage` con try/catch, plegado por defecto) que muestra/oculta `.filters` (proyecto, checks, buscador, avisos, historial) en ≤ 760 px. `/` lo despliega y enfoca el buscador.
  - Contadores en un renglón con `overflow-x: auto` propio.
  - `tabNotifyVisible(mode, supported)` (puro, exportado): en `paired` se oculta `#btn-notify`; `applyMode` llama a `updateNotifyBtn`.
  - `rowView(s, { phone })` devuelve `phone`; `row()` arma `.row-body` con título (1.er renglón) y `.row-sub` (proyecto · hasta 2 chips · hora). Alto ≥ 44 px.
  - Bandeja: título a todo el ancho (header con wrap). Timeline: `.ev-text` a 100 % (hora · tipo arriba). Detalle sin borde/padding de card en teléfono.
- **Verificado (corrido):** `node --test web/` 98/98; `go vet ./...` ok; `go test -race ./...` ok; `gofmt -l .` vacío; `rg innerHTML web/app.js` vacío.
- **Medición (Chromium headless, CDP, fixtures servidos por HTTP):**
  - 390×844 `?fixture=lista`: encabezado plegado 87 px, filas 49 px, scrollWidth 390 = ancho. 360×740: 87 px, 49 px, scrollWidth 360.
  - `remote=paired`: `#btn-notify` oculto; en local visible.
  - 1440×900: idéntico a la base (`git archive HEAD`): encabezado 91 px, filas 28 px, 9/9 con `?fixture` y 13/13 con `?fixture=lista`, sin cambios.
- **No verificado / matices:**
  - Teléfono real (PWA emparejada con datos reales): lo cubre TASK-038.
  - La cifra "16/16" de TASK-029 no se reprodujo con los fixtures actuales; se comparó contra la base y el conteo es el mismo.
  - Teclado en escritorio: no hay cambios en `onKey` salvo `/` (solo actúa en teléfono); no se probó interactivamente.
  - A 360×740 con `?fixture&remote=paired` ninguna fila queda en el primer viewport porque la bandeja "Te esperan" ocupa la pantalla; es contenido, no overflow.
  - Los botones de avisos (push) quedan dentro del panel "Filtros": hay que desplegarlo para activarlos.
- **Desvíos:** ninguno respecto al design. El commit va sin trailer de sesión porque el hook del usuario lo bloquea (regla: sin atribución de AI).

