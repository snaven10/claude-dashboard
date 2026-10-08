# PLAN-003 — Diseño técnico

Extiende los contratos de PLAN-001 y PLAN-002. Lo que no se nombra no cambia.

## Contrato: campos nuevos por sesión en `/api/sessions` y en el SSE `snapshot`

```json
{
  "context": {                  // null si no hay mensajes del asistente con usage
    "current": 412000,          // input + cache_read + cache_creation del ÚLTIMO mensaje del asistente
    "compactions": 2,           // líneas compact_boundary en todo el transcript
    "lastCompactAt": 1791476712278,  // ms, null si nunca compactó
    "lastPreTokens": 967001,    // preTokens de la última compactación, null si nunca
    "warn": false               // current >= -context-warn
  },
  "rate": {                     // null si no hubo actividad en la ventana
    "windowMin": 5,
    "output": 12000,            // output_tokens de mensajes con timestamp en los últimos 5 min
    "calls": 9,                 // mensajes del asistente (message.id únicos) en esa ventana
    "high": false               // output > -rate-output || calls > -rate-calls
  }
}
```

## Decisiones

**DD-13 — Contexto absoluto, sin porcentaje.** El tamaño de ventana no está en ninguna fuente.
`warn = current >= -context-warn` (default **800k**: las auto-compactaciones medidas saltan a
~967-988k). El texto en la UI: "contexto 412k · compactó 2×"; con `warn`, "⚠ contexto 850k, cerca de
compactar". `lastPreTokens` permite decir "la última vez compactó en 967k".

**DD-14 — Ritmo desde los timestamps del transcript, no del reloj del poller.** El acumulador de
tokens (DD-9) ya recorre cada línea `assistant`: guarda por archivo una cola de
`(timestamp, output)` por `message.id` único de los últimos 10 min y calcula la ventana de 5 min
contra `now`. Así sobrevive a reinicios (la primera lectura llena la cola) y no depende de cuándo
corrió el poller. Flags `-rate-output 20000` y `-rate-calls 25` (≈ p99 medido). La UI dice
"ritmo inusual", no "loop": un worker intenso también llega ahí.

**DD-15 — Terminadas viejas: solo presentación.** Frontend: `status == done` y
`lastActivityAt` (o `startedAt` si no hay) de hace más de 24 h → van a una sección colapsada
"N terminadas hace más de 1 día", con toggle persistido. **Nada se borra, nada se sugiere borrar**:
`claude rm` elimina sesión y worktree.

**DD-16 — Total por plan en el frontend.** Con "agrupar por plan", la cabecera del grupo suma
`tokens` de sus sesiones (input+output+cacheRead+cacheCreation y, aparte, output). Solo cuenta
las sesiones que F1 todavía lista (las borradas con `claude rm` no están): decirlo en un `title`.

**DD-17 — Búsqueda en el frontend.** Un input filtra por `name`, `title`, `lastPrompt`, `project`,
`orq.plan/task` (sin distinguir mayúsculas ni tildes). Se combina con los filtros existentes.
`/` enfoca el input; Esc lo limpia.
