# PLAN-001 — Dashboard local de sesiones Claude Code

**Fecha:** 2026-10-08
**Fase:** 1 (Planning) — pendiente de aprobación
**Diseño:** [`PLAN-001-design.md`](./PLAN-001-design.md)
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario — ver en un navegador qué hace cada sesión de Claude Code y en qué estado está

## 1. Qué resuelve
Con 15-20 sesiones abiertas (interactivas, `--bg`, las de `orq`) no hay forma de ver de un
vistazo cuál está trabajando, cuál terminó y cuál está **trabada esperándote**. El panel en
`http://localhost:7777` muestra una tarjeta por sesión, ordenada por urgencia, y se actualiza solo.

## 2. Hallazgos que reorientan el plan
- **`claude agents --json --all` existe y ya da el estado** (`busy`/`idle`/`waiting` +
  `waitingFor: "input needed"`). Verificado hoy: 21 sesiones → 10 idle, 6 busy, 1 waiting,
  4 background blocked. → No hace falta adivinar el estado por timestamps ni configurar hooks (DD-1, DD-2).
- `~/.claude/sessions/<pid>.json` y `~/.claude/projects/**.jsonl` dan el detalle, pero son
  formato interno → solo para enriquecer, con degradación (DD-1).
- Node 20 está EOL; el python local no tiene pip → **Go stdlib** (DD-5).
- `orq` no podía despachar este plan (workspace `~/mi-empresa` fijo, `--add-dir ~/mi-empresa` forzado,
  sin specialist → sesión genérica en silencio). **Resuelto en el PLAN-001 de
  `orquestar-sesiones`** (workspaces `repo`, 3 escalones, scout, gate de spawn).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-001** | Scaffold Go + colector F1/F2 → `[]Session` | claude-dashboard-worker | sonnet | — | `pending` |
| **TASK-002** | Lector de cola del `.jsonl` + cache | claude-dashboard-worker | sonnet | TASK-001 (solo el `go.mod`) | `pending` |
| **TASK-003** | Server: `/api/sessions`, SSE, poller, embed | claude-dashboard-worker | sonnet | TASK-001, TASK-002 | `pending` |
| **TASK-004** | Frontend vanilla (tarjetas, filtros, orden) | claude-dashboard-worker | sonnet | contrato del design (no código) | `pending` |
| **TASK-005** | `make install` + unidad systemd `--user` + README | claude-dashboard-worker | haiku | TASK-003 | `pending` |

`claude-dashboard-worker` **todavía no existe**: lo propone `orq` (escalón 3, scout) y el usuario
elige el scope al guardarlo.

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-001**, **TASK-004** | TASK-004 solo depende del contrato del design; no toca archivos Go |
| 2 | **TASK-002** | requiere `go.mod` y el tipo `Session` de TASK-001 |
| 3 | **TASK-003** | requiere TASK-001 y TASK-002 cerradas |
| 4 | **TASK-005** | requiere el binario de TASK-003 |

## 4. Fuera de alcance
- Hooks hacia el server (eventos en tiempo real sub-segundo). Si 3 s de latencia no alcanzan → PLAN-002.
- Acciones desde el panel (attach, stop, mandar mensajes). v1 es **solo lectura**; a lo sumo
  un botón que copia el comando `claude attach <id>`.
- Sesiones de remota (vía SSH) y de claude.ai.
- Historial / métricas de costo.
- Integrar con `orq` (mostrar tokens, olas). Candidato a PLAN-002.

## 5. Cierre
<!-- SE LLENA AL CERRAR EL PLAN -->
