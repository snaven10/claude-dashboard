# TASK-007 — Frontend: avisos, sin actividad, orq, tokens y línea de tiempo

- **Plan:** PLAN-002 — Alertas, workers de orq, tokens y línea de tiempo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-002-design.md` (no del código de TASK-006: van en paralelo)
- **Estado:** `done`

---

## Objetivo
La UI usa los campos nuevos del contrato y avisa cuando una sesión te necesita.

## Contexto verificado
- Contrato, DD-8 y DD-12 en el design. Frontend vanilla de PLAN-001 en `web/` (sin framework, sin build, sin CDN).
- Todo texto externo con `textContent`, nunca `innerHTML` (los prompts son hostiles: el fixture ya trae `<script>` e `<img onerror>`).

## Archivos
- **Modificar:** `web/index.html`, `web/app.js`, `web/app.css`, `web/fixture.json` (agregar `stale`, `orq`, `tokens` y casos nulos)
- **Crear:** `web/fixture-timeline.json` (para probar el panel con `?fixture` sin server)

## Pasos
- [x] **Paso 1 — avisos (DD-12)**: botón "activar avisos"; notificar solo en la transición a `waiting`/`blocked`; nunca en el primer snapshot; on/off en `localStorage`; click en el aviso enfoca la pestaña.
- [x] **Paso 2 — sin actividad**: badge "sin actividad hace X" cuando `stale`.
- [x] **Paso 3 — orq**: en la tarjeta, `PLAN-1 · TASK-005 · specialist`; toggle "agrupar por plan" con contadores por grupo (persistido).
- [x] **Paso 4 — tokens**: total legible (`1.2M`), detalle en `title`; sin precio en dólares (DD-11).
- [x] **Paso 5 — línea de tiempo**: click en la tarjeta abre un panel con `/api/sessions/{id}/timeline` (o `fixture-timeline.json` con `?fixture`); cerrar con Esc; usable a 360 px.

## Criterios de aceptación
- [ ] Con `?fixture`: badges, agrupación, tokens y panel se ven bien en claro/oscuro, a 1280 y 360 px, sin scroll horizontal.
- [ ] Con el fixture el texto hostil sigue inerte también dentro del panel de línea de tiempo.
- [ ] La lógica de transición (qué sesiones notificar entre dos snapshots) es una función pura exportable y probada con `node --test` (sin dependencias).
- [ ] `node --check web/app.js` y los tests de node en verde, pegados en `Verificado por`.

## Resultado
- **Estado final:** done (commit 94b1813)
- **Resumen:** Avisos del navegador en la transición a waiting/blocked (botón, localStorage, nunca en el primer snapshot, click enfoca la pestaña); badge "sin actividad hace X"; línea `PLAN-N · TASK-NNN · specialist` y toggle "agrupar por plan" con contadores (persistido); tokens legibles (`1.2M`) con detalle en `title`, sin dólares; panel de línea de tiempo (click en tarjeta o botón, Esc/✕/fondo cierran, `fixture-timeline.json` con `?fixture`). Todo texto externo con `textContent`. Lógica pura (`notifyTargets`, `formatTokens`, `groupByPlan`, …) exportada por `module.exports` solo bajo node.
- **Archivos tocados:** `web/index.html`, `web/app.js`, `web/app.css`, `web/fixture.json`, `web/fixture-timeline.json` (nuevo), `web/app.test.js` (nuevo, no estaba en la lista de la task; no se embebe).
- **Verificado por:** `node --check web/app.js` OK; `node --test web/` → 8 pass, 0 fail; `gofmt -l .` vacío; `go vet ./...` OK; `go test -race ./...` OK. NO verificado: render visual en navegador (claro/oscuro, 1280/360 px, sin scroll horizontal) ni la notificación real; CSS escrito con `min-width:0`/`overflow-wrap`, pero sin ojos. **Gotcha para el merge:** `web/embed.go` lista los archivos a mano (`index.html app.js app.css fixture.json`), así que `fixture-timeline.json` NO se sirve desde el binario y `?fixture` mostrará "No se pudo cargar la línea de tiempo" hasta agregarlo a la directiva `//go:embed` (fuera de los archivos de esta task).
