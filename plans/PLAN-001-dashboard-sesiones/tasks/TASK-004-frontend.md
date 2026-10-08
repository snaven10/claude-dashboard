# TASK-004 — Frontend vanilla: tarjetas por sesión

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** Contrato del design (puede ir en paralelo con TASK-001/002)
- **Estado:** `done`

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
- **Estado final:** done
- **Resumen:** Frontend vanilla (sin build/framework/CDN). Header con contadores (esperándote/trabajando/idle) e indicador de conexión; tarjetas ordenadas waiting/blocked → busy → idle → done (desempate por actividad reciente); badge color + ícono + texto; prompt a 2 líneas expandible por click; última acción; "hace X"; marca remoto; botón copiar (`claude attach <id>` para background, `cd '<cwd>' && claude --resume <sessionId>` para interactive); filtros proyecto / ocultar done / ocultar idle en `localStorage`; `document.title` con `(N) esperándote`; SSE `/api/events` evento `snapshot` con reconexión y fetch inicial de `/api/sessions`. Todo texto externo va con `textContent` (cero `innerHTML`). `index.html?fixture` carga `fixture.json` (sintético, incluye un prompt con HTML hostil y un cwd con comilla simple). Dark mode vía `prefers-color-scheme`; grilla `minmax(min(100%,320px),1fr)` para 360 px.
- **Archivos tocados:** `web/index.html`, `web/app.js`, `web/app.css`, `web/fixture.json` (rama `feature/plan-1-task-004`, commits `751b2dc` y `51038d8` — favicon inline)
- **Verificado por:** QA del orquestador con Playwright (2026-10-08, `?fixture` vía server estático): 5 tarjetas en orden waiting→blocked→busy→idle→done, HTML hostil inerte (sin `<script>`/`<img>` en el DOM de tarjetas, sin dialogs), título `(2) esperándote`, 360 px sin scroll horizontal, dark mode, filtros persistidos, copiar OK, consola sin errores JS. NO verificado: SSE contra el server real (llega con TASK-003) y los íconos emoji en un navegador real (en headless salen como cuadros). Nota: `?fixture` usa `fetch`, así que desde `file://` el navegador lo bloquea; hace falta cualquier server estático (ej. `python3 -m http.server -d web`) o el server de TASK-003. `web/embed.go` queda para TASK-003.

