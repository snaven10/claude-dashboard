# TASK-016 — Frontend: vista de historial y "sin mergear"

- **Plan:** PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master (no código: el backend va en paralelo)
- **Estado:** `pending`

---

## Objetivo
La UI de DD-29: panel de historial y `(N sin mergear)` en el encabezado del plan.

## Contexto verificado
- Leé el master entero, sobre todo §4 y DD-29.
- `web/app.js`: el panel de la línea de tiempo (`openTimeline`) muestra cómo abrir un panel que pide un endpoint, con foco y cierre; con `?fixture` usa `web/fixture-timeline.json`. Hacé lo mismo: `web/fixture-history.json` (sintético, 30 días con algunos en 0, 7 modelos para ver `"otros"`, un nombre de proyecto hostil `<img src=x onerror=alert(1)>`) y agregalo a `web/embed.go`.
- `planHeading` arma el encabezado del grupo; agregale el `(N sin mergear)` y su test.
- `formatTokens` ya formatea números grandes.
- **Solo `textContent`/`createElement`. Prohibido `innerHTML`.** Funciones puras exportadas y testeadas con `node --test`.

## Archivos
- **Crear:** `web/fixture-history.json`
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/embed.go`, `web/fixture.json` (un plan con `unmerged`)

## Pasos
- [ ] **Paso 1** — Lógica pura: escala de barras (máximo del período = 100 %, días en 0 sin barra pero con espacio), día seleccionado, filas por modelo/proyecto.
- [ ] **Paso 2** — Botón "📊 historial" y panel: 30 barras de output (hoy marcado, `title` con fecha y valor, click selecciona el día, accesibles por teclado), totales del día y tablas; `ready:false` → "calculando historial…" + reintento a los 3 s.
- [ ] **Paso 3** — `planHeading` con `(N sin mergear)`.

## Criterios de aceptación
- [ ] `node --test web/`: escala (incluye todos en 0 sin dividir por cero), selección de día, encabezado con y sin `unmerged`.
- [ ] `grep -n innerHTML web/app.js` vacío.
- [ ] Con `?fixture`: panel de historial en claro y oscuro, 1280 y 360 px, sin scroll horizontal; texto hostil inerte. (Si no podés abrir un navegador, decilo: lo verifica el orquestador.)
- [ ] `node --check web/app.js`, `node --test web/`, `go test ./...` (el embed compila) en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
