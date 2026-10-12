# TASK-039 — Vista teléfono legible (encabezado compacto, filas de 2 renglones, timeline apilada)

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `feature/plan-11-task-039` desde `main`
- **Depende de:** TASK-036
- **Estado:** `pending`

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
- [ ] **Paso 1 — encabezado compacto (≤ 760 px).** Contadores en **un** renglón (chips más chicos;
  si no entran, fila con scroll horizontal propio `overflow-x: auto`, nunca scroll de página).
  Filtros + buscador + "historial" detrás de un botón **"Filtros"** (`aria-expanded`, plegado por
  defecto, estado en `localStorage` con try/catch). `/` sigue enfocando el buscador (lo despliega).
  Meta: encabezado ≤ 120 px plegado.
- [ ] **Paso 2 — un solo botón de avisos.** En `remoteMode === 'paired'` se oculta `#btn-notify`
  (queda "avisos en este teléfono"). En local y en escritorio, sin cambios.
- [ ] **Paso 3 — filas de 2 renglones en teléfono.** Renglón 1: punto + título a todo el ancho
  (elipsis solo al final del ancho disponible). Renglón 2 (más chico, color secundario): proyecto
  (si aplica la regla actual) · hasta 2 chips · hora. Alto ≥ 44 px. Extender `rowView` con un modo
  teléfono testeable (p. ej. `opts.phone`) en vez de ramas sueltas en el render. Bandeja: misma idea
  (título a todo el ancho, motivo debajo).
- [ ] **Paso 4 — timeline apilada en teléfono.** "hora · tipo" en una línea chica arriba y el
  texto a todo el ancho debajo.
- [ ] **Paso 5 — detalle sin doble margen** en teléfono (padding/borde del card interno).
- [ ] **Paso 6 — tests.** `node --test web/`: modo teléfono de `rowView` (2 chips, sin truncar
  el título por chips), ocultar `#btn-notify` en `paired` (lógica pura si se puede). `go test -race ./...`
  y `gofmt -l .` en verde; `rg innerHTML web/app.js` vacío.
- [ ] **Paso 7 — medición.** Chromium headless con `?fixture=lista` y con `?fixture&remote=paired`
  a 390×844 y 360×740: alto del encabezado plegado, filas visibles, alto de fila, `scrollWidth ≤`
  ancho; y a 1440×900 que el escritorio no cambió (mismo conteo que TASK-029: 16/16, fila 28 px).
  Capturas fuera del repo.

## Criterios de aceptación
- [ ] Encabezado plegado ≤ 120 px a 390 de ancho; sin scroll horizontal a 360 y 390.
- [ ] Ningún título de fila se recorta por los chips en teléfono (los chips van al 2º renglón).
- [ ] Filas ≥ 44 px en teléfono; escritorio sin cambios (28 px, 16/16 visibles con el fixture).
- [ ] En `paired` hay un solo botón de avisos.
- [ ] Teclado (j/k, Enter, 1/2/3, `/`, Esc) sigue funcionando en escritorio.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
