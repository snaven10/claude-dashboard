# PLAN-009 — Publicación en GitHub: limpiar datos privados y reescribir el historial

**Fecha:** 2026-10-09
**Fase:** 2 (Ejecución) — aprobado 2026-10-09
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario — publicar el panel como repo público (MIT) para aportes de la comunidad

## 1. Qué resuelve
El panel ya es genérico (solo lee el `~/.claude/` de quien lo corre), pero el repo arrastra
nombres del cliente del usuario en datos de prueba, comentarios y planes, y en todo su historial.
Este plan limpia el árbol, agrega licencia, reescribe el historial y publica.

## 2. Hallazgos (verificados 2026-10-09)
- **El código no necesita archivo de config**: no hay nada del usuario en la lógica. Lo que hay:
  - Comentarios: `internal/plans/plans.go:149`, `internal/plans/plans_test.go:286` ("layout MI-EMPRESA").
  - Datos de prueba: `web/fixture-history.json` (`"name": "mi-empresa"` ×23), `web/app.test.js:281`
    (`PLAN-042-calidad`), ramas `plan-042-*` en fixtures/tests.
  - Planes: PLAN-001, 002, 005, 006 (rutas `/home/usuario/mi-empresa`, nombre real de un plan del
    cliente, `web-frontend`, `remota`).
  - Agente: `.claude/agents/claude-dashboard-worker.md` ("datos de MI-EMPRESA").
- **Historial:** 44 líneas con el nombre del cliente en `git log --all -p`; además `PLAN-042-*`.
- **Autor de los 102 commits:** `snaven10 <funes@samuelfunes.com>`.
- Los SHAs citados en `plans/` quedan viejos tras reescribir (PLAN-002 de orq, DD-5).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-020** | Limpiar árbol: fixtures, tests, comentarios, planes, agente; `LICENSE` MIT; README de instalación | claude-dashboard-worker | sonnet | — | `pending` |
| **TASK-021** | Reescribir historial y publicar (gate del usuario antes del push) | orquestador | — | TASK-020 | `pending` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-020** | Sin dependencias; en paralelo con PLAN-002 de orq (otro repo) |
| 2 | **TASK-021** | Requiere TASK-020 mergeada y PLAN-002 TASK-011 de orq cerrada |

TASK-020 puede correr en paralelo con PLAN-002 de orq (otro repo). TASK-021 comparte
`reemplazos.txt` con PLAN-002 TASK-012 y se ejecutan juntas.

## 4. Fuera de alcance
- Cambios de funcionalidad del panel.
- Releases con binarios, CI de GitHub Actions, traducción al inglés: pueden venir después.
- Soporte de otros sistemas además de Linux/WSL con systemd `--user` (el README lo dice).

## 5. Cierre
<!-- SE LLENA AL CERRAR EL PLAN -->
