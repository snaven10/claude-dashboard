# PLAN-002 — Diseño técnico

Extiende el contrato de PLAN-001 (`PLAN-001-design.md`). Todo lo que no se nombra acá no cambia.

## Contrato: campos nuevos en cada sesión de `/api/sessions` y del evento SSE `snapshot`

```json
{
  "stale": false,                      // busy/working y sin actividad hace más de -stale (default 10m)
  "orq": {                             // null si la sesión no es un worker de orq
    "job": "9efa2c1a", "plan": "1", "task": "005",
    "specialist": "claude-dashboard-worker", "ws": "claude-dashboard"
  },
  "tokens": {                          // null si no hay transcript o todavía no se contó
    "input": 1234, "output": 5678,
    "cacheRead": 900000, "cacheCreation": 45000,
    "model": "claude-opus-5-5"         // el del último mensaje del asistente
  }
}
```

## `GET /api/sessions/{sessionId}/timeline`

```json
{ "sessionId": "…", "events": [
  { "at": 1791476712278, "kind": "prompt", "text": "…" },   // prompt real del humano
  { "at": 1791476713000, "kind": "tool",   "text": "Bash: go test ./..." },
  { "at": 1791476714000, "kind": "text",   "text": "…" }     // primeros 200 chars
]}
```
Últimos 20 eventos, del más viejo al más nuevo. Se lee de la COLA (DD-3), no del archivo entero.
**Solo se aceptan `sessionId` presentes en el último snapshot** (404 si no): el id nunca se usa
para armar una ruta sin validar. Prompts filtrados con `harnessText` (PLAN-001, TASK-002).

## Decisiones

**DD-8 — `stale` se calcula en el server.** `stale = status ∈ {busy} && now - lastActivityAt > umbral`.
Flag `-stale 10m`. El texto en la UI es "sin actividad hace X", no "colgada": un build largo también
se ve así. `stale` NO entra al hash del snapshot salvo cuando cambia de valor (si no, cada ciclo emite).

**DD-9 — Tokens: acumulador incremental por archivo, deduplicado por `message.id`.**
Estado por `path`: `offset`, totales, `lastMessageID`. En cada ciclo se lee de `offset` al final,
solo líneas completas (la última incompleta se deja para el próximo ciclo). Si `size < offset`
(archivo rotado o truncado) se resetea. Una línea `assistant` suma su `usage` solo si su
`message.id` es distinto del último contado (las repetidas son consecutivas). La primera lectura de
un transcript grande es entera una sola vez; medirla y loguearla.

**DD-10 — `orq` por `~/.orq/jobs.json`.** Se lee en cada ciclo (es chico; cache por mtime).
Unión: `F1.name == job.bgname`. `plan`/`task` con regex `p(\d+)-task(\d+)` sobre `job.key`
(si no matchea, `null` en esos dos campos). Si el archivo no existe o no parsea → `orq: null` en
todas, sin error. Paquete nuevo `internal/orq`.

**DD-11 — Sin costo en dólares.** Los precios cambian y no vienen en el transcript; un número
inventado es peor que ninguno. Se muestran tokens y modelo.

**DD-12 — Notificaciones solo en el frontend.** `Notification` API (localhost cuenta como contexto
seguro). Permiso pedido con un botón explícito (los navegadores bloquean el pedido sin gesto).
Se notifica en la TRANSICIÓN a `waiting`/`blocked` (no en cada snapshot), comparando contra el
snapshot anterior por `sessionId`; el primer snapshot tras cargar la página no notifica.
Preferencia on/off en `localStorage`.
