# PLAN-002 — Alertas, workers de orq, tokens y línea de tiempo

**Fecha:** 2026-10-08
**Fase:** Cerrado
**Diseño:** [`PLAN-002-design.md`](./PLAN-002-design.md)
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario tras cerrar PLAN-001 — features 1 a 5 de la propuesta (remota descartado: hoy no se usa)

## 1. Qué resuelve
El panel de PLAN-001 muestra el estado, pero solo si lo estás mirando, no distingue una sesión
trabajando de una colgada, no sabe qué worker de `orq` es cuál y no dice cuánto gasta cada sesión.

| # | Feature | Para qué |
|---|---------|----------|
| 1 | Notificación del navegador al pasar a `waiting`/`blocked` | Enterarte aunque estés en otra ventana |
| 2 | Workers de `orq`: plan, task, specialist; agrupar por plan | Ver un lote sin correr `orq status` |
| 3 | `busy` sin actividad hace N min → "sin actividad" | Distinguir trabajando de colgada |
| 4 | Tokens por sesión (input, output, cache) | Saber quién se come el presupuesto |
| 5 | Línea de tiempo al abrir una tarjeta (últimas 20 acciones) | Ver qué hizo sin `attach` |

## 2. Hallazgos que reorientan el plan
- **Tokens: cada mensaje del asistente ocupa varias líneas del `.jsonl` (una por bloque) y TODAS
  repiten el mismo `usage`.** Verificado hoy: 596 líneas = 276 `message.id`; 320 repetidas, 0 con
  `usage` distinto. Sumar por línea da más del doble → **deduplicar por `message.id`** (DD-9).
- Sumar tokens exige leer TODO el archivo, no la cola (DD-3 de PLAN-001 no alcanza) → acumulador
  incremental por offset (DD-9).
- `orq`: el `name` de F1 es exactamente el `bgname` del job (`orq-<key>-<jid>`); plan y task salen
  del `key` (`…:p1-task005@…`). `~/.orq/jobs.json` no tiene campos de plan propios (DD-10).
- No hay precios oficiales en el transcript: se muestran tokens y modelo, **no** un costo en dólares
  inventado (DD-11).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-006** | Backend: `stale`, `orq`, `tokens` en `Session` + `GET /api/sessions/{id}/timeline` | claude-dashboard-worker | sonnet | contrato del design | `done` |
| **TASK-007** | Frontend: avisos, badge sin actividad, datos y agrupación orq, tokens, panel de línea de tiempo | claude-dashboard-worker | sonnet | contrato del design (no código) | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-006**, **TASK-007** | Archivos disjuntos (`internal/`+`cmd/` vs `web/`); los une el contrato del design, como TASK-001/004 en PLAN-001 |

## 4. Fuera de alcance
- Sesiones de remota (no se usa hoy).
- Acciones desde el panel (stop, mensajes): sigue siendo solo lectura.
- Costo en dólares (DD-11).
- Persistir históricos de tokens entre reinicios del servicio.

## 5. Cierre
- **Cerrado:** 2026-10-08
- **Shipeó:** rama `main` (sin remoto). Merges `07e2829` TASK-006 y `57bf121` TASK-007; fixes de QA `28232d5` (embed) y `1eb8457` (poller llama a `Usage`); `d5eb269` (tokens por bloques) entró con TASK-006. Servicio reinstalado con `make install`.
- **Verificación:**
  - `go test -race` en 4 paquetes + 8 tests de node, en `main` tras el merge (código idéntico al de la rama de integración que pasó QA).
  - Contra datos reales: tokens de 26 sesiones cuadran con un conteo independiente por `message.id` único; la dedup por "último id" (DD-9) coincide con la de id único en 60 transcripts; RSS 24 MB; workers de orq con plan/task/specialist; timeline de 20 eventos; 0 errores de consola.
  - Bugs que la QA atrapó y los tests de los workers no: el poller nunca llamaba a `Usage` (0 sesiones con tokens), lectura entera en RAM de transcripts de hasta 112 MB, fixture de timeline fuera del embed.
  - Primer lote con los fixes de orq de `a26b57a` en `orquestar-sesiones`: los workers cerraron sus tasks en sus worktrees y `main` quedó limpio.
- **NO verificado:** notificación real de Windows (headless sin permiso; lógica cubierta por tests de node), `stale` con una sesión realmente colgada (cubierto por test), modo oscuro de lo nuevo.
- **Quedó afuera:** lo de §4. Sin tasks `skipped`.
- **Continúa en:** —
