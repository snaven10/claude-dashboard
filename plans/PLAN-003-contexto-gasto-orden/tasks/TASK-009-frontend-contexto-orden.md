# TASK-009 — Frontend: contexto, total por plan, terminadas viejas, búsqueda y ritmo

- **Plan:** PLAN-003 — Contexto, gasto por plan, orden y ritmo
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato de `PLAN-003-design.md` (no del código de TASK-008: van en paralelo)
- **Estado:** `done`

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
- [x] **Paso 1** — Contexto en la tarjeta ("contexto 412k · compactó 2×"; con `warn`, aviso visible).
- [x] **Paso 2** — Ritmo: badge "ritmo inusual" con detalle en `title` (output y llamadas en 5 min).
- [x] **Paso 3** — Total de tokens en la cabecera de cada grupo de plan (DD-16).
- [x] **Paso 4** — Terminadas viejas en sección colapsada con contador, toggle persistido (DD-15).
- [x] **Paso 5** — Búsqueda (DD-17): input, `/` enfoca, Esc limpia, sin tildes ni mayúsculas.

## Criterios de aceptación
- [x] `node --test`: formateo de contexto, suma por plan (con `tokens` null en alguna), corte de 24 h de "terminadas viejas", normalización de la búsqueda (tildes/mayúsculas).
- [ ] (NO verificado visualmente: sin navegador en el worker) Con `?fixture`: todo visible en claro y oscuro, 1280 y 360 px, sin scroll horizontal; el texto hostil inerte.
- [x] Ningún botón ni texto propone borrar sesiones.
- [x] `node --check web/app.js` y `node --test web/` en verde, pegados en `Verificado por`.

## Resultado
- **Estado final:** done (salvo verificación visual en navegador, pendiente)
- **Resumen:** Tarjeta con `context` ("contexto 412k · compactó 2×", con `warn` "⚠ contexto 850k, cerca de compactar") y badge "⚡ ritmo inusual" con detalle en `title`. Con agrupar por plan, la cabecera suma tokens (total y output; `title` aclara que las borradas con `claude rm` no cuentan). Terminadas con más de 24 h (`lastActivityAt` o `startedAt`) van a una sección colapsada con contador y toggle persistido en localStorage; sin acciones de borrado. Búsqueda `#f-search` por name/title/lastPrompt/project/orq plan-task, sin tildes ni mayúsculas, combinada con los filtros; `/` enfoca, Esc limpia (y cierra la línea de tiempo primero si está abierta). Con `?fixture` el "ahora" es `generatedAt` para que el corte de 24 h no caduque. Texto externo siempre con `textContent`; el fixture hostil quedó intacto.
- **Archivos tocados:** web/index.html, web/app.js, web/app.css, web/app.test.js, web/fixture.json (commit 9703be1)
- **Verificado por:** `node --check web/app.js` OK; `node --test web/` 15/15 pass; `gofmt -l .` vacío; `go vet ./...` OK; `go test -race ./...` OK. NO verificado: render con `?fixture` en claro/oscuro, 1280 y 360 px (no hay navegador acá); el CSS usa `min-width:0`, `overflow-wrap` y `flex-wrap` para evitar scroll horizontal, pero hay que mirarlo.
