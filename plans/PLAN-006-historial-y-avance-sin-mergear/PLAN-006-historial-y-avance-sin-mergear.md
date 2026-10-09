# PLAN-006 — Historial de tokens por día y avance que cuenta lo no mergeado

**Fecha:** 2026-10-08
**Fase:** Cerrado
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** mejoras opcionales que quedaron afuera de PLAN-005; el usuario pidió "dale con esas"

## 1. Qué resuelve
| # | Mejora | Para qué |
|---|--------|----------|
| 1 | Historial de tokens por día (últimos 30 días) | Ver cuánto gastás por día, por modelo y por proyecto |
| 2 | El avance del plan cuenta las tasks cerradas en worktrees sin mergear | Que "PLAN-5 0/3" no mienta mientras los workers ya terminaron |

## 2. Hallazgos (medidos 2026-10-08)
- **No hace falta persistir nada:** `~/.claude/projects` tiene 991 transcripts (1,6 GB) con 52 días de
  datos (desde 2026-08-16; `cleanupPeriodDays` no está seteado). Un prototipo en Python suma el
  `output` por día de TODOS en 7,8 s. → El historial se CALCULA de los transcripts (DD-26): sigue
  siendo solo lectura y sobrevive reinicios sin escribir un byte.
- **Los subagentes tienen transcript propio** (`<sessionId>/subagents/*.jsonl`): 38.500 mensajes con
  `usage`. El historial los cuenta (si no, subestima mucho). Ojo: los tokens por sesión de las
  tarjetas NO los incluyen hoy (fuera de alcance, se documenta).
- Los `timestamp` están en UTC (`…Z`): agrupar por día UTC corre el día 6 h en El Salvador → se
  agrupa por día LOCAL (`time.Local`).
- Un transcript borrado (por `claude rm` o por la limpieza automática) desaparece del historial.
- **Worktrees sin `git`:** `<repo>/.git/worktrees/<x>/gitdir` contiene la ruta al `.git` del
  worktree → de ahí la raíz del worktree. Un worker de orq cierra su task en la COPIA del plan de su
  worktree; `main` la sigue viendo `pending` hasta el merge.

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-014** | Backend: historial por día (`internal/history`, `GET /api/history`) | claude-dashboard-worker | sonnet | contrato §4 | `done` |
| **TASK-015** | Backend: avance del plan con worktrees sin mergear | claude-dashboard-worker | sonnet | contrato §4 | `done` |
| **TASK-016** | Frontend: vista de historial y "sin mergear" en el encabezado del plan | claude-dashboard-worker | sonnet | contrato §4 (no código) | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-014**, **TASK-015**, **TASK-016** | 014 = paquete nuevo + ruta; 015 = `internal/plans`; 016 = solo `web/`. Se cruzan en `server.go`/`main.go`/README: integra el orquestador. local admite 2 a la vez |

## 4. Contrato
### `GET /api/history` (nuevo)
```json
{
  "generatedAt": 1791476712278,
  "ready": true,
  "days": [
    { "date": "2026-10-08", "input": 1200, "output": 1899605, "cacheRead": 9.1e8, "cacheCreation": 3.2e7,
      "messages": 4210, "sessions": 23,
      "byModel":   [{ "name": "claude-opus-5-5", "output": 1500000, "total": 7.0e8 }],
      "byProject": [{ "name": "mi-empresa", "output": 1200000, "total": 5.0e8 }] }
  ]
}
```
- `days`: los últimos 30 días LOCALES terminando hoy, en orden ascendente, **incluidos los días en 0**
  (30 entradas siempre, salvo `ready:false`).
- `byModel` y `byProject` ordenados por `total` desc, máximo 5 (el resto se suma en `"otros"`).
- `ready:false` + `days: []` mientras termina el primer escaneo.
### `plan` en cada sesión (cambia)
Se agrega `"unmerged": 2` = tasks cuyo estado sale de un worktree (DD-28). Los demás contadores ya
incluyen ese estado.

### Decisiones
**DD-26 — Historial calculado, no persistido.** Paquete `internal/history`: recorre
`~/.claude/projects/**/*.jsonl` (incluye `subagents/`), lee incremental por archivo (offset, como
`usage.go`, bloques de 4 MB, líneas hasta 64 MB), dedup por `message.id` consecutivo por archivo.
Solo líneas `type:"assistant"` con `message.usage`. Acumula por (día local, modelo, proyecto,
sessionId). `sessions` = sessionIds distintos con mensajes ese día (un subagente cuenta para la
sesión padre: el `sessionId` del directorio). Proyecto = mismo criterio que `agents.projectName`
sobre el `cwd` de la línea (exportalo o reusalo, sin duplicar la lógica). Archivo que se achica →
se recalcula desde 0 restando lo que había aportado. Archivo que desaparece → se resta su aporte.

**DD-27 — Un escáner aparte, nunca en el poller.** Goroutine propia: primer escaneo al arrancar en
segundo plano (`ready:false` mientras tanto) y después cada 60 s. El poller de 3 s y el SSE no se
tocan. `GET /api/history` devuelve el último resultado calculado (no escanea en la request).
Log del tiempo del primer escaneo.

**DD-28 — Avance con worktrees sin mergear.** Si el directorio del plan está DENTRO del repo
principal (`<repo>/plans/…`), por cada worktree del repo (de `<repo>/.git/worktrees/*/gitdir`, sin
ejecutar `git`) se mira `<worktree>/plans/<mismo plan>/tasks/<mismo archivo>`. Si existe, su estado
es distinto al de `main` y su `mtime` es más nuevo → gana el del worktree y suma a `unmerged`. Tasks
que solo existen en un worktree NO se agregan. Plan fuera del repo (MI-EMPRESA: `~/mi-empresa/plans`) → sin
cambios. Nunca se escribe nada.

**DD-29 — Frontend.**
- Botón "📊 historial" en el encabezado → panel (mismo patrón que la línea de tiempo) que pide
  `/api/history`: barras por día del **output** (30 barras, la de hoy marcada) y debajo, para el día
  elegido (por defecto hoy; click en una barra lo cambia), totales y las tablas por modelo y por
  proyecto. Barras dibujadas con elementos (divs con altura) o `<canvas>`, nunca `innerHTML`.
  `ready:false` → "calculando historial…" y reintento a los 3 s.
- Encabezado del plan: `PLAN-5 · 3/3 done (2 sin mergear)` cuando `unmerged > 0`.

## 5. Fuera de alcance
- Sumar los subagentes a los tokens por sesión de las tarjetas.
- Historial más allá de lo que Claude Code conserva en disco; costo en dólares (DD-11).

## 6. Cierre
- **Cerrado:** 2026-10-08
- **Shipeó:** rama `main` (sin remoto). Merge `54daddf` (integración de TASK-014 `44d36d7`, TASK-015 `4307ada`, TASK-016 `1e81974` y arreglos de QA). Servicio reinstalado con `make install`.
- **Verificación:**
  - `gofmt`, `go vet`, `go test -race` (6 paquetes, incluido el nuevo `internal/history`) y 33 tests de node en `main`; el código de `main` es idéntico a la rama de integración que pasó QA.
  - Mutaciones (descartando las que no compilan): 014 10/10; 015 3/6, de las sobrevivientes 2 son equivalentes (doble resguardo) y 1 era un hueco real (orden del glob) → test nuevo.
  - Historial contra los transcripts reales, recuento independiente en Python: **29/30 días exactos** (input, output, caches, mensajes, sesiones) y las filas por modelo/proyecto suman el total; hoy difiere en 3 de 4.606 mensajes por sesiones vivas escribiendo entre el escaneo y el recuento. Primer escaneo: **5,2-5,5 s para 1,58 GB, 24 MB de RSS**.
  - `unmerged` con un caso real: los 3 worktrees de este plan con las tasks cerradas y `main` en `pending` → `3/3 done`, `unmerged 3`.
  - Navegador: panel de historial con datos reales en claro 1280 y oscuro 360, sin scroll horizontal; fixture con proyecto hostil inerte; `(2 sin mergear)` en el encabezado.
  - **Bugs que la QA atrapó:** `scanFile` reservaba un buffer de 4 MB por archivo en CADA pasada aunque no hubiera bytes nuevos (~4 GB/min de basura con 991 archivos) → sale antes y dimensiona el buffer, con test de memoria que falla con el código viejo. Tablas del historial ilegibles a 360 px (nombres partidos) → columna output oculta en angosto.
- **NO verificado:** el cambio de día a medianoche con el server corriendo; el historial después de que la limpieza automática borre transcripts viejos.
- **Quedó afuera:** lo de §5. Sin tasks `skipped`.
- **Continúa en:** —
