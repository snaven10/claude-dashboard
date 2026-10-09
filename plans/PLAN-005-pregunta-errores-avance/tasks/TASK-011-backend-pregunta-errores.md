# TASK-011 — Backend: qué pregunta la sesión (`ask`) y errores recientes (`errors`)

- **Plan:** PLAN-005 — Qué te pregunta, errores, avance del plan, rama y atajos
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** contrato §4 del master
- **Estado:** `done`

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
- [x] **Paso 1** — `ask` en `parse`: seguir `tool_use` abiertos (id → nombre/input) y cerrarlos con cada `tool_result`; aplicar la prioridad de DD-21 al final de la cola. Texto en una línea y truncado a 300 runas.
- [x] **Paso 2** — `errors` en el acumulador: cola de timestamps de `tool_result` con `is_error:true`, excluyendo rechazos (`The user doesn't want to proceed`) e interrupciones (`[Request interrupted`); ventana de 10 min contra `now`; `lastAt`.
- [x] **Paso 3** — `Session.Ask`/`Session.Errors`, cableado en el poller, `high` con `-errors-warn` (0 desactiva), hash sin emitir por solo envejecer. README.

## Criterios de aceptación
- [x] Test `ask`: pregunta abierta → `question` con `more` correcto; pregunta ya contestada + texto final del asistente → `text`; `tool_use` Bash sin resultado → `tool` con `"Bash: …"`; último mensaje del usuario → `null`; líneas del harness ignoradas.
- [x] Test `errors`: cuenta solo dentro de 10 min según `now`; excluye rechazo e interrupción; `content` como string y como lista; `high` en el umbral exacto y no debajo.
- [x] Test: la lectura de errores sigue siendo incremental (2º ciclo sin cambios lee 0 bytes).
- [x] Test en el server: el snapshot trae `ask` y `errors` y falla si el poller deja de pedirlos.
- [x] `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados en `Verificado por`.

## Resultado
- **Estado:** `done`
- **Commit:** `df460c8` (código) en la rama `feature/plan-5-task-011`.
- **Archivos:** `internal/transcript/transcript.go` (+`Ask`, `Detail.Ask`, `askScan` dentro de `parse`), `internal/transcript/usage.go` (+`ErrorStats`, `Errors()`, `errTimes`, `countToolErrors`), `internal/agents/agents.go` (`Session.Ask`/`Errors`), `internal/server/{poller,server}.go` (hook `Errors`, `ErrorsWarn`), `cmd/claude-dashboard/main.go` (`-errors-warn`, default 5), `README.md`, y los tests de transcript/usage/server.
- **Decisiones:** `Errors` es una función aparte (`transcript.Errors(cwd, id, now)`) sobre la MISMA pasada incremental de `scan`, en vez de cambiar la firma de `Activity` (no rompe fakes ni otras tasks). `ask` sale de la misma lectura de cola de `Summary`. Una herramienta/pregunta abierta solo cuenta si es posterior al último prompt real; el `text` solo si ningún mensaje del usuario (ni `tool_result`) es más nuevo.
- **Verificado por:** `gofmt -l .` → vacío; `go vet ./...` → ok; `go test -race ./...` → ok (agents, orq, server, transcript). Tests nuevos: `TestAskDD21` (10 casos), `TestAskTruncatesTo300Runes`, `TestErrorsWindowAndExclusions`, `TestErrorsIncremental`, `TestSessionsCarryAskAndErrors`.
- **No verificado:** ejecución contra el daemon y transcripts reales.
- **Gotchas para la integración:** (1) `Session`, `poller.go`, `main.go` y README se cruzan con TASK-012 (`Branch`/`Plan`). (2) `errors.count` cambia cuando la ventana envejece y eso SÍ cambia el hash (igual que `rate.calls`); lo que no lo cambia es el paso del tiempo sin cambio de conteo. (3) El filtro `"is_error"` en `consume` es un `bytes.Contains` previo al parseo, para no pagar el JSON de bloques en cada línea `user`.
