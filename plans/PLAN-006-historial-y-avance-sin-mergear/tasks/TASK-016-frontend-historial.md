# TASK-016 — Frontend: vista de historial y "sin mergear"

- **Plan:** PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master (no código: el backend va en paralelo)
- **Estado:** `done`

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
- [x] **Paso 1** — Lógica pura: escala de barras (máximo del período = 100 %, días en 0 sin barra pero con espacio), día seleccionado, filas por modelo/proyecto.
- [x] **Paso 2** — Botón "📊 historial" y panel: 30 barras de output (hoy marcado, `title` con fecha y valor, click selecciona el día, accesibles por teclado), totales del día y tablas; `ready:false` → "calculando historial…" + reintento a los 3 s.
- [x] **Paso 3** — `planHeading` con `(N sin mergear)`.

## Criterios de aceptación
- [x] `node --test web/`: escala (incluye todos en 0 sin dividir por cero), selección de día, encabezado con y sin `unmerged`.
- [x] `grep -n innerHTML web/app.js` vacío.
- [x] Con `?fixture`: panel de historial en claro y oscuro, 1280 y 360 px, sin scroll horizontal; texto hostil inerte. (Si no podés abrir un navegador, decilo: lo verifica el orquestador.)
- [x] `node --check web/app.js`, `node --test web/`, `go test ./...` (el embed compila) en verde y pegados en `Verificado por`.

## Resultado
- **Commit:** 1e81974 (rama `feature/plan-6-task-016`)
- **Archivos:** creado `web/fixture-history.json`; modificados `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/embed.go`, `web/fixture.json` (PLAN-007 con `unmerged: 2`).
- **Hecho:** funciones puras `historyScale`, `selectDay`, `historyRows` y `planHeading` con `(N sin mergear)` (exportadas, testeadas); botón "📊 historial" y panel (30 barras `<button>` accesibles por teclado, hoy marcado, `title` con fecha y valor, click selecciona día, totales y tablas modelo/proyecto, `ready:false` → "calculando historial…" con reintento a 3 s); Esc cierra.
- **Verificado por:**
  - `node --check web/app.js` OK
  - `node --test web/`: 33 pass, 0 fail
  - `go vet ./...` y `gofmt -l .` limpios; `go test ./...` OK (el embed compila)
  - `grep -n innerHTML web/app.js` vacío (solo `textContent`/`createElement`)
- **NO verificado:** render visual con `?fixture` (claro/oscuro, 1280 y 360 px, sin scroll horizontal, texto hostil inerte) — no abrí navegador; lo verifica el orquestador. Por diseño el texto hostil va solo por `textContent`.
- **Gotchas:** el fixture trae 7 modelos/proyectos → 5 + "otros" como en el contrato. `server.go`/`main.go` no se tocaron: el backend (`/api/history`, `unmerged`) lo integra el orquestador.
