# TASK-001 — Scaffold Go + colector de sesiones (F1/F2)

- **Plan:** PLAN-001 — Dashboard de sesiones
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** — (primera del plan)
- **Estado:** `done`

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
- **Estado final:** done
- **Resumen:** `agents.List(ctx)` y `agents.Collector{Run, SessionsDir}.List(ctx)` devuelven `[]Session` con el contrato del design. F1 vía `exec.CommandContext` (timeout 10 s, error claro si `claude` no está en PATH), F2 solo por PID devuelto por F1 (falta/basura → se ignora), DD-6 aplicado (background `blocked`→blocked, estados terminales→done, `working`→busy), `project`=basename(cwd), `id` derivado de `sessionId[:8]` si F1 no lo trae. Rama `feature/plan-1-task-001`, commit `a28aaf6` (sin push). Hallazgo F1 real: las background `blocked` vienen SIN `pid` ni `status`; las interactive no traen `id`. El valor exacto de `state` para background terminada no pudo observarse (no había ninguna), se asumió un set de estados terminales (`done/completed/finished/failed/stopped/...`) y background sin state ni status → done: Validado contra la forma real observada después (`state:"done"`, `status:"idle"`, con `pid`): caso `g` en el fixture, commit `432fca6`.
- **Archivos tocados:** `go.mod`, `.gitignore`, `cmd/claude-dashboard/main.go` (stub), `internal/agents/agents.go`, `internal/agents/agents_test.go`, `internal/agents/testdata/f1.json`, `internal/agents/testdata/sessions/{101,102}.json`
- **Verificado por:** QA del orquestador (2026-10-08, a pedido del usuario): `gofmt -l .` limpio, `go vet` OK, `go test` 5/5 PASS, `go.mod` sin `require`. Prueba de mutación: mover `case r.Status != ""` antes de `finishedStates` hace fallar `TestListNormalizacion` (sesión g) — el test protege el orden.

