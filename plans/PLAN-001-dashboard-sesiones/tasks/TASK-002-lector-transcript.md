# TASK-002 — Lector de cola del transcript .jsonl

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** TASK-001 (solo el go.mod / tipo Session)
- **Estado:** `pending`

---

## Objetivo
`internal/transcript.Summary(cwd, sessionID) (Detail, error)` devuelve `title`, `lastPrompt`, `lastAction`, `lastActivityAt` leyendo solo el final del archivo.

## Contexto verificado
- Ruta: `~/.claude/projects/<slug>/<sessionId>.jsonl`, `slug` = `cwd` con `/` y `.` → `-` (ej. `/home/usuario/mi-empresa` → `-home-usuario-mi-empresa`). **No confiar ciegamente en el slug**: fallback `glob ~/.claude/projects/*/<sessionId>.jsonl`.
- Tipos de línea vistos en la cola: `user`, `assistant`, `attachment`, `ai-title`, `last-prompt`, `mode`, `permission-mode`, `bridge-session`.
- `user` puede ser un `tool_result` (no es un prompt real) → ignorarlo para `lastPrompt`.
- `assistant.message.content[]` trae bloques `text` y `tool_use` (`name`, `input`).

## Archivos
- **Crear:** `internal/transcript/transcript.go`, `internal/transcript/transcript_test.go`, `internal/transcript/testdata/*.jsonl`

## Pasos
- [ ] **Paso 1 — leer cola** (256 KB desde el final, descartar la primera línea parcial; duplicar hasta 2 MB si no hay `user` real).
- [ ] **Paso 2 — parsear tolerante**: línea que no parsea se salta.
- [ ] **Paso 3 — extraer** último `ai-title`, último prompt real, última acción (`tool_use` → `"<Tool>: <resumen input>"`, o primeros 200 chars del texto), timestamp más reciente.
- [ ] **Paso 4 — cache** por `(path, size, mtime)` con `sync.Mutex`.
- [ ] **Paso 5 — tests** con fixtures sintéticos (NO copiar transcripts reales: tienen datos de MI-EMPRESA).

## Criterios de aceptación
- [ ] Archivo de 50 MB se resume leyendo ≤ 2 MB (test con archivo generado).
- [ ] Segunda llamada sin cambios no relee (verificable con un contador en test).
- [ ] Archivo inexistente → `Detail` vacío sin error fatal.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

