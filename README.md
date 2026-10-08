# claude-dashboard

Panel local de solo lectura: una tarjeta por sesión de Claude Code, ordenadas por
urgencia (`waiting`/`blocked` → `busy` → `idle` → `done`) y actualizadas por SSE.
Daemon en Go (solo stdlib) con frontend vanilla embebido. URL: <http://localhost:7777>.

## Qué muestra
Por sesión: proyecto (nombre del repo, también si corre en un worktree), estado, qué
espera, título, último prompt y última acción.

## De dónde saca los datos
| Fuente | Uso | Estabilidad |
|--------|-----|-------------|
| `claude agents --json --all` | Fuente de verdad del estado | Interfaz pública |
| `~/.claude/sessions/<pid>.json` | Enriquecimiento (`statusUpdatedAt`, `bridgeSessionId`) | Formato interno |
| `~/.claude/projects/<slug>/<sessionId>.jsonl` | Detalle (título, prompt, acción), leyendo solo la cola | Formato interno |

Un único poller (cada 3 s, `-interval`) consulta F1; el snapshot se emite por SSE solo si cambió.

## Instalación
```
make install     # binario en ~/.local/bin + unidad systemd --user, enable --now
make uninstall   # deja todo como estaba
make test | make build
```
La unidad fija `PATH=%h/.local/bin:/usr/bin:/bin` porque el servicio necesita `claude` en PATH.
Ver el estado: `systemctl --user status claude-dashboard`.

## Seguridad
Bindea siempre a `127.0.0.1` (el panel expone prompts). Un `-addr` no-loopback se rechaza
salvo `-allow-remote`. No hay acciones desde el panel.

## Limitaciones
- F2 y F3 son formatos internos de Claude Code sin contrato: si cambian, el panel degrada
  (sin detalle) pero no se cae.
- Latencia de hasta ~3 s (polling), sin hooks.
- Solo sesiones locales; sin historial ni métricas.
- `project` se deriva leyendo `.git` (sin ejecutar `git`); fuera de un repo es el basename del cwd.
