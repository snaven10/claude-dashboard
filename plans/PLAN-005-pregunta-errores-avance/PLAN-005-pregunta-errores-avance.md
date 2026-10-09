# PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos

**Fecha:** 2026-10-08
**Fase:** Aprobado — en ejecución
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** mejoras 1-6 propuestas tras cerrar PLAN-004; el usuario pidió "dale con todas"

## 1. Qué resuelve
| # | Mejora | Para qué |
|---|--------|----------|
| 1 | Qué te está preguntando cada sesión | Saber QUÉ te pide una `waiting` sin abrirla (y contestarla con el QR) |
| 2 | Sesiones peleando con errores | Ver una sesión activa que da vueltas en el mismo error |
| 3 | Avance del plan junto a sus workers | "PLAN-042: 14/28 done · 4 blocked" en el encabezado del grupo |
| 4 | Contador en el ícono de la pestaña | Ver cuántas te esperan aunque la pestaña esté escondida |
| 5 | Contadores clickeables | Tocar "5 esperándote" y ver solo esas |
| 6 | Rama de git de cada sesión | Saber en qué rama trabaja cada worker |

## 2. Hallazgos (medidos 2026-10-08, solo por claves, sin leer contenido)
- **Preguntas:** 203 `tool_use` `AskUserQuestion` en los últimos 40 transcripts; `input.questions[]`
  trae `question`, `header`, `options`, `multiSelect`. Un `tool_use` sin su `tool_result` al final
  de la cola es lo que la sesión está esperando (pregunta o permiso).
- **Errores:** los `tool_result` traen `is_error` (11.952 con la clave, 638 en `true`). En 300
  transcripts: 1.144 errores; 10 son rechazos del usuario ("The user doesn't want to proceed…") y 1
  interrupción → esos NO cuentan. En ventanas de 10 min con al menos un error: p50 1 · p90 2 ·
  p95 3 · **p99 6** · máx 12 → umbral por defecto **5 en 10 min** (DD-22).
- **`Estado:` de las tasks:** 501 líneas en MI-EMPRESA + este repo. 85 % es `` `pending` ``/`` `done` ``
  con backticks; hay variantes sin backticks, en español (`Pendiente`, `PENDIENTE`), con emoji
  (`🟢 \`pending\``) y con texto después (`` `done` — cerrada… ``) → parser tolerante (DD-23).
- **Dónde está el plan de un worker:** los jobs de orq de este repo traen `p<N>-task<M>` en el key;
  los de MI-EMPRESA son `worker@<repo>` y el plan está en la RAMA (`feat-plan-042-…`). Los planes viven en
  `<raíz>/plans/PLAN-<N>-<slug>/` donde la raíz es el repo (este) o un padre (MI-EMPRESA: `~/mi-empresa/plans`).
- La rama sale de `.git/HEAD` (o del `HEAD` del gitdir si `.git` es archivo, como en los worktrees);
  `agents.projectName` ya sigue ese gitdir.

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-011** | Backend: `ask` y `errors` desde el transcript | claude-dashboard-worker | sonnet | contrato §4 | `pending` |
| **TASK-012** | Backend: `branch` y `plan` (avance desde `plans/`) | claude-dashboard-worker | sonnet | contrato §4 | `pending` |
| **TASK-013** | Frontend: pregunta, errores, avance, favicon, contadores, rama | claude-dashboard-worker | sonnet | contrato §4 (no código) | `pending` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-011**, **TASK-012**, **TASK-013** | 011 toca `internal/transcript`, 012 `internal/plans` (nuevo) + git en `internal/agents`, 013 solo `web/`. Se cruzan en `Session`, `poller.go`, `main.go` y README: lo integra el orquestador |

## 4. Contrato — campos nuevos en cada sesión de `GET /api/sessions` / SSE
```json
{
  "ask":    { "kind": "question", "text": "¿Mergeo y reinstalo?", "more": 1 },
  "errors": { "windowMin": 10, "count": 6, "lastAt": 1791476712278, "high": true },
  "branch": "feat-plan-042-latido-front",
  "plan":   { "number": 133, "name": "PLAN-042-ejemplo",
              "path": "/home/…/plans/PLAN-042-ejemplo",
              "total": 28, "done": 14, "inProgress": 2, "blocked": 4,
              "pending": 6, "skipped": 2, "unknown": 0 }
}
```
Cualquiera de los cuatro es `null` si no hay dato. Nada nuevo se escribe en disco.

### Decisiones
**DD-21 — `ask`: lo último que la sesión dejó abierto.** De la cola del transcript (la misma lectura
de `Summary`), en orden de prioridad:
1. Último `tool_use` `AskUserQuestion` SIN `tool_result` posterior → `kind:"question"`,
   `text` = `questions[0].question`, `more` = cantidad de preguntas restantes.
2. Último `tool_use` de otra herramienta SIN `tool_result` posterior → `kind:"tool"`,
   `text` = mismo formato que `lastAction` (`"Bash: git push"`). Si la sesión está `waiting`, la UI
   lo muestra como permiso pedido.
3. Si el último mensaje con contenido es del asistente y tiene texto → `kind:"text"`, ese texto.
4. Si el último mensaje real es del usuario (Claude está trabajando) → `null`.
`text` truncado a 300 runas. Las líneas inyectadas por el harness se ignoran igual que en `lastPrompt`.

**DD-22 — `errors`: `tool_result` con `is_error:true` en los últimos 10 min.** Se calcula en el
acumulador incremental de `usage.go` (misma pasada que `rate`), contra `now` inyectable. NO cuentan
rechazos del usuario (contenido que empieza con `The user doesn't want to proceed`) ni interrupciones
(`[Request interrupted`). El contenido puede ser string o lista de bloques `{type:"text",text}`.
`high` = `count >= -errors-warn` (default **5**, 0 desactiva). Mismo trato de hash que `rate`: no
emitir solo porque envejeció la ventana.

**DD-23 — `plan`: avance leído de los markdown, tolerante.**
- Número de plan: del key de orq (`p<N>-task`) si existe; si no, de la rama con
  `(?i)plan-?(\d+)`. Sin número → `plan: null`.
- Directorio: desde la raíz del repo principal de la sesión (siguiendo el gitdir de un worktree),
  subir hasta `$HOME` buscando `plans/` con una carpeta `PLAN-<N>-*` (N comparado como entero:
  `PLAN-003` = 3). La primera que aparece gana. Nunca se sale de `$HOME`.
- Estado de cada `tasks/*.md`: la PRIMERA línea que contiene `**Estado:**` en las primeras 40
  líneas; se busca la primera palabra del vocabulario `pending|in_progress|blocked|done|skipped`
  (sin mayúsculas, con o sin backticks); `pendiente` = `pending`. Nada reconocible → `unknown`.
- Cache por archivo `(mtime, size)` y por plan; solo lectura.

**DD-24 — `branch` desde `HEAD`, sin ejecutar `git`.** `ref: refs/heads/<x>` → `<x>`; HEAD
desacoplado → los primeros 7 caracteres del sha; sin `.git` → `null`. Se valida que sea una línea
corta de texto imprimible (≤ 200) antes de mandarla.

**DD-25 — Frontend.**
- Tarjeta `waiting`/`blocked`: `ask` destacado (pregunta / "pide permiso: Bash: …" / último texto),
  con "+N preguntas" si `more > 0`. En `idle` se muestra chico como "último mensaje". En `busy` no.
- `errors.high` → insignia "⚠ N errores en 10 min" (con `count` > 0 y sin `high`, solo en el tooltip).
- Grupo por plan: clave = `plan.path` si hay `plan`, si no `orq.plan` como hoy. El encabezado muestra
  `PLAN-042 · 14/28 done · 4 blocked · 2 in_progress` y una barra de progreso.
- Favicon: el círculo actual + número rojo con las que te esperan, dibujado en `<canvas>` →
  `toDataURL`; 0 = favicon original; 10+ = "9+".
- Contadores del encabezado son botones: tocar uno filtra por ese estado; tocar el activo lo quita.
  Se guarda con los demás filtros.
- Rama en la tarjeta (chica, monoespaciada, truncada con `title` completo); entra en la búsqueda.

## 5. Fuera de alcance
- Historial de tokens por día (necesita persistir datos; se decide después de usar el panel).
- Contestar la pregunta desde el panel (sigue siendo solo lectura; para eso está el QR).
- Ejecutar `git` o leer el estado del árbol de trabajo (solo `HEAD`).

## 6. Cierre
