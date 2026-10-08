# TASK-009 — Frontend: contexto, total por plan, terminadas viejas, búsqueda y ritmo

- **Plan:** PLAN-003 — Contexto, gasto por plan, orden y ritmo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-003-design.md` (no del código de TASK-008: van en paralelo)
- **Estado:** `pending`

---

## Objetivo
La UI muestra `context` y `rate`, suma tokens por plan, colapsa las terminadas viejas y permite buscar.

## Contexto verificado
- Leé `PLAN-003-design.md` (DD-13, DD-15, DD-16, DD-17). Frontend vanilla en `web/` (sin framework, sin build, sin CDN). Lógica pura exportada para `node --test` como en PLAN-002 (`web/app.test.js`).
- Todo texto externo con `textContent`. El fixture trae texto hostil a propósito: no lo saques.
- **DD-15: no se borra nada ni se sugiere `claude rm`** (borra sesión y worktree).

## Archivos
- **Modificar:** `web/index.html`, `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json` (agregar `context`/`rate`, sesiones `done` viejas, una con `warn`, una con `rate.high`, casos `null`)

## Pasos
- [ ] **Paso 1** — Contexto en la tarjeta ("contexto 412k · compactó 2×"; con `warn`, aviso visible).
- [ ] **Paso 2** — Ritmo: badge "ritmo inusual" con detalle en `title` (output y llamadas en 5 min).
- [ ] **Paso 3** — Total de tokens en la cabecera de cada grupo de plan (DD-16).
- [ ] **Paso 4** — Terminadas viejas en sección colapsada con contador, toggle persistido (DD-15).
- [ ] **Paso 5** — Búsqueda (DD-17): input, `/` enfoca, Esc limpia, sin tildes ni mayúsculas.

## Criterios de aceptación
- [ ] `node --test`: formateo de contexto, suma por plan (con `tokens` null en alguna), corte de 24 h de "terminadas viejas", normalización de la búsqueda (tildes/mayúsculas).
- [ ] Con `?fixture`: todo visible en claro y oscuro, 1280 y 360 px, sin scroll horizontal; el texto hostil inerte.
- [ ] Ningún botón ni texto propone borrar sesiones.
- [ ] `node --check web/app.js` y `node --test web/` en verde, pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
