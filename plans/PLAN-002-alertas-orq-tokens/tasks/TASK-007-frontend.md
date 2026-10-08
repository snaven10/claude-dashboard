# TASK-007 — Frontend: avisos, sin actividad, orq, tokens y línea de tiempo

- **Plan:** PLAN-002 — Alertas, workers de orq, tokens y línea de tiempo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-002-design.md` (no del código de TASK-006: van en paralelo)
- **Estado:** `pending`

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
- [ ] **Paso 1 — avisos (DD-12)**: botón "activar avisos"; notificar solo en la transición a `waiting`/`blocked`; nunca en el primer snapshot; on/off en `localStorage`; click en el aviso enfoca la pestaña.
- [ ] **Paso 2 — sin actividad**: badge "sin actividad hace X" cuando `stale`.
- [ ] **Paso 3 — orq**: en la tarjeta, `PLAN-1 · TASK-005 · specialist`; toggle "agrupar por plan" con contadores por grupo (persistido).
- [ ] **Paso 4 — tokens**: total legible (`1.2M`), detalle en `title`; sin precio en dólares (DD-11).
- [ ] **Paso 5 — línea de tiempo**: click en la tarjeta abre un panel con `/api/sessions/{id}/timeline` (o `fixture-timeline.json` con `?fixture`); cerrar con Esc; usable a 360 px.

## Criterios de aceptación
- [ ] Con `?fixture`: badges, agrupación, tokens y panel se ven bien en claro/oscuro, a 1280 y 360 px, sin scroll horizontal.
- [ ] Con el fixture el texto hostil sigue inerte también dentro del panel de línea de tiempo.
- [ ] La lógica de transición (qué sesiones notificar entre dos snapshots) es una función pura exportable y probada con `node --test` (sin dependencias).
- [ ] `node --check web/app.js` y los tests de node en verde, pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
