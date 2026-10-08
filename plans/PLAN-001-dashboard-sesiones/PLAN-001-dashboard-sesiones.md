# PLAN-001 — Dashboard local de sesiones Claude Code

**Fecha:** 2026-10-08
**Fase:** Cerrado
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
| **TASK-001** | Scaffold Go + colector F1/F2 → `[]Session` | claude-dashboard-worker | sonnet | — | `done` |
| **TASK-002** | Lector de cola del `.jsonl` + cache | claude-dashboard-worker | sonnet | TASK-001 (solo el `go.mod`) | `done` |
| **TASK-003** | Server: `/api/sessions`, SSE, poller, embed | claude-dashboard-worker | sonnet | TASK-001, TASK-002 | `done` |
| **TASK-004** | Frontend vanilla (tarjetas, filtros, orden) | claude-dashboard-worker | sonnet | contrato del design (no código) | `done` |
| **TASK-005** | `make install` + unidad systemd `--user` + README + `project` por repo | claude-dashboard-worker | sonnet | TASK-003 | `done` |

`claude-dashboard-worker` lo propuso `orq` (escalón 3, scout) y se guardó con scope proyecto
(`.claude/agents/claude-dashboard-worker.md`, commit `95d9a10`).

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
- **Cerrado:** 2026-10-08
- **Shipeó:** rama `main` (sin remoto). Merges `2fc7342` TASK-001, `bab4775` TASK-004, `a68187c` TASK-002, `5cba136` TASK-003, `d2bba59` TASK-005. Instalado como servicio `systemd --user` (`claude-dashboard.service`, `enabled`), sirviendo `http://localhost:7777`.
- **Verificación:**
  - Cada task con su `## Resultado` y QA del orquestador (vet, `go test -race`, pruebas de mutación donde había un orden o filtro que proteger).
  - Smoke end-to-end contra el servicio instalado (Playwright): 27 sesiones reales, orden de urgencia correcto, indicador `conectado`, título `(5) esperándote`, sin scroll horizontal, consola limpia. Poller 403-475 ms por ciclo con 25-27 sesiones (límite 1,5 s).
  - Criterios cruzados del design: DD-4 un solo poller y emisión solo si cambia (log `sin cambios`); DD-7 `-addr 0.0.0.0` rechazado; DD-6 normalización con la forma real de una background terminada (`state:"done", status:"idle"`).
  - `make install` / `make uninstall` probados de verdad: el uninstall deja la máquina idéntica a antes.
  - Bugs que la QA atrapó y los tests del worker no: test de 50 MB que nunca llegaba a 50 MB (TASK-002), `lastPrompt` tomando mensajes inyectados por el harness (~45 % de los `user` reales, TASK-002), ciclo de orden `After=`/`WantedBy=default.target` en la unidad (TASK-005).
- **NO verificado:** íconos emoji en un navegador real (headless los muestra como cuadros); reconexión del SSE tras reiniciar el servicio con el navegador abierto; arranque en frío con un reinicio real de WSL. Sesiones cuyo worktree ya fue borrado muestran `project` = nombre del worktree (sin `.git` no hay de dónde sacar el repo).
- **Quedó afuera:** lo listado en §4. Sin tasks `skipped`.
- **Continúa en:** candidato PLAN-002 — integración con `orq` (tokens, olas) y hooks si 3 s de latencia no alcanzan. Los huecos de `orq` detectados acá (`tras_merge` no destruye, `harvest` no commitea cierres, `orq status` con jobs de `bgid` vacío como `busy`) van al repo de `orquestar-sesiones`.
