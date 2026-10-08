# PLAN-003 — Contexto, gasto por plan, orden y ritmo

**Fecha:** 2026-10-08
**Fase:** Cerrado
**Diseño:** [`PLAN-003-design.md`](./PLAN-003-design.md)
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** mejoras 1-5 propuestas tras cerrar PLAN-002

## 1. Qué resuelve
| # | Mejora | Para qué |
|---|--------|----------|
| 1 | Contexto actual, compactaciones y aviso antes de compactar | Enterarte ANTES de que una sesión pierda detalle |
| 2 | Tokens totales por plan de `orq` | Saber cuánto costó un lote |
| 3 | Terminadas viejas agrupadas y colapsadas (**no se borra nada**) | Que 15 `done` de ayer no tapen lo de hoy |
| 4 | Búsqueda por texto | Encontrar una sesión entre 30 |
| 5 | Ritmo inusual de tokens | Ver una sesión que quema mucho más de lo normal |

## 2. Hallazgos que reorientan el plan
- **Ni F1 ni F2 dicen el tamaño de ventana.** Pero cada compactación deja una línea
  `{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger","preTokens","postTokens",…}}`.
  Medido hoy en 150 transcripts: 66 compactaciones; las automáticas saltan con `preTokens` entre
  **967k y 988k** (ventanas de 1M). Contexto máximo observado: opus-5-5 925k, sonnet-5-5 674k.
  → Se muestra contexto ABSOLUTO + compactaciones + aviso por umbral absoluto (DD-13), sin
  inventar un porcentaje sobre una ventana que no se conoce.
- **Ritmo normal medido** en 7.609 ventanas de 5 min con actividad: output p50 3,4k · p95 12,9k ·
  **p99 19,3k** · máx 52,5k; llamadas p99 23. → umbral por defecto 20k output o 25 llamadas en 5 min (DD-14).
- `claude rm` **borra** la sesión y su worktree (y con flags, commits sin pushear): el panel NO lo
  sugiere. "Terminadas viejas" es solo presentación (DD-15).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-008** | Backend: `context` y `rate` por sesión desde el transcript | claude-dashboard-worker | sonnet | contrato del design | `done` |
| **TASK-009** | Frontend: contexto, total por plan, terminadas viejas, búsqueda, ritmo | claude-dashboard-worker | sonnet | contrato del design (no código) | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-008**, **TASK-009** | Archivos disjuntos (`internal/`+`cmd/` vs `web/`), unidos por el contrato del design |

## 4. Fuera de alcance
- Borrar o archivar sesiones de verdad (DD-15).
- Costo en dólares (DD-11 de PLAN-002 sigue).
- Historial persistente entre reinicios (el ritmo se recalcula del transcript, no hace falta).

## 5. Cierre
- **Cerrado:** 2026-10-08
- **Shipeó:** rama `main` (sin remoto). Merges `a8e94a4` TASK-008 y `d51f7dd` TASK-009. Servicio reinstalado con `make install`.
- **Verificación:**
  - `go test -race` (4 paquetes) y 15 tests de node en `main`; el código de `main` es idéntico al de la rama de integración que pasó QA.
  - Contra 30 sesiones reales: `context` cuadra en 26/26 con un conteo independiente; `rate` exacto en las sesiones quietas; 1 sesión real con `warn`.
  - Navegador: búsqueda, totales por plan, contexto y ritmo con datos reales; terminadas viejas, aviso de contexto y ritmo inusual con el fixture en oscuro a 360 px. Nada en la UI propone borrar sesiones (DD-15).
  - **Primer lote sin bugs en la QA**: el worker ya corre sus tests y los criterios pedían tests a nivel server (lección de PLAN-002).
- **NO verificado:** la sección de terminadas viejas con datos reales (hoy no hay `done` de más de 24 h); "ritmo inusual" con una sesión real por encima del p99; modo claro a 360 px de lo nuevo.
- **Quedó afuera:** lo de §4. Sin tasks `skipped`.
- **Continúa en:** —
