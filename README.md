# claude-dashboard

Panel local de solo lectura: una tarjeta por sesión de Claude Code, ordenadas por
urgencia (`waiting`/`blocked` → `busy` → `idle` → `done`) y actualizadas por SSE.
Daemon en Go (solo stdlib) con frontend vanilla embebido. URL: <http://localhost:7777>.

## Qué muestra
Por sesión: proyecto (nombre del repo, también si corre en un worktree), estado, qué
espera, título, último prompt y última acción. Además (PLAN-002):

| Campo | Qué es |
|-------|--------|
| `stale` | `true` si la sesión está `busy` y sin actividad hace más de `-stale` (default `10m`, `0` lo desactiva). Es "sin actividad hace X", no "colgada": un build largo se ve igual. |
| `orq` | `{job, plan, task, specialist, ws}` si la sesión es un worker de orq (une `name` de F1 con `bgname` de `~/.orq/jobs.json`); `null` si no. `plan`/`task` salen del key (`p<N>-task<M>`) y son `null` si no matchea. Si `jobs.json` falta o está roto, todas quedan en `null`. |
| `tokens` | `{input, output, cacheRead, cacheCreation, model}` acumulados del transcript (deduplicados por `message.id`); `null` si no hay transcript. Sin costo en dólares. |

PLAN-003 suma, calculados en la misma pasada incremental del transcript que `tokens`:

| Campo | Qué es |
|-------|--------|
| `context` | `{current, compactions, lastCompactAt, lastPreTokens, warn}`. `current` = input + cache_read + cache_creation del ÚLTIMO mensaje del asistente (no la suma). `compactions` cuenta las líneas `compact_boundary`; `lastCompactAt`/`lastPreTokens` son `null` si nunca compactó. `warn` si `current >= -context-warn` (default `800000`, `0` lo desactiva). Es absoluto: el tamaño de ventana no está en ninguna fuente, por eso no hay porcentaje. `null` sin mensajes con usage. |
| `rate` | `{windowMin: 5, output, calls, high}` con los tokens de salida y las llamadas (`message.id` únicos) de los últimos 5 min según los timestamps del transcript. `high` si `output > -rate-output` (default `20000`) o `calls > -rate-calls` (default `25`); `0` desactiva cada uno. `null` sin actividad en la ventana. Es "ritmo inusual", no "loop". |
| `branch` | Rama de git de la sesión, leída de `HEAD` (en un worktree, el `HEAD` del worktree), sin ejecutar `git`. Desacoplado → primeros 7 del sha. `null` sin `.git` o con `HEAD` ilegible. |
| `plan` | Avance del plan `{number, name, path, total, done, inProgress, blocked, pending, skipped, unknown, unmerged}` contando el primer `**Estado:**` de cada `plans/PLAN-<N>-*/tasks/*.md`. `N` sale del key de orq y, si no, de la rama (`plan-?<N>`). Busca `plans/` desde la raíz del repo principal hacia arriba, sin pasar de `$HOME`. `unmerged` = tasks cuyo estado sale de un worktree sin mergear (DD-28): si el plan está dentro del repo, por cada worktree de `.git/worktrees/*/gitdir` (sin ejecutar `git`) gana la copia de la task con estado distinto y `mtime` más nuevo; las tasks que solo existen en el worktree no suman. Plan fuera del repo: sin cambios. `null` sin número o sin plan. |

PLAN-005 suma, de la misma lectura del transcript:

| Campo | Qué es |
|-------|--------|
| `ask` | `{kind, text, more}` con lo último que la sesión dejó abierto, en este orden: un `AskUserQuestion` sin respuesta (`kind:"question"`, `text` = la primera pregunta, `more` = preguntas restantes); una herramienta sin resultado (`kind:"tool"`, `"Bash: git push"`, o sea un permiso pedido si la sesión está `waiting`); el último texto del asistente desde el último prompt real (`kind:"text"`), aunque después haya resultados de herramientas: un permiso pendiente NO queda en el transcript, así que en una `waiting` lo que se ve es lo último que Claude dijo. `null` si no hay texto del asistente desde el último prompt. Una línea, 300 caracteres como máximo. |
| `errors` | `{windowMin: 10, count, lastAt, high}`: `tool_result` con `is_error` de los últimos 10 min. No cuentan los rechazos del usuario ni las interrupciones. `high` si `count >= -errors-warn` (default `5`, `0` lo desactiva). `null` sin errores en la ventana. Se calcula incrementalmente, sin releer el archivo. |
| `away` | `{text, at}`: el último resumen `away_summary` ("mientras no estabas") **posterior al último prompt real**; una línea, 400 caracteres como máximo. `null` si es anterior al prompt o no hay. |
| `turnStartedAt` | ms epoch del último prompt real (el inicio del turno actual; el frontend lo usa solo en `busy`). `null` sin dato. |
| `prs` | Hasta 5 `{number, url, repo}` de los `pr-link` (los últimos, distintos por URL). Solo `https://` y número > 0. `[]` sin dato. |
| `artifacts` | Hasta 5 `{title, url}` de los `frame-link` con `frameUrl` que empiece con `https://claude.ai/` (los vacíos se descartan). `[]` sin dato. |
| `lines` | `{added, removed}` del último `cost-state`. **El costo en dólares no se expone.** `null` sin dato. |
| `permissionMode` | Último `permission-mode` (`auto`, `acceptEdits`, `default`…), solo letras 1-32; un valor raro da `null`. |
| `subagents` | `{active, total}`: archivos `*.jsonl` de `<sessionId>/subagents/` junto al transcript; `active` = modificados en los últimos 120 s. `null` si el directorio no existe. Un `ReadDir` por sesión por ciclo, sin leer contenido. |

`prs`, `artifacts`, `lines` y `permissionMode` salen de la misma pasada incremental de tokens/errores
(pueden estar al principio del archivo); `away` y `turnStartedAt`, de la cola del transcript.

`GET /api/sessions/{sessionId}/timeline` devuelve `{sessionId, events}` con los últimos 20 eventos
(`prompt` | `tool` | `text`, del más viejo al más nuevo), leídos de la cola del transcript. Solo acepta
ids presentes en el último snapshot (404 si no).

`GET /api/history` (PLAN-006) devuelve `{generatedAt, ready, days}`: los últimos 30 días **locales**
terminando hoy (ascendente, con los días en cero), cada uno con `input`, `output`, `cacheRead`,
`cacheCreation`, `messages`, `sessions` y los desgloses `byModel` / `byProject` (top 5 por `total`
más una fila `"otros"`). Se calcula de `~/.claude/projects/**/*.jsonl` — incluye los transcripts de
subagentes, que suman al día y a la sesión padre — con un escáner propio (primer escaneo en segundo
plano y después cada `-history-interval`, default `60s`, `0` lo desactiva), incremental por archivo y
sin escribir nada. La request nunca escanea: devuelve el último resultado; `ready:false` y `days: []`
hasta que termina el primer escaneo. Un transcript borrado deja de contar. Los tokens por sesión de las
tarjetas siguen sin incluir subagentes.

## De dónde saca los datos
| Fuente | Uso | Estabilidad |
|--------|-----|-------------|
| `claude agents --json --all` | Fuente de verdad del estado | Interfaz pública |
| `~/.claude/sessions/<pid>.json` | Enriquecimiento (`statusUpdatedAt`, `bridgeSessionId`) | Formato interno |
| `~/.claude/projects/<slug>/<sessionId>.jsonl` | Detalle (título, prompt, acción), leyendo solo la cola | Formato interno |

Un único poller (cada 3 s, `-interval`) consulta F1; el snapshot se emite por SSE solo si cambió.

## Abrir una sesión en el teléfono (PLAN-004)
Las sesiones con Remote Control activo traen `remoteUrl` (`https://claude.ai/code/<bridgeSessionId>`;
`null` si no hay bridge o el id no tiene el formato visto `session_…`, que no está documentado). En su
tarjeta aparece **📱 abrir en el teléfono**: un panel con el QR (dibujado localmente en un `<canvas>`
con `web/vendor/qrcode.js`, qrcode-generator 1.4.4, MIT; nada sale de la máquina), el link y "copiar link".

- Interactiva sin link: la tarjeta indica escribir `/remote-control` (o `/rc`) en esa sesión; el QR
  aparece solo en ~3 s.
- Background sin link: no se dice nada (no está documentado que admitan Remote Control).
- "Copiar comando" en una interactiva sin terminar avisa que `--resume` abre una segunda copia de la
  conversación y, si hay link, sugiere el QR.
- El setting `remoteControlAtStartup` de Claude Code activa Remote Control en toda sesión interactiva
  nueva. Es decisión tuya; el panel solo lo menciona y no lo toca.

El panel sigue en `127.0.0.1`: se abre la SESIÓN en el teléfono, no el panel.

## Instalación
**Requisitos:** Linux o WSL con systemd `--user`, Go (versión de `go.mod`) para compilar, y
[Claude Code](https://claude.com/claude-code) instalado con `claude` en el `PATH` (el panel ejecuta
`claude agents --json --all`). No hay otras dependencias: solo la stdlib de Go.

```
git clone <url-del-repo> && cd claude-dashboard
make install     # binario en ~/.local/bin + unidad systemd --user, enable --now
make uninstall   # deja todo como estaba
make test | make build
```
Después abrí <http://localhost:7777>. Es **solo lectura** (no ejecuta acciones sobre tus sesiones, no
escribe en `~/.claude/`) y **solo escucha en `127.0.0.1`**. Para probar la interfaz sin daemon:
`web/index.html?fixture`.

La unidad fija `PATH=%h/.local/bin:/usr/bin:/bin` porque el servicio necesita `claude` en PATH.
Ver el estado: `systemctl --user status claude-dashboard`.

**Nota sobre `plans/`:** los SHAs citados en los documentos de `plans/` son de antes de la
publicación (el historial se reescribió) y ya no existen en este repo; sirven solo como referencia
histórica.

## Seguridad
Bindea siempre a `127.0.0.1` (el panel expone prompts). Un `-addr` no-loopback se rechaza
salvo `-allow-remote`. No hay acciones desde el panel.

## Limitaciones
- F2 y F3 son formatos internos de Claude Code sin contrato: si cambian, el panel degrada
  (sin detalle) pero no se cae.
- Latencia de hasta ~3 s (polling), sin hooks.
- Solo sesiones locales; sin historial ni métricas.
- `project` se deriva leyendo `.git` (sin ejecutar `git`); fuera de un repo es el basename del cwd.
