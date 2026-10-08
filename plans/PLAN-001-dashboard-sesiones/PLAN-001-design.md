# PLAN-001 — Diseño técnico

## Fuentes de datos (verificadas 2026-10-08, Claude Code 2.1.294)

| # | Fuente | Qué da | Estabilidad |
|---|--------|--------|-------------|
| F1 | `claude agents --json --all` | Lista de sesiones vivas + background terminadas. Campos: `id`, `sessionId`, `pid`, `cwd`, `kind` (`interactive`/`background`), `name`, `startedAt`, `status` (`idle`/`busy`/`waiting`), `waitingFor` (ej. `"input needed"`), `state` (background: `blocked`, …) | **Alta** — interfaz pública documentada "for scripting" |
| F2 | `~/.claude/sessions/<pid>.json` | `name`, `cwd`, `status`, `statusUpdatedAt`, `bridgeSessionId`, `version`, `procStart` | Media — formato interno, sin contrato |
| F3 | `~/.claude/projects/<cwd-slug>/<sessionId>.jsonl` | Detalle: líneas `user`, `assistant` (texto + `tool_use`), `ai-title`, `last-prompt`, timestamps | Media — formato interno |

**DD-1 — F1 es la fuente de verdad del estado.** F2 solo para enriquecer (`statusUpdatedAt`,
`bridgeSessionId`). F3 solo para el detalle (última tarea, última acción). Si F2/F3 cambian de
formato, el panel degrada (sin detalle) pero NO se cae.

**DD-2 — Sin hooks en v1.** F1 ya entrega `waiting` + `waitingFor`, que era lo único que
justificaba los hooks (`Notification`). No se toca `~/.claude/settings.json`.

**DD-3 — Leer solo la cola del `.jsonl`.** Los transcripts pesan MB. Se lee desde el final
(últimos 256 KB, ampliando si no aparece un `user`). Cache por `(sessionId, size, mtime)`:
si no cambió, no se relee.

**DD-4 — Polling de F1 cada 3 s, push por SSE.** `claude agents --json` es un proceso Node
(~300-500 ms). Un solo poller en el server, nunca uno por cliente. Se emite un evento SSE solo
si el snapshot cambió (hash).

## Lenguaje — DD-5: Go (stdlib) + frontend sin build

| Opción | A favor | En contra |
|--------|---------|-----------|
| **Go 1.27** ✅ | Binario único con `go:embed` del frontend; `net/http` + SSE + `os/exec` + `encoding/json` sin dependencias; ~10 MB RAM como daemon; compila en segundos | No es tu lenguaje del día a día |
| Rust 1.98 | Ya lo usás (DevCtxEngine); binario único | axum + tokio + serde + rust-embed para algo que es leer archivos; iterar es más lento |
| Node 20 | Tu stack (TS) | **Node 20 está EOL desde abril 2026**; TS necesita build o Node ≥22.6; un daemon más pesado |
| Python 3.14 | Rápido de escribir | El Python de la máquina **no tiene pip**: todo stdlib, SSE con hilos a mano; sin binario |

Frontend: `index.html` + `app.js` + `app.css` vanilla, embebidos. Sin framework: son tarjetas
que se repintan con cada evento SSE. Angular/React acá es usar una grúa para colgar un cuadro.

## Contrato `GET /api/sessions` (y payload de cada evento SSE `snapshot`)

```json
{
  "generatedAt": 1791476712278,
  "sessions": [{
    "id": "971b887d", "sessionId": "971b887d-…", "pid": 1816810,
    "kind": "interactive",
    "name": "demo-5c",
    "cwd": "/home/usuario/mi-empresa", "project": "mi-empresa",
    "status": "busy",               // busy | waiting | idle | blocked | done
    "waitingFor": null,
    "startedAt": 1791476356334, "lastActivityAt": 1791476712278,
    "remote": true,                 // tiene bridgeSessionId
    "title": "…",                   // ai-title
    "lastPrompt": "…",              // último mensaje user (truncado 500)
    "lastAction": { "kind": "tool", "text": "Bash: git status" }  // o kind "text"
  }]
}
```

Normalización de `status` (DD-6): interactive → `status` de F1; background → `state` de F1
(`blocked` se muestra como "esperándote"); background terminada (`--all`) → `done`.

Orden en UI: `waiting`/`blocked` primero (te necesitan), luego `busy`, `idle`, `done`.

## Layout

```
claude-dashboard/
├── cmd/claude-dashboard/main.go   # flags (-addr 127.0.0.1:7777), wiring
├── internal/agents/               # F1 + F2 → []Session (TASK-001)
├── internal/transcript/           # F3, lectura de cola + cache (TASK-002)
├── internal/server/               # HTTP, SSE, poller, embed (TASK-003)
├── web/                           # index.html, app.js, app.css (TASK-004)
├── deploy/claude-dashboard.service  # systemd --user (TASK-005)
└── Makefile
```

**DD-7 — Bind a `127.0.0.1` siempre.** El panel expone tus prompts; nunca `0.0.0.0`. WSL2
reenvía `localhost` a Windows, así que el navegador de Windows lo abre igual.
