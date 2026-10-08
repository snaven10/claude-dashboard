# TASK-004 — Frontend vanilla: tarjetas por sesión

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** Contrato del design (puede ir en paralelo con TASK-001/002)
- **Estado:** `pending`

---

## Objetivo
`web/index.html` + `app.js` + `app.css` muestran una tarjeta por sesión, ordenadas por urgencia, actualizadas por SSE.

## Contexto verificado
- Contrato y orden de UI en el design. Sin build, sin framework, sin CDN (tiene que andar offline).

## Archivos
- **Crear:** `web/index.html`, `web/app.js`, `web/app.css`, `web/fixture.json` (para desarrollar sin backend)

## Pasos
- [ ] **Paso 1 — layout**: header con contadores (esperándote / trabajando / idle), grilla de tarjetas.
- [ ] **Paso 2 — tarjeta**: nombre/título, proyecto, badge de estado (color + texto, no solo color), `waitingFor`, último prompt (2 líneas, expandible), última acción, "hace X min", ícono si `remote`.
- [ ] **Paso 3 — SSE** con `EventSource('/api/events')`; reconexión automática; indicador "desconectado".
- [ ] **Paso 4 — filtros**: por proyecto, ocultar `done`/`idle`; guardar en `localStorage`.
- [ ] **Paso 5 — botón copiar** `claude attach <id>` (background) o `cd <cwd> && claude --resume <sessionId>`.
- [ ] **Paso 6 — `document.title`** con `(N) esperándote` para verlo desde la pestaña.

## Criterios de aceptación
- [ ] Abriendo `index.html?fixture` renderiza `fixture.json` sin server.
- [ ] Las sesiones `waiting`/`blocked` siempre arriba.
- [ ] Usable a 360 px de ancho y en dark mode (`prefers-color-scheme`).
- [ ] Texto de prompts insertado con `textContent`, nunca `innerHTML` (los prompts traen HTML/código).

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

