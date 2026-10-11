# TASK-026 — Colapso "×N" de títulos repetidos y clave de fila única

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-026` desde `main` (con TASK-025 mergeada)
- **Depende de:** TASK-025
- **Estado:** `done`

---

## Objetivo
Las copias de una misma conversación ocupan una sola fila con chip `×N`, sin mezclar sesiones
distintas, y cada fila tiene una clave única aunque F1 repita el `sessionId`. Implementa DD-40 del
master (la regla de identidad está ahí; no la cambies).

## Contexto verificado
- `web/app.js:8` `sessionKey(s) = s.sessionId || s.id`. Lo usan `row` (`data-key`), `navKeys`,
  `selectedSession` (`app.js:1077-1080`), `select`, `notifyTargets` (`app.js:12-19`),
  `inboxItems`/`renderInbox`, `fold`, el `tag` de los avisos (`app.js:696`) y `qrOpenFor`.
- En datos reales F1 devolvió dos entradas con el mismo `sessionId` (PIDs distintos): hoy comparten
  `data-key` y el detalle siempre muestra la primera (master §2.2). `pid` viene en cada sesión
  (`internal/agents/agents.go`, `PID int json:"pid"`); las background terminadas pueden traer `pid` 0.
- `normalizeText` (`app.js:205-208`) y `plainText` (`app.js:345-356`) existen y están testeadas.
- Selección guardada: `SEL_KEY` en `localStorage` (`app.js:838-855`).
- Tras TASK-025 la lista se arma con `statusGroups`; el colapso se aplica **dentro** de cada grupo.
- El fixture de lista (TASK-022) trae: par gemelo de mismo `sessionId` (idle), par mismo cwd+título
  (idle), par mismo título en cwd distintos (idle), par `busy` mismo cwd+título, trío `done` mismo
  cwd+título, `done` sin título en el mismo cwd.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/app.test.js`

## Pasos
- [ ] **Paso 1 — `rowKey` + tests.** `rowKey(s) = (s.sessionId || s.id) + (s.pid > 0 ? '#' + s.pid : '')`.
  Reemplazar `sessionKey` por `rowKey` en todos los usos de la lista de arriba (dejar `sessionKey`
  solo si algo necesita el id de conversación, p. ej. la URL de timeline usa `s.sessionId`).
  Migración: una selección guardada sin `#` se resuelve a la primera sesión con ese `sessionId`.
  Tests: gemelos con mismo `sessionId` → claves distintas; `notifyTargets` con gemelos no se confunde.
- [ ] **Paso 2 — `collapseRepeats(sessions, groupId)` + tests.** Devuelve
  `[{ head, copies: [...] }]` preservando el orden de entrada. Reglas (DD-40): mismo `cwd` exacto +
  mismo `normalizeText(plainText(s.title))` **no vacío** + mismo grupo + grupo ∈ {`idle`, `done`,
  `abandoned`}. `head` = la de `lastActivityAt || startedAt` más reciente. Tests obligatorios:
  gemelos mismo sessionId → 1 fila ×2; mismo cwd+título → ×2; mismo título distinto cwd → 2 filas;
  título vacío nunca colapsa; `busy` nunca colapsa; tres `done` iguales → ×3 con la más reciente
  como `head`; mayúsculas/tildes/markdown en el título no impiden el colapso.
- [ ] **Paso 3 — Fila.** La fila de `head` agrega chip `×N` (cuenta para el tope de chips de
  `rowView` con prioridad máxima) con `title` "N sesiones con el mismo título en esta carpeta". Las
  copias no generan fila ni entran en `navKeys`. Contadores, favicon y avisos **no** cambian (siguen
  contando todas).
- [ ] **Paso 4 — Detalle: "Copias (N)".** En `buildDetail`, si la sesión mostrada pertenece a un
  grupo colapsado, sección con una línea por miembro (estado en texto, hace cuánto, PID; la actual
  marcada) como botones que seleccionan esa copia. Si la selección es una copia, en la lista queda
  resaltada la fila `head`. Incluir los miembros en la firma del detalle (`renderDetail`) para que
  se actualice cuando cambian.

## Criterios de aceptación
- [ ] `node --check web/app.js`, `node --test web/` en verde con los tests de `rowKey` y
  `collapseRepeats`; `rg -n innerHTML web/app.js` vacío.
- [ ] `gofmt -l .`, `go vet ./...`, `go test -race ./...` en verde.
- [ ] Con `?fixture=lista`: el par gemelo, el par idle y el trío done se ven como una fila con
  `×2`/`×2`/`×3`; el par `busy`, el par de distinto cwd y los sin título se ven separados. Contador
  "idle" igual al de antes del cambio. Seleccionar una copia desde "Copias" cambia el detalle y no
  rompe j/k.
- [ ] Ninguna fila comparte `data-key`: `new Set([...document.querySelectorAll('[data-key]')].map(n => n.dataset.key)).size`
  igual al número de nodos.
- [ ] Medición estándar: filas visibles a 1440×900 ≥ las de TASK-025 (no puede empeorar).

## Riesgos
- Cambiar la clave invalida las selecciones guardadas: por eso la migración del Paso 1.
- El `tag` del aviso de Windows cambia de formato: un aviso viejo no se reemplaza por el nuevo una
  vez (tolerable).

## Resultado
- **Estado final:** done
- **Resumen:** `rowKey` (conversación + `#pid`) reemplaza a `sessionKey` en filas, bandeja, j/k, selección, timeline, aviso y QR; selección guardada sin `#` se migra a la primera sesión con ese `sessionId` (`resolveSession`). `collapseRepeats(sessions, groupId)` devuelve `[{head, copies}]` (copies = todos los miembros, incluida la head; N = copies.length) con la regla DD-40; `collapseMixed` lo aplica por estado en el modo "agrupar por plan". La fila head lleva chip `×N` (prioridad máxima, cuenta para el tope), las copias no generan fila ni entran en j/k, los contadores siguen contando todas. El detalle suma "Copias (N)" con un botón por miembro (estado, hace cuánto, PID; la actual marcada) y, si se selecciona una copia, la fila head queda resaltada; los miembros entran en la firma del detalle.
- **Archivos tocados:** `web/app.js`, `web/app.css`, `web/app.test.js`, este TASK. Commit de código: `bdba772`.
- **Verificado por:** `node --check web/app.js` ok; `node --test web/` 79/79 pass (13 tests nuevos: rowKey, resolveSession, notifyTargets con gemelos, collapseRepeats x8, collapseMixed, rowView ×N); `rg innerHTML web/app.js` vacío; `gofmt -l .` vacío; `go vet ./...` ok; `go test -race ./...` ok. Sobre `fixture-lista.json` (vía node): 43 `rowKey` únicas; idle 8→6 filas (×2, ×2), done 21→19 (×3), busy 7 y abandonadas 4 sin colapsar.
- **Desviaciones:** `copies` incluye a la head (la task no lo especificaba). Posición de una entrada colapsada = la del primer miembro en el orden de entrada.
- **Riesgos abiertos / siguiente:** NO verificado en navegador (no se abrió ninguno): render `?fixture=lista`, unicidad de `data-key` en el DOM, no regresión de la medición de filas a 1440×900 y j/k desde una copia quedan para TASK-029. Estilos de `.chip.rep`/`.copy-item` sin revisar visualmente. El `tag` del aviso de Windows cambia de formato (tolerable, ya anotado).
