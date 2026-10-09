# TASK-011 — Backend: qué pregunta la sesión (`ask`) y errores recientes (`errors`)

- **Plan:** PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `pending`

---

## Objetivo
Cada sesión trae `ask` (DD-21) y `errors` (DD-22) según el contrato §4 del master.

## Contexto verificado
- Leé el master entero, sobre todo §2, §4, DD-21 y DD-22.
- `ask` sale de la MISMA lectura de cola que `Summary` (`internal/transcript/transcript.go`: `summarizeFile`, `parse`, `realUserText`, `harnessText`, `assistantAction`, `summarizeInput`, `truncate`). Extendé `Detail`; no agregues otra lectura del archivo.
- `errors` sale del acumulador incremental de `internal/transcript/usage.go` (`usageState.consume`, `result(now)`, `pruneRecent`, cola de `rate`). Extendelo igual que se hizo con `rate`; la lectura sigue siendo incremental.
- Forma real de un error: línea `type:"user"` con `message.content[]` que contiene `{"type":"tool_result","tool_use_id":"…","is_error":true,"content": <string | [{"type":"text","text":"…"}]>}` y `timestamp` en la línea.
- Forma real de una pregunta: bloque del asistente `{"type":"tool_use","id":"toolu_…","name":"AskUserQuestion","input":{"questions":[{"question":"…","header":"…","options":[…],"multiSelect":false}]}}`. Su respuesta es un `tool_result` con el mismo `tool_use_id`.
- El poller (`internal/server/poller.go`: `applyDetail`, `applyActivity`) y `TestSessionsCarryContextAndRate` muestran cómo cablear y cómo cuidar que el poller no deje de pedir los datos. Mirá también cómo el hash del snapshot evita emitir cuando `rate` solo envejece.
- **Fixtures sintéticos solamente.** Nunca copies texto de transcripts reales a tests.

## Archivos
- **Modificar:** `internal/transcript/transcript.go`, `internal/transcript/transcript_test.go`, `internal/transcript/usage.go`, `internal/transcript/usage_test.go`, `internal/agents/agents.go` (campos `Ask`, `Errors` en `Session` + tipos), `internal/server/poller.go`, `internal/server/server.go` (umbral), `internal/server/server_test.go`, `cmd/claude-dashboard/main.go` (flag `-errors-warn`, default 5), `README.md`

## Pasos
- [ ] **Paso 1** — `ask` en `parse`: seguir `tool_use` abiertos (id → nombre/input) y cerrarlos con cada `tool_result`; aplicar la prioridad de DD-21 al final de la cola. Texto en una línea y truncado a 300 runas.
- [ ] **Paso 2** — `errors` en el acumulador: cola de timestamps de `tool_result` con `is_error:true`, excluyendo rechazos (`The user doesn't want to proceed`) e interrupciones (`[Request interrupted`); ventana de 10 min contra `now`; `lastAt`.
- [ ] **Paso 3** — `Session.Ask`/`Session.Errors`, cableado en el poller, `high` con `-errors-warn` (0 desactiva), hash sin emitir por solo envejecer. README.

## Criterios de aceptación
- [ ] Test `ask`: pregunta abierta → `question` con `more` correcto; pregunta ya contestada + texto final del asistente → `text`; `tool_use` Bash sin resultado → `tool` con `"Bash: …"`; último mensaje del usuario → `null`; líneas del harness ignoradas.
- [ ] Test `errors`: cuenta solo dentro de 10 min según `now`; excluye rechazo e interrupción; `content` como string y como lista; `high` en el umbral exacto y no debajo.
- [ ] Test: la lectura de errores sigue siendo incremental (2º ciclo sin cambios lee 0 bytes).
- [ ] Test en el server: el snapshot trae `ask` y `errors` y falla si el poller deja de pedirlos.
- [ ] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
