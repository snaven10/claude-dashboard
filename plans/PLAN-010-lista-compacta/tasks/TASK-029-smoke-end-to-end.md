# TASK-029 — Smoke end-to-end: capturas, conteo de filas y panel real

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** orquestador (con el usuario para el paso en el panel real)
- **Proyecto:** claude-dashboard (raíz del repo), rama `main` con TASK-022..TASK-028 mergeadas
- **Depende de:** TASK-022, TASK-023, TASK-024, TASK-025, TASK-026, TASK-027, TASK-028 (o su `skipped`)
- **Estado:** `pending`

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
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
