# TASK-027 — Motivo real de espera en bandeja, detalle y avisos

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-027` desde `main` (con TASK-026 mergeada)
- **Depende de:** TASK-026 (mismo archivo; lógicamente solo necesita TASK-025). TASK-023 no bloquea
  la implementación, pero su log es lo que confirma §2.1 del master al cerrar el plan.
- **Estado:** `pending`

---

## Objetivo
Ninguna tarjeta de la bandeja vuelve a decir "input needed": muestra la pregunta real si existe y,
si no, un motivo en castellano (permiso / diálogo / respuesta) con contexto (lo último que hizo
Claude o tu último pedido). Implementa DD-41 del master.

## Contexto verificado
- Causa (master §2.1): `renderInbox` (`web/app.js:1032-1040`) cae a `s.waitingFor` crudo de F1
  cuando `askView` (`app.js:103-104`) es `null`; `ask` es nulo cuando no hay transcript
  (`internal/transcript/transcript.go:137-138`) o cuando el turno actual no dejó texto ni
  `tool_use` abierto — un permiso pendiente no se escribe en el transcript
  (`transcript.go:240-242`).
- Valores de `waitingFor` vistos: `"input needed"`, `"permission prompt"`, `"dialog open"`; las
  background `blocked` pueden traer texto propio (en `web/fixture.json`:
  `"permission to run: rm -rf build/"`).
- Otros lugares que muestran `waitingFor`: el detalle (`app.js:1254`, línea `.waiting-for`) y el
  cuerpo del aviso de Windows (`app.js:695`: `s.waitingFor || lastAction.text || 'necesita tu atención'`).
- `askView` también se usa en el detalle para idle (`mode: 'small'`): no cambiar ese caso.
- `lastAction` (`{kind, text}`) y `lastPrompt` llegan en el snapshot; `oneLine`/`plainText` existen.

## Archivos
- **Modificar:** `web/app.js`, `web/app.test.js`, `web/app.css` (solo si el contexto necesita estilo)

## Pasos
- [ ] **Paso 1 — `waitingLabel(wf)` y `waitView(s)` + tests.** Según DD-41: con `ask` → igual que
  `askView` en modo `big`; sin `ask` → etiqueta por `waitingFor` (`permission prompt` → "pide
  permiso en la terminal", `dialog open` → "tiene un diálogo abierto en la terminal", `input needed`
  → "espera tu respuesta", otro → `plainText(wf)`, vacío → "necesita tu atención") y `context`
  ("lo último: ▸ " + `oneLine(lastAction.text)` si `lastAction`; si no "tu último pedido: " +
  `plainText(lastPrompt)` recortado a ~120; si no `''`). Tests: los tres valores conocidos, uno
  desconocido, `null`, con y sin `lastAction`/`lastPrompt`, `ask` presente gana siempre, texto
  hostil sigue siendo texto, **ningún resultado contiene la cadena `input needed`**.
- [ ] **Paso 2 — Bandeja.** La tarjeta usa `waitView`: etiqueta en negrita + texto (o contexto) en
  la línea de motivo; texto completo en `title`.
- [ ] **Paso 3 — Detalle.** Reemplazar la línea suelta `.waiting-for` por `waitView` cuando no hay
  `ask` (cuando hay `ask`, el bloque `ask` actual ya lo muestra; no duplicar).
- [ ] **Paso 4 — Aviso de Windows.** Cuerpo: proyecto + `waitView(s).text || waitView(s).label` +
  contexto si entra.
- [ ] **Paso 5 — Fixture.** Confirmar que `?fixture=lista` cubre `input needed` con `lastAction` y
  `permission prompt` con `lastPrompt` (los dejó TASK-022); en `web/fixture.json` no hace falta tocar nada.

## Criterios de aceptación
- [ ] `node --check web/app.js`, `node --test web/` en verde con los tests de `waitView`;
  `rg -n innerHTML web/app.js` vacío.
- [ ] `gofmt -l .`, `go vet ./...`, `go test -race ./...` en verde.
- [ ] Con `?fixture=lista` y con `?fixture`: ninguna tarjeta ni detalle muestra "input needed"
  (`document.body.innerText.includes('input needed') === false`); la sesión sin `ask` con
  `lastAction` muestra "espera tu respuesta · lo último: ▸ Bash: npm test".
- [ ] Medición estándar: la bandeja no crece de alto respecto a TASK-024 (motivo en una línea).

## Riesgos
- Si TASK-023 muestra en vivo que la causa dominante es `sin-transcript` en sesiones recién
  abiertas, `lastAction`/`lastPrompt` también estarán vacíos: el motivo traducido es todo lo que hay.
  Es correcto; no inventar texto.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
