# TASK-019 — Frontend: datos nuevos en la bandeja

- **Plan:** PLAN-008 — Datos nuevos por sesión
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-017 (PLAN-007) mergeada en `main`; contrato §4
- **Estado:** `pending`

---

## Objetivo
Mostrar en filas y detalle de la bandeja los campos de §4 según DD-36 y la maqueta (`../../PLAN-007-bandeja/maqueta.html`).

## Contexto verificado
- Leé el master entero (§4, DD-36) y la maqueta: chips de la fila, bloque "Mientras no estabas", stats "turno actual" y "líneas", sección "Links".
- La bandeja ya está en `web/app.js` (TASK-017): fila compacta y panel de detalle; funciones puras exportadas y testeadas en `web/app.test.js`.
- URLs de PR son de GitLab interno: validá en el cliente solo `https://` (sin espacios ni comillas) antes de usarlas en `href`; artifacts solo `https://claude.ai/`.
- **Solo `textContent`/`createElement`; prohibido `innerHTML`.**
- `web/fixture.json`: agregá a varias sesiones los 7 campos con datos sintéticos (incluí un PR con URL hostil `javascript:alert(1)` y un `away.text` hostil) para probar que no se usan.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json`

## Pasos
- [ ] **Paso 1** — Lógica pura + tests: chips de la fila, minutos de turno, validación de URLs, línea secundaria con `away`.
- [ ] **Paso 2** — Fila y detalle según DD-36.
- [ ] **Paso 3** — Búsqueda por PR y repo.

## Criterios de aceptación
- [ ] `node --test web/`: chips (subagentes solo activos > 0, turno solo busy ≥ 15 min, permisos ocultos en `default`), URL hostil descartada, `away` como línea secundaria solo fuera de `busy`.
- [ ] `grep -n innerHTML web/app.js` vacío.
- [ ] Con `?fixture`: claro/oscuro, 1280 y 360 px sin scroll horizontal; hostil inerte. (Si no podés abrir navegador, decilo.)
- [ ] `node --check web/app.js`, `node --test web/`, `go test ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
