# PLAN-010 — Lista compacta: todas las sesiones en una pantalla y el motivo real de espera

**Fecha:** 2026-10-11
**Fase:** 2 (Ejecución) — aprobado por el usuario 2026-10-11
**Fase anterior:** [PLAN-007](../PLAN-007-bandeja/PLAN-007-bandeja.md) (bandeja) y [PLAN-008](../PLAN-008-datos-de-sesion/PLAN-008-datos-de-sesion.md) (datos de sesión)
**Proyectos:** claude-dashboard (raíz del repo)
**Origen:** observación del orquestador en el panel real a 1440×900 (18 sesiones vivas + 21 terminadas + 4 abandonadas) y queja del usuario: "no puedo ver todas las sesiones, se pierde información"

## 1. Qué resuelve

| # | Problema observado | Cambio | Decisión |
|---|---|---|---|
| 1 | Cada fila mide ~68 px (título, proyecto/resumen, chips en 3 líneas): entran ~8 sesiones | Fila de **una línea** (~26-28 px) | DD-37 |
| 2 | Terminadas (21) y abandonadas (4) quedan en dos líneas plegadas al fondo, invisibles | **Grupos por estado con encabezado y contador**; terminadas y abandonadas también como contador en el encabezado de la página | DD-38 |
| 3 | `.split` es `5fr 6fr`: el detalle es más ancho que la lista y los resúmenes se cortan | Lista ~60 % (`3fr 2fr`), detalle sticky | DD-37 |
| 4a | El proyecto se repite en cada fila | Proyecto solo con el filtro en "Todos" y como etiqueta corta | DD-37 |
| 4b | La sesión que espera sale en la bandeja **y** en la lista | Se **quita de la lista**; la bandeja entra en la navegación j/k | DD-39 |
| 4c | Títulos repetidos (la misma conversación retomada aparece ×2) | Colapsar en una fila con "×N"; las copias se ven en el detalle | DD-40 |
| 5 | La bandeja muestra el genérico "input needed" | Motivo traducido + contexto (última acción o último pedido); diagnóstico en el log | DD-41 |
| 6 | La línea de tiempo del detalle tiene `max-height: 220px`: se ven 3-4 eventos | Timeline alta (usa el alto libre del detalle) | DD-37 |
| 7 | Íconos que se ven como `▯` en el Chromium headless del orquestador | Reemplazo por SVG inline / caracteres seguros (decisión del usuario: se hace igual, por robustez) | DD-42 |

Meta medible: **≥ 25 sesiones visibles sin scroll a 1440×900** con el fixture de lista (§3, "Medición estándar").

## 2. Hallazgos que reorientan el plan (verificados 2026-10-11 contra el código y el panel local)

### 2.1 Por qué la bandeja dice "input needed"
Cadena exacta:
1. `web/app.js:1032-1040` (`renderInbox`): si `askView(s)` es `null`, la tarjeta pinta
   `s.waitingFor || 'necesita tu atención'`. `waitingFor` es el texto **crudo de F1**
   (`claude agents --json`), en inglés y genérico.
2. `web/app.js:103-104` (`askView`): devuelve `null` si la sesión no trae `ask`.
3. `internal/server/poller.go:236-238` (`applyDetail`): `ask` solo existe si el transcript dio un
   `Detail.Ask`. `poller.go:56-58`: si `Summary` falla, no hay detalle.
4. `internal/transcript/transcript.go:137-138` (`Summary`): transcript no encontrado → `Detail{}`
   vacío, sin error (por diseño, DD-1). → `ask` nulo.
5. `internal/transcript/transcript.go:240-242` (comentario de `askScan`, QA PLAN-005): **un permiso
   pendiente NO queda escrito en el transcript.** `askScan` solo mira lo posterior al último prompt
   real (`transcript.go:372`, `ask.done = true`) y toma un `text` (`:304-305`) o un `tool_use` sin
   `tool_result`. Si en el turno actual Claude solo llamó herramientas (todas contestadas) y no
   escribió texto, y ahora espera un permiso o un diálogo, `askScan.result()` es `nil`. → `ask` nulo.

Evidencia adicional de datos locales (solo estructura, nada copiado): los archivos F2
(`~/.claude/sessions/<pid>.json`) traen **tres** valores distintos de `waitingFor`:
`"permission prompt"`, `"dialog open"` e `"input needed"`; varios de esos casos **no tienen
transcript** (sesión recién abierta en un diálogo, p. ej. confiar en la carpeta). O sea: el dato
para decir algo mejor que "input needed" **ya llega** (`waitingFor` distingue permiso / diálogo /
respuesta, y `lastAction` / `lastPrompt` dan contexto); el frontend lo tira.

**No verificado:** cuál de las dos ramas (4 o 5) produjo el caso puntual que vio el orquestador
(hoy no hay ninguna sesión interactiva esperando para inspeccionar). TASK-023 agrega un log que lo
dice en vivo; TASK-027 arregla la presentación para ambas ramas.

### 2.2 Colisión de claves de fila (bug latente)
`sessionKey(s) = s.sessionId || s.id` (`web/app.js:8`). En el panel real hay un par de entradas de
F1 con el **mismo `sessionId`** (misma conversación abierta en dos procesos, ambos `idle`): las dos
filas comparten `data-key`, las dos quedan `aria-selected` y el detalle siempre muestra la primera.
También afecta `notifyTargets`. Se arregla en TASK-026 junto con el colapso "×N".

### 2.3 Repetidos en datos reales (agregado, sin nombres)
43 sesiones: 4 grupos con mismo `cwd` + mismo título. Uno es el par de mismo `sessionId`; otro es un
par `busy` con `sessionId` distintos (dos sesiones trabajando de verdad: **no** se deben esconder);
dos grupos son de sesiones **sin título** (6 y 2 sesiones, `title` vacío): el título vacío **nunca**
puede ser identidad. De ahí la regla de DD-40.

### 2.4 Íconos (hallazgo a verificar en el navegador del usuario)
Caracteres en uso (`web/app.js`, `web/index.html`): `📊` historial, `📡` remoto, `🔢` tokens, `📱`
teléfono/QR, `⏳ ⚙ 💤 ✔` en la píldora de estado (`STATUS_ICON`, `app.js:478`), `⚡` ritmo, `⚠`
alertas, `⎇` rama, y `✕ ▾ ▸ ✓ “ −` en controles.
- `📊 📡 🔢 📱 ⏳ 💤 ⚡` tienen presentación emoji por defecto: sin fuente de emoji de color (típico de
  Chromium headless en Linux/WSL sin `fonts-noto-color-emoji`) salen como `▯`. Coincide con lo que vio
  el orquestador ("remoto", tokens, "historial", QR).
- `⎇` (U+2387, Misc Technical) falta en muchas fuentes de sistema aunque no sea emoji.
- `✕ ▾ ▸ ✓ “ − ⚠ ⚙ ✔` están en las fuentes de sistema comunes.
- Si el usuario abre el panel desde un navegador de Windows (Segoe UI Emoji), lo más probable es que
  **se vean bien**. Por eso TASK-028 arranca con un paso de verificación y puede terminar `skipped`.

### 2.5 Medición de partida
Con el layout de PLAN-007: fila de 2-3 líneas (~68 px con chips), encabezado de 3 renglones, `.split
5fr 6fr`, `.tl-wrap .events { max-height: 220px }` (`web/app.css:275`), breakpoint de teléfono
`max-width: 760px` (`web/app.css:329`). El `web/fixture.json` actual tiene 20 sesiones: no alcanza
para medir 25+ filas → TASK-022 crea un fixture de lista con el escenario real.

## 3. Tasks y orden

| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-022** | Fixture de lista (`?fixture=lista`, 43 sesiones sintéticas) + timeline de 20 eventos + medición de partida | claude-dashboard-worker | sonnet | — | `pending` |
| **TASK-023** | Backend: log de diagnóstico cuando una sesión espera sin `ask` (motivo: sin transcript / sin texto en el turno) | claude-dashboard-worker | sonnet | — | `pending` |
| **TASK-024** | Fila de una línea, lista 60 %, encabezado en 2 renglones, timeline alta | claude-dashboard-worker | sonnet | TASK-022 | `pending` |
| **TASK-025** | Grupos por estado con contador; terminadas/abandonadas en el encabezado; la que espera sale de la lista y la bandeja entra en j/k | claude-dashboard-worker | sonnet | TASK-024 | `pending` |
| **TASK-026** | Colapso "×N" de repetidos + clave de fila única + sección "Copias" en el detalle | claude-dashboard-worker | sonnet | TASK-025 | `pending` |
| **TASK-027** | Motivo real de espera (`waitView`) en bandeja, detalle y avisos | claude-dashboard-worker | sonnet | TASK-026 (mismo archivo; lógicamente solo TASK-025). TASK-023 confirma §2.1 al cerrar | `pending` |
| **TASK-028** | Íconos: verificar en el navegador del usuario y, si hace falta, SVG inline / caracteres seguros | claude-dashboard-worker | sonnet | TASK-027 | `pending` |
| **TASK-029** | Smoke end-to-end con capturas y conteo de filas (claro/oscuro, 1440×900 y teléfono) | orquestador | — | TASK-022..TASK-028 | `pending` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-022**, **TASK-023** | En paralelo: TASK-022 toca `web/fixture-lista.json`, `web/fixture-timeline.json`, `web/embed.go` y el cargador de fixture en `web/app.js`; TASK-023 solo `internal/server/` (y quizás `internal/transcript/`). No se pisan |
| 2 | **TASK-024** | Requiere TASK-022 (mide con el fixture). Puede correr mientras TASK-023 sigue |
| 3 | **TASK-025** | Secuencial tras TASK-024 |
| 4 | **TASK-026** | Secuencial tras TASK-025 |
| 5 | **TASK-027** | Secuencial tras TASK-026 (mismo archivo) |
| 6 | **TASK-028** | Secuencial tras TASK-027 |
| 7 | **TASK-029** | Al final, sobre la rama integrada |

**Archivos compartidos — por qué el frontend va en serie:** TASK-022, 024, 025, 026, 027 y 028 tocan
`web/app.js`; 024, 025, 026 y 028 tocan `web/app.css`; 024, 025 y 028 tocan `web/index.html`;
025, 026 y 027 tocan `web/app.test.js`. Correrlas en paralelo garantiza conflictos en
`render`/`renderList`/`renderInbox`. TASK-026 y TASK-027 son lógicamente independientes entre sí
pero comparten `renderInbox`/`buildDetail`: se serializan en el orden de la tabla.
**Único paralelismo real:** TASK-023 (Go) con cualquiera del frontend.

Ramas: una por task, `feature/plan-10-task-0XX` desde `main`; se mergea en orden antes de abrir la
siguiente del frontend.

### Medición estándar (la usan todas las tasks de UI y TASK-029)
1. Servir `web/` estático en loopback: `python3 -m http.server 8799 --bind 127.0.0.1 -d web`
   (herramienta de desarrollo, no dependencia del repo).
2. Abrir `http://127.0.0.1:8799/index.html?fixture=lista` con `localStorage` limpio (filtros por
   defecto, sin agrupar por plan).
3. Viewport **1440×900** y **390×844** (teléfono), `prefers-color-scheme` claro y oscuro.
4. Contar filas de sesión **completamente** visibles sin scroll:
   ```js
   [...document.querySelectorAll('#rows .row, #inbox-items .ask-card')]
     .filter(r => { const b = r.getBoundingClientRect(); return b.top >= 0 && b.bottom <= innerHeight; }).length
   ```
5. Sin scroll horizontal: `document.documentElement.scrollWidth <= innerWidth`.
6. Captura PNG de cada combinación. **Las capturas no se commitean** (pueden ir a `/tmp`).

Si el worker no puede abrir un navegador, lo declara en `Verificado por` y lo mide el orquestador
en TASK-029.

### Decisiones

**DD-37 — Fila de una línea y lista ancha.** Cada fila es un solo renglón: punto de estado · título
(elipsis) · etiqueta de proyecto (solo si el filtro de proyecto es "Todos"; muted, máx. ~14ch) ·
hasta 2 chips (los de mayor prioridad: errores > contexto > ritmo > sin actividad > turno > PR >
subagentes > líneas > permisos) + chip "+N" con el resto en `title` · hora relativa a la derecha.
Sin texto de estado en la fila (el punto + el encabezado de grupo lo dicen; el estado sigue en el
`aria-label` de la fila: color + texto, no solo color). El resumen (`away`, `ask` corto, rama)
pasa al `title` de la fila y completo al detalle. Se quita la mini barra de contexto de la fila
(el chip de contexto ya avisa cuando importa). Altura objetivo ≤ 28 px. `.split` pasa a
`minmax(0, 3fr) minmax(0, 2fr)`; el detalle sigue sticky. El encabezado de la página baja a dos
renglones (título + conexión + contadores; filtros + ayuda de teclado). La bandeja usa tarjetas de
dos renglones (título · proyecto · hora; motivo en una línea). La línea de tiempo deja el tope fijo
de 220 px: crece con el alto libre del detalle (`max-height: none` dentro del detalle que ya
scrollea, o `min(60vh, 640px)` si el scroll anidado resulta incómodo). Teléfono (≤ 760 px): misma
fila de una línea; `body.phone-detail` sin cambios.

**DD-38 — Grupos por estado con contador.** Sin "agrupar por plan", la lista se divide en
encabezados `Trabajando (N)`, `Idle (N)`, `Terminadas (N)` y `Abandonadas (N)`. Terminadas y
abandonadas siguen plegadas por defecto (recordado en `localStorage` como hoy), pero su encabezado
es una fila visible con contador, no una línea de texto perdida. En el encabezado de la página se
suman `N terminadas` y `N abandonadas` como botones discretos que despliegan y llevan a su grupo
(no son filtros 1/2/3; el atajo de teclado no cambia). Con "agrupar por plan" se mantienen los
encabezados de plan (avance, sin mergear, tokens) y dentro de cada plan el orden por estado; las
terminadas viejas y las abandonadas siguen al final como grupos plegables con contador. Los
encabezados van en `role="group"` con `aria-label` dentro del `listbox` (las filas siguen siendo
`option`).

**DD-39 — La sesión que espera no se duplica: sale de la lista** (confirmado por el usuario 2026-10-11). Las `waiting`/`blocked` no
abandonadas viven **solo** en la bandeja "Te esperan". Justificación: (a) la bandeja ya muestra más
que la fila (motivo, acciones, QR), así que la fila no aporta nada; (b) cada fila que se quita es
una sesión más que entra en pantalla; (c) marcarla ("↑ en bandeja") ocupa el mismo renglón que se
quiere liberar. Para no perder nada: las tarjetas de la bandeja entran **primero** en el orden de
j/k (`navKeys`), se marcan seleccionadas como las filas (`aria-selected` + borde de acento) y
`Enter` en teléfono abre su detalle. Con el filtro de estado "esperándote" (tecla `1`) la lista
muestra "Las N sesiones que te esperan están arriba, en Te esperan". Las abandonadas siguen en su
grupo de la lista (no están en la bandeja). La bandeja sigue ignorando los filtros, como hoy.

**DD-40 — Repetidos "×N": regla de identidad** (confirmada por el usuario 2026-10-11). Dos sesiones se colapsan en una fila si y solo si:
1. mismo `cwd` exacto, **y**
2. mismo título **no vacío** tomado de `s.title` (el título del transcript; nunca el respaldo
   `name`/`id`), comparado con `normalizeText(plainText(title))`, **y**
3. caen en el mismo grupo de estado, **y** ese grupo es `idle`, `done` o abandonadas.
   `busy` y `waiting` **nunca** se colapsan: son sesiones distintas trabajando o pidiendo algo
   (§2.3, par `busy` real).

La fila visible es la de actividad más reciente; muestra un chip `×N` con `title` "N sesiones con
el mismo título en esta carpeta". Las demás no ocupan fila ni entran en j/k, pero **siguen contando**
en los contadores, el favicon y los avisos. El detalle de la fila visible agrega una sección
"Copias (N)" con cada una (estado, hace cuánto, PID), clicable para seleccionarla (el detalle
cambia a esa copia y la fila representante queda resaltada). Una fila es un `<button>`: el "×N" no
puede ser otro botón adentro; por eso la expansión vive en el detalle.
**Clave de fila única:** `rowKey(s) = (s.sessionId || s.id) + (s.pid > 0 ? '#' + s.pid : '')`.
Una selección guardada con la clave vieja (sin `#pid`) se resuelve a la primera fila con ese
`sessionId`. `notifyTargets` pasa a usar `rowKey`.

**DD-41 — Motivo real de espera.** Función pura `waitView(s)` → `{ label, text, context }`:
- con `ask`: como hoy (`pregunta:` / `pide permiso:` / `último mensaje:` + texto + "+N preguntas").
- sin `ask`, por `waitingFor` de F1: `"permission prompt"` → "pide permiso en la terminal";
  `"dialog open"` → "tiene un diálogo abierto en la terminal"; `"input needed"` → "espera tu
  respuesta"; otro valor → el texto tal cual (como texto, con `plainText`); sin nada → "necesita tu
  atención".
- `context` (solo sin `ask`): "lo último: ▸ {lastAction}" si hay `lastAction`; si no, "tu último
  pedido: {lastPrompt}" recortado; si no, nada.
Se usa en la tarjeta de la bandeja, en el detalle (reemplaza la línea suelta de `waitingFor`,
`app.js:1254`) y en el cuerpo del aviso de Windows (`app.js:695`). El backend **no** cambia el
contrato: el dato ya llega. TASK-023 solo agrega un log `waiting sin ask: <sessionId corto>
motivo=sin-transcript|sin-texto-en-turno` (sin texto del usuario) para confirmar §2.1 en vivo.

**DD-42 — Íconos sin depender de emoji de color.** El usuario decidió (2026-10-11) hacerlo igual,
por robustez: helper `icon(name)` que arma SVG con
`document.createElementNS` (sin `innerHTML`, `aria-hidden="true"`, `fill="currentColor"`, 1em) para
historial, remoto, tokens, teléfono y rama; la píldora de estado pierde el ícono (el texto basta);
`⚡`/`⚠` pasan a la variante de texto (`︎`) o a SVG. En `index.html` el SVG del botón de
historial va inline en el HTML estático. Los caracteres de controles (`✕ ▾ ▸ ✓ “ −`) se quedan.

## 4. Restricciones duras (valen para todas las tasks)
- **Solo lectura** sobre las sesiones: el panel no escribe en `~/.claude/` ni actúa sobre sesiones.
- Escucha en **127.0.0.1** (DD-7 intacto).
- **Sin costos en dólares**: nada nuevo consulta APIs pagas; no se muestra costo (DD-11).
- **Go stdlib + JS vanilla**, cero dependencias, cero build de frontend, sin CDN.
- Texto de sesiones solo con `textContent`/`createElement`. `grep -n innerHTML web/app.js` vacío.
- **Fixtures sintéticos.** El repo es público: ningún archivo (plan, fixture, test, comentario,
  commit) puede contener nombres de clientes, repos de trabajo, URLs internas, IPs ni rutas del HOME
  del usuario. Ejemplos genéricos: `api-backend`, `web-app`, `Plan 42`, `/home/user/...`.
- Cada task declara verificación concreta: `node --test web/` y/o `go test`, más la "Medición
  estándar" donde haya UI.
- Se mantiene: atajos (`j`/`k`/flechas, `Enter`, `1`/`2`/`3`, `/`, `Esc`), vista teléfono (≤ 760 px,
  `body.phone-detail`, "← volver"), filtros (proyecto, ocultar done/idle, estado, búsqueda), agrupar
  por plan, temas claro/oscuro (`prefers-color-scheme`), accesibilidad (estado = color + texto,
  foco visible, `prefers-reduced-motion`), SSE con conservación de selección/foco/scroll, `?fixture`
  original.

## 5. Fuera de alcance
- Cambiar el contrato de `GET /api/sessions` o `docs/API.md` (DD-41 no lo necesita).
- Borrar, archivar o "fusionar" sesiones en el backend: el colapso "×N" es solo presentación.
- Selector de tema manual (sigue `prefers-color-scheme`).
- Virtualizar la lista (con < 100 sesiones no hace falta).
- Nuevos atajos de teclado.

## 6. Cierre
<!-- SE LLENA AL CERRAR EL PLAN -->
- **Cerrado:**
- **Shipeó:**
- **Verificación:**
- **Quedó afuera:**
- **Continúa en:**
