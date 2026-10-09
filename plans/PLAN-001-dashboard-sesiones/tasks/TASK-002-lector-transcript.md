# TASK-002 — Lector de cola del transcript .jsonl

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-001 (solo el go.mod / tipo Session)
- **Estado:** `done`

---

## Objetivo
`internal/transcript.Summary(cwd, sessionID) (Detail, error)` devuelve `title`, `lastPrompt`, `lastAction`, `lastActivityAt` leyendo solo el final del archivo.

## Contexto verificado
- Ruta: `~/.claude/projects/<slug>/<sessionId>.jsonl`, `slug` = `cwd` con `/` y `.` → `-` (ej. `~/proyectos/mi-empresa` → `-home-user-proyectos-mi-empresa`). **No confiar ciegamente en el slug**: fallback `glob ~/.claude/projects/*/<sessionId>.jsonl`.
- Tipos de línea vistos en la cola: `user`, `assistant`, `attachment`, `ai-title`, `last-prompt`, `mode`, `permission-mode`, `bridge-session`.
- `user` puede ser un `tool_result` (no es un prompt real) → ignorarlo para `lastPrompt`.
- `assistant.message.content[]` trae bloques `text` y `tool_use` (`name`, `input`).

## Archivos
- **Crear:** `internal/transcript/transcript.go`, `internal/transcript/transcript_test.go`, `internal/transcript/testdata/*.jsonl`

## Pasos
- [x] **Paso 1 — leer cola** (256 KB desde el final, descartar la primera línea parcial; duplicar hasta 2 MB si no hay `user` real).
- [x] **Paso 2 — parsear tolerante**: línea que no parsea se salta.
- [x] **Paso 3 — extraer** último `ai-title`, último prompt real, última acción (`tool_use` → `"<Tool>: <resumen input>"`, o primeros 200 chars del texto), timestamp más reciente.
- [x] **Paso 4 — cache** por `(path, size, mtime)` con `sync.Mutex`.
- [x] **Paso 5 — tests** con fixtures sintéticos (NO copiar transcripts reales: tienen datos de el workspace multi-repo).

## Criterios de aceptación
- [x] Archivo de 50 MB se resume leyendo ≤ 2 MB (test con archivo generado).
- [x] Segunda llamada sin cambios no relee (verificable con un contador en test).
- [x] Archivo inexistente → `Detail` vacío sin error fatal.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:** done
- **Resumen:** `transcript.Summary(cwd, sessionID) (Detail, error)`. Lee la cola (256 KB, ampliando solo el tramo nuevo hasta 2 MB si no hay `user` real; total leído ≤ 2 MB), descarta la primera línea parcial, parseo tolerante, ignora `user` con `tool_result`, `lastAction` = último `tool_use` ("Tool: resumen") o texto (200 chars), cache por (path,size,mtime) con `sync.Mutex`, fallback por glob si el slug no existe, archivo inexistente → `Detail{}` sin error. `Root` (var) permite apuntar a fixtures; contadores `bytesRead`/`readCalls` para los tests. Nombres `aiTitle`/`lastPrompt` verificados contra transcripts reales (solo claves, sin contenido); hay `last-prompt` sin `lastPrompt`, y el recorrido de atrás hacia adelante no los deja pisar uno bueno. Fix del orquestador (`56268a9`): en transcripts reales ~45 % de los `user` sin tool_result son inyectados por el harness (`isMeta`, `<task-notification>`, `<local-command-*>`, `<command-*>`); ahora no cuentan como `lastPrompt` (consecuencia: un slash command no se muestra como prompt).
- **Archivos tocados:** `internal/transcript/transcript.go`, `internal/transcript/transcript_test.go`, `internal/transcript/testdata/-work-demo/sess-basic.jsonl`, `internal/transcript/testdata/-other-slug/sess-glob.jsonl`. Commits `ea0c1de`, `6adec8d`, `56268a9` en rama `feature/plan-1-task-002`.
- **Verificado por:** QA del orquestador (2026-10-08): `gofmt` limpio, `go vet` OK, `go test -race` 8/8 PASS, `go.mod` sin dependencias, fixtures sintéticos. `TestBigFileReadsAtMost2MB` fallaba por un bug del propio test (archivo de 52.428.685 B < 50 MiB por división entera) — corregido en `6adec8d`. Prueba de mutación: quitar el filtro `isMeta` o el de `<task-notification>` hace fallar `TestSummaryBasic`.
