# TASK-001 — Scaffold Go + colector de sesiones (F1/F2)

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** — (primera del plan)
- **Estado:** `pending`

---

## Objetivo
`internal/agents.List(ctx) ([]Session, error)` devuelve las sesiones normalizadas según el contrato del design.

## Contexto verificado
- `claude agents --json --all` imprime un array JSON sin TTY (campos en design §Fuentes F1). Tarda ~0.3-0.5 s.
- `~/.claude/sessions/<pid>.json` trae `statusUpdatedAt`, `bridgeSessionId`, `procStart`. Hay ~170 archivos, muchos de PIDs muertos → solo leer los de los PIDs que devolvió F1.
- Go 1.27.1 instalado. Solo stdlib.

## Archivos
- **Crear:** `go.mod` (module `claude-dashboard`), `cmd/claude-dashboard/main.go` (stub), `internal/agents/agents.go`, `internal/agents/agents_test.go`, `.gitignore`

## Pasos
- [ ] **Paso 1 — scaffold.** `go mod init claude-dashboard`, layout del design.
- [ ] **Paso 2 — tipo `Session`** con tags JSON exactos del contrato.
- [ ] **Paso 3 — ejecutar F1** con `exec.CommandContext` + timeout 10 s; parsear a struct crudo.
- [ ] **Paso 4 — enriquecer con F2** por `pid` (si el archivo falta o no parsea, se ignora).
- [ ] **Paso 5 — normalizar `status`** (DD-6) y derivar `project` = basename del `cwd`.
- [ ] **Paso 6 — test** con un JSON fixture de F1 (sin ejecutar `claude`): inyectar el runner como función.

## Criterios de aceptación
- [ ] Test de normalización cubre interactive busy/idle/waiting, background blocked y background terminada → `done`.
- [ ] Si `claude` no está en PATH, `List` devuelve error claro, no panic.
- [ ] Cero dependencias externas en `go.mod`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**

