# Referencia técnica

Contrato de datos del daemon y de dónde sale cada campo. Para instalar y usar el panel, ver el
[README](../README.md).

## Campos por sesión (`GET /api/sessions` y SSE `GET /api/events`)
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

Esto abre la SESIÓN en el teléfono, no el panel. Para ver el panel mismo desde el teléfono (tailnet +
PWA + avisos push) está el [modo remoto](#modo-remoto-plan-011); sin él, el panel sigue solo en
`127.0.0.1`.

## Modo remoto (PLAN-011)

Se activa con `-public-url https://<nombre>.ts.net` (`https://`, sin path, sin query, sin
credenciales; se normaliza sin `/` final). **Sin ese flag**: no se abre el segundo listener, no se
crea el directorio de estado, las rutas de esta sección no existen (404) y el poller no avisa. Los
estáticos de la PWA (`manifest.webmanifest`, `icons/`, `sw.js`) se sirven siempre: no tienen datos.

### Dos listeners, los dos en loopback

| Listener | Flag | Default | Quién entra |
|---|---|---|---|
| Local | `-addr` | `127.0.0.1:7777` | el navegador de la PC |
| Remoto | `-remote-addr` | `127.0.0.1:7778` | solo lo que `tailscale serve` reenvía |

`-remote-addr` es loopback siempre (`-allow-remote` no lo afecta) y distinto de `-addr`. El origen
de una petición lo decide **el listener por el que entró**, no las cabeceras ni `RemoteAddr`
(`tailscale serve` es un proxy local: todo llega como `127.0.0.1`).

### Qué exige cada listener

| Ruta | Local | Remoto |
|---|---|---|
| estáticos (`/`, `app.js`, `sw.js`, manifest, íconos) | sí | sí, sin cookie |
| `GET /api/sessions`, `GET /api/events`, `GET /api/sessions/{sessionId}/timeline`, `GET /api/history` | sí, sin cookie | cookie válida o `401` |
| `GET /api/remote` | sí | sí, sin cookie |
| `POST /api/pair/new` | sí | `404` |
| `GET /api/devices`, `DELETE /api/devices/{id}` | sí | `404` |
| `POST /api/pair/redeem` | `404` | sí, sin cookie (con límite de intentos) |
| `GET /api/push/key`, `POST /api/push/subscribe`, `POST /api/push/unsubscribe` | sí (`deviceId` `"local"`) | cookie válida o `401` |

Un `401` responde `{"error":"unpaired"}`. Toda ruta `POST`/`DELETE` exige
`Content-Type: application/json`; si viene `Origin` tiene que ser el del listener (`-public-url` en
el remoto, `http://127.0.0.1:<puerto>` o `http://localhost:<puerto>` en el local) y si viene
`Sec-Fetch-Site` tiene que ser `same-origin` o `none`; si no, `403 {"error":"forbidden"}`. No hay
CORS. Todas las respuestas JSON llevan `Cache-Control: no-store`.

### Endpoints

**`GET /api/remote`** → `{"enabled": true, "local": <bool>, "paired": <bool>}`. `local` es `true` en
el listener local; `paired` dice si la petición trae una cookie de dispositivo válida. Con el modo
remoto apagado la ruta no existe (`404`): el frontend lo toma como `enabled:false`.

**`POST /api/pair/new`** (solo local) → `{"url": "<public-url>/#pair=<token>", "code": "ABCD-EFGH",
"expiresAt": <ms epoch>}`. El token va en el **fragmento** para que no viaje en la petición ni en
logs. Token y código vencen a los 5 minutos y comparten un solo uso; hay **un** pendiente a la vez
(pedir otro invalida el anterior). Vive solo en memoria.

**`POST /api/pair/redeem`** (solo remoto) con `{"token": "…"}` **o** `{"code": "…", "name": "…"}`
(`name` es opcional; si falta se deduce del User-Agent: `iPhone`, `iPad`, `Android` o `dispositivo`).
`200 {"deviceId": "<id>"}` más `Set-Cookie: cd_device=…; Path=/; HttpOnly; Secure; SameSite=Strict;
Max-Age=7776000` (90 días). Inválido o vencido: `401 {"error":"invalid"}` (no distingue). 5 canjes
fallidos invalidan el pendiente. Cuerpo máximo 4 KB.

**`GET /api/devices`** (solo local) → `{"devices": [{"id", "name", "login", "createdAt",
"lastSeenAt"}]}` (ms epoch; `login` es el `Tailscale-User-Login` visto al emparejar, solo
informativo; nunca se expone el hash de la cookie).

**`DELETE /api/devices/{id}`** (solo local) → `204`; `404 {"error":"not found"}` si no existe.
Borra el dispositivo y **todas** sus suscripciones push, su cookie deja de valer en la siguiente
petición y corta sus conexiones SSE abiertas. No toca ninguna sesión de Claude Code.

**`GET /api/push/key`** → `{"key": "<clave pública VAPID, 65 bytes b64url>"}`; es el
`applicationServerKey` de `pushManager.subscribe`.

**`POST /api/push/subscribe`** con `{"endpoint": "…", "keys": {"p256dh": "…", "auth": "…"}}` →
`200 {"ok": true}`. `400 {"error":"invalid subscription"}` si el endpoint no es `https://` con host
no vacío (≤ 1024 caracteres, sin IP literal ni `localhost`: evita usar el daemon contra la red
local) o si `p256dh` no son 65 bytes / `auth` no son 16. Upsert por `endpoint`. Cuerpo máximo 8 KB.

**`POST /api/push/unsubscribe`** con `{"endpoint": "…"}` → `204`. Un dispositivo remoto solo borra
sus propias suscripciones (las ajenas se ignoran con el mismo `204`). Cuerpo máximo 4 KB.

### Archivos de estado

Directorio `-state-dir` (default `~/.config/claude-dashboard/`, nunca `~/.claude/`), `0700`; cada
archivo `0600`, escritura atómica (temp + `fsync` + `rename`). Si lee uno con permisos más abiertos,
loguea un aviso y los corrige.

| Archivo | Contenido |
|---|---|
| `vapid.json` | clave privada ECDSA P-256 + pública (b64url) + `createdAt`. Cambiarla invalida todas las suscripciones |
| `devices.json` | `{id, name, login, cookieHash, createdAt, lastSeenAt}`; `cookieHash` = SHA-256 de la cookie (la cookie en claro no se guarda). `lastSeenAt` se escribe como mucho una vez por minuto por dispositivo |
| `subscriptions.json` | `{deviceId, endpoint, p256dh, auth, createdAt}` |

### Avisos push

El poller evalúa en **cada** ciclo si una sesión pasó a esperarte (`waiting` o `blocked`):

- el primer ciclo tras arrancar solo registra estados;
- una sesión avisa si estuvo en `waiting`/`blocked` **2 ciclos seguidos** y el ciclo anterior no lo
  estaba;
- `-push-cooldown` (default `2m`) evita repetir la misma sesión;
- las abandonadas (sin actividad hace más de 24 h) no avisan;
- si en un ciclo avisan más de 3, sale un solo aviso agrupado "N sesiones te esperan";
- sin suscripciones no hace nada.

El payload es JSON `{"v":1,"title","body","tag","url"}`, cifrado de punta a punta (RFC 8291,
`aes128gcm`) y firmado con VAPID (RFC 8292); el servicio de push ve destino, tamaño y hora.

| `-push-content` | `title` | `body` | `url` |
|---|---|---|---|
| `title` (default) | título (o nombre) de la sesión + `" — esperándote"` | el proyecto | `/?s=<sessionId>` |
| `full` | igual | `proyecto: ` + la pregunta pendiente o lo que espera (≤ 180 caracteres) | `/?s=<sessionId>` |
| `none` | `claude-dashboard` | `Una sesión te espera` | `/` |

En el aviso agrupado el título es `N sesiones te esperan` y la URL `/`. Con `title`/`full` el cuerpo
agrupado lista los proyectos; con `none`, solo la cuenta.

Un push que responde `404`/`410` borra esa suscripción; los `429`/`5xx` no se reintentan. Los
envíos van por una cola acotada que nunca bloquea al poller.
