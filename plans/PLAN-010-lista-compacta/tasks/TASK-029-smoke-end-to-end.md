# TASK-029 — Smoke end-to-end: capturas, conteo de filas y panel real

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** orquestador (con el usuario para el paso en el panel real)
- **Proyecto:** claude-dashboard (raíz del repo), rama `main` con TASK-022..TASK-028 mergeadas
- **Depende de:** TASK-022, TASK-023, TASK-024, TASK-025, TASK-026, TASK-027, TASK-028 (o su `skipped`)
- **Estado:** `done`

---

## Objetivo
Comprobar el flujo completo, no las piezas: con el fixture y con el panel real, todas las metas del
master §1 se cumplen y nada de lo que había que mantener (§4) se rompió.

## Contexto verificado
- "Medición estándar" en el master §3 (servidor estático en loopback, `?fixture=lista`, 1440×900
  y 390×844, claro/oscuro, script de conteo de filas).
- Línea base anotada en el `## Resultado` de TASK-022.

## Archivos
- Ninguno del código. Solo se llena este `## Resultado` y el `## 6. Cierre` del master.

## Pasos
- [ ] **Paso 1 — Tests de la rama integrada.** `node --check web/app.js`, `node --test web/`,
  `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `rg -n innerHTML web/app.js`.
- [ ] **Paso 2 — Fixture, escritorio.** `?fixture=lista` a 1440×900, claro y oscuro: contar filas
  visibles (meta ≥ 25), alto de fila ≤ 28 px, `Terminadas (21)` y `Abandonadas (4)` visibles,
  ninguna sesión en bandeja y lista a la vez, `×N` donde corresponde (DD-40) y separadas donde no,
  ningún "input needed", timeline con ≥ 10 eventos, sin `▯`, sin scroll horizontal. Capturas fuera
  del repo.
- [ ] **Paso 3 — Fixture, teléfono.** 390×844: filas de un renglón, sin scroll horizontal, fila →
  detalle → "← volver", tarjeta de bandeja → detalle.
- [ ] **Paso 4 — Teclado y filtros.** j/k (bandeja → filas), Enter, 1/2/3 (con `1` el mensaje de
  DD-39), `/` + Esc, filtro de proyecto (desaparece la etiqueta de proyecto de las filas), ocultar
  done/idle, agrupar por plan (avance y "sin mergear" del Plan 42).
- [ ] **Paso 5 — `?fixture` original.** Sigue cargando y se ve coherente.
- [ ] **Paso 6 — Panel real (con el usuario).** Reinstalar (`make install`, con OK del usuario) y
  mirar el panel real a 1440×900: filas visibles, terminadas/abandonadas con contador, repetidos
  colapsados sin mezclar sesiones distintas. Revisar el log del servicio buscando `waiting sin ask`
  (TASK-023) para confirmar la causa de §2.1 cuando aparezca una sesión esperando sin pregunta.
  **No** copiar nada del panel real a archivos del repo.

## Criterios de aceptación
- [ ] Todos los números de los Pasos 2-3 anotados junto a la línea base de TASK-022.
- [ ] Cada punto de §4 del master ("Se mantiene") marcado como probado o explícitamente "no probado".
- [ ] `## 6. Cierre` del master lleno, incluyendo qué **no** se verificó.

## Resultado
- **Estado final:** done — Pasos 1-6 (2026-10-11; el Paso 6 con OK del usuario para `make install`).
- **Resumen:** el flujo integrado en `main` cumple las metas con el fixture. Un bug encontrado y arreglado en el smoke: con el filtro `1` (esperándote) el aviso "Las N sesiones que te esperan están arriba" no salía si había abandonadas en la lista (`listed` las contaba); ahora cuenta solo las filas vivas (`web/app.js`, `renderList`).
- **Verificado por:**
  - Paso 1: `node --check` ok; `node --test web/` 82/82; `gofmt -l .` vacío; `go vet` ok; `go test -race ./...` ok; `innerHTML` 0.
  - Paso 2 (`?fixture=lista`, Chromium headless, `localStorage` limpio, 1440×900):

    | Medida | Línea base (TASK-022) | Ahora |
    |---|---|---|
    | Elementos visibles sin scroll | 12 de 24 | **16 de 16 activos** (3 tarjetas + 13 filas: 7 trabajando + 6 idle tras ×N) |
    | Alto de fila | 48 px | **28 px** |
    | Eventos de timeline visibles | 3-4 | **11 de 20** |
    | Duplicados bandeja/lista | sí | **0** |
    | "input needed" / "permission prompt" crudos | sí | **no** |
    | `▯` | sí | **no** (SVG) |
    | `scrollWidth` | 1425 | 1425 (sin scroll horizontal) |

    `Terminadas (21)` y `Abandonadas (4)` visibles plegadas y como botones en el encabezado; `×2` en los dos pares idle; el par busy con el mismo título queda separado (DD-40); 13 `data-key` únicas. Temas claro y oscuro mirados en captura.
  - La meta "≥ 25 visibles" no aplica tal cual: con terminadas plegadas (DD-38) el fixture solo tiene 16 entradas activas, y entran todas.
  - Paso 3 (390×844): 3 tarjetas + 8 filas visibles (base: 6), fila 28 px, `scrollWidth` 375; fila → detalle → "← volver" ok; tarjeta → detalle ok.
  - Paso 4: `j` recorre bandeja → filas; `1` filtra y muestra el aviso (tras el fix); `2` deja 7 trabajando; `/` enfoca la búsqueda, "docs" deja 2 filas, `Esc` limpia; filtro de proyecto oculta la etiqueta de proyecto; agrupar por plan muestra `PLAN-42 · 14/28 done · 1 blocked · 2 in_progress` con barra de avance.
  - Paso 5: `?fixture` original carga (6 tarjetas, 10 filas), sin `▯` ni texto crudo.
  - Paso 6 (panel real tras `make install`, 1440×900, tema oscuro): 1 tarjeta en la bandeja + 16 filas (6 trabajando, 12 idle con dos `×2`), **todas visibles sin scroll**; `Terminadas (34)` y `Abandonadas (4)` plegadas; 0 duplicados bandeja/lista; 16 claves únicas; fila 28 px; sin textos crudos de `waitingFor`; sin `▯`; sin scroll horizontal. La tarjeta muestra `pide permiso: Bash: …` (con `ask`). Log del servicio: `waiting sin ask … waitingFor=- motivo=sin-transcript` para 4 sesiones (las abandonadas): confirma el camino "sin transcript" de §2.1. Nada del panel real se copió al repo.
- **No verificado:** `Enter`/flechas con teclado físico (solo eventos sintéticos), lector de pantalla, `prefers-reduced-motion`, conservación de selección/scroll con SSE (el panel real actualizó sin errores, pero no se midió la selección), un caso real de `waitingFor` = `permission prompt` sin `ask`.
- **Observaciones para después (fuera de alcance):** en la bandeja de 3 columnas el título de una tarjeta con rama queda muy recortado ("Revisar mig…"); el encabezado de plan muestra "· 8 ·" (cantidad de sesiones) sin etiqueta, comportamiento anterior a este plan.
