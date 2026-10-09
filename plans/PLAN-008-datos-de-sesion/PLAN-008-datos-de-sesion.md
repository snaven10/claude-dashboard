# PLAN-008 — Datos nuevos por sesión: mientras no estabas, PRs, subagentes, líneas, turno y permisos

**Fecha:** 2026-10-09
**Fase:** Cerrado
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** investigación de registros del transcript sin usar + maqueta aprobada (`../PLAN-007-bandeja/maqueta.html`)

## 1. Qué resuelve
Mostrar en la bandeja (PLAN-007) lo que Claude Code ya registra y el panel ignora.

## 2. Hallazgos (medidos 2026-10-09, últimos 60 transcripts, solo por claves)
- `system`/`away_summary`: resumen que Claude escribe al volver vos (`content`, p50 239 / máx 396
  caracteres), en 18/60 sesiones. **7 de 18 son ANTERIORES a tu último prompt** (ya no aplican) →
  solo cuenta si es posterior al último prompt real.
- `pr-link`: `prNumber`, `prRepository`, `prUrl`; 50 PRs distintos. **Los URL son de tu GitLab
  interno**, no de GitHub → validar solo `https://` + número > 0.
- `frame-link`: `frameUrl` vacío en 4.999 de 5.029; los no vacíos son `https://claude.ai/…`.
- `cost-state`: `totalLinesAdded`/`totalLinesRemoved` acumulados (nunca decrecen en 68 pares). Trae
  también `totalCostUSD`: **NO se expone** (DD-11 sigue).
- `permission-mode`: `auto` 6.564 · `acceptEdits` 818 · `default` 282.
- `pendingBackgroundAgentCount` solo viene en `turn_duration` (fin de turno) → viejo mientras el turno
  corre. **Se descarta**; los subagentes activos se cuentan por los transcripts
  `<sessionId>/subagents/*.jsonl` modificados en los últimos 2 min.
- `queue-operation`: `enqueue` 4.872 vs `dequeue`+`remove` 4.847 y sin id para emparejar → la cola
  pendiente no se puede calcular confiable. **Se descarta.**
- `turn_duration`: p50 0,8 min, p90 5,1 min, máx 1.871 min → "turno largo" desde 15 min.

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-018** | Backend: `away`, `prs`, `artifacts`, `lines`, `permissionMode`, `subagents`, `turnStartedAt` | claude-dashboard-worker | sonnet | contrato §4 | `done` |
| **TASK-019** | Frontend: mostrar los datos nuevos en filas y detalle de la bandeja | claude-dashboard-worker | sonnet | TASK-017 (PLAN-007) mergeada + contrato §4 | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-018** | Solo backend; corre en paralelo con TASK-017 (PLAN-007, solo `web/`) |
| 2 | **TASK-019** | Requiere la bandeja de TASK-017 en `main` |

## 4. Contrato — campos nuevos por sesión
```json
{
  "away": { "text": "Terminé la migración…", "at": 1791476712278 },
  "prs": [{ "number": 87, "url": "https://gitlab.ejemplo/grupo/repo/-/merge_requests/87", "repo": "grupo/repo" }],
  "artifacts": [{ "title": "Plan de pruebas", "url": "https://claude.ai/artifact/abc" }],
  "lines": { "added": 412, "removed": 96 },
  "permissionMode": "auto",
  "subagents": { "active": 2, "total": 7 },
  "turnStartedAt": 1791476000000
}
```
Cualquiera es `null` (o `[]` en las listas) sin dato.

### Decisiones
**DD-35 — Dónde se calcula cada uno.**
- `away` y `turnStartedAt`: en la lectura de cola de `Summary` (`transcript.go`). `away` = último
  `system`/`away_summary` con `timestamp` posterior al último prompt real (mismo criterio de "prompt
  real" que `lastPrompt`), texto en una línea ≤ 400 runas. `turnStartedAt` = timestamp del último
  prompt real (el frontend lo usa solo en `busy`).
- `prs`, `artifacts`, `lines`, `permissionMode`: en el acumulador incremental de `usage.go` (pueden
  estar al principio del archivo). `prs`: distintos por URL, los últimos 5; solo `https://` y
  número > 0; `repo` ≤ 200. `artifacts`: `frameUrl` no vacío que empiece con `https://claude.ai/`,
  distintos, los últimos 5, título ≤ 200. `lines`: último `cost-state`. `permissionMode`: último
  valor, solo `^[A-Za-z]{1,32}$`. **`totalCostUSD` nunca sale.**
- `subagents`: archivos `*.jsonl` en `<dir del transcript>/<sessionId>/subagents/`; `active` = con
  `mtime` en los últimos 120 s respecto de `now` inyectable; `total` = todos. `null` sin directorio.
  Un `ReadDir` por sesión por ciclo, sin leer contenido.

**DD-36 — Frontend (TASK-019), según la maqueta.**
- Fila: línea secundaria = `away.text` si hay (y la sesión no está `busy`); chips: `N subagentes`
  (activos > 0), `PR #N` (el último), `+A −R`, `turno de N min` (busy y ≥ 15 min), `permissionMode`
  si no es `default`.
- Detalle: bloque "Mientras no estabas" arriba; stats "turno actual" (busy) y "líneas"; sección
  "Links" con los PRs y artifacts (`<a target=_blank rel=noopener>`, URL revalidada en el cliente);
  `permisos: X` en la meta.
- Búsqueda: también por `PR #N` y `repo`.

## 5. Fuera de alcance
- Cola de mensajes y `pendingBackgroundAgentCount` (no confiables, §2). Costo en dólares.

## 6. Cierre
- **Cerrado:** 2026-10-09
- **Shipeó:** rama `main`. TASK-018 `d7b042d` (mergeada con PLAN-007 en `1a142d5`) y TASK-019 `e95a35c` (merge "PLAN-008 TASK-019 con QA"), más arreglos de QA. Servicio reinstalado.
- **Verificación:**
  - `go test -race ./...` (6 paquetes) y 53 tests de node en `main`; código idéntico a las ramas de integración que pasaron QA.
  - Recuento independiente contra 36 sesiones reales con transcript: `prs`, `artifacts`, `lines`, `permissionMode` **36/36**; `away` 36/36 (la única diferencia inicial era el resumen escrito entre el snapshot y el recuento; re-pedido, coincide). El JSON del snapshot no contiene `totalCostUSD` ni `cost`.
  - Navegador: con fixture, chips/“Mientras no estabas”/links con `noopener noreferrer`, hostil inerte, ningún `href` que no sea `https://`; con datos reales 28/36 filas con chips, sin scroll horizontal.
  - Mutaciones: 018 → 2 validaciones de PR sobrevivían (el test insertaba los inválidos al principio y el límite de 5 los sacaba igual) y el reset al truncar no tenía test → tests nuevos que fallan con el código mutado. 019 → validación de URL cubierta (una mutación agresiva rompe 3 tests).
  - **Bug que la QA atrapó (018):** una línea con el marcador `"type":"pr-link"` (u otro) que no fuera de ese tipo se saltaba su conteo de tokens → `consumeExtra` devuelve si consumió; test nuevo.
- **NO verificado:** `subagents.active > 0` con una sesión real (hoy 0 activos); `turnStartedAt` de un turno real ≥ 15 min.
- **Quedó afuera:** cola de mensajes y `pendingBackgroundAgentCount` (§2), costo en dólares.
- **Continúa en:** —
