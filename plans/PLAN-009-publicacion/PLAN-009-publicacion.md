# PLAN-009 — Publicación en GitHub: limpiar datos privados y reescribir el historial

**Fecha:** 2026-10-09
**Fase:** Cerrado
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario — publicar el panel como repo público (MIT) para aportes de la comunidad

## 1. Qué resuelve
El panel ya es genérico (solo lee el `~/.claude/` de quien lo corre), pero el repo arrastra
nombres del cliente del usuario en datos de prueba, comentarios y planes, y en todo su historial.
Este plan limpia el árbol, agrega licencia, reescribe el historial y publica.

## 2. Hallazgos (verificados 2026-10-09)
- **El código no necesita archivo de config**: no hay nada del usuario en la lógica. Lo privado
  estaba en dos comentarios de `internal/plans/`, en los datos de prueba (`web/fixture*.json`,
  `web/app.test.js`, `internal/plans/plans_test.go`: nombre del workspace del cliente y número de
  un plan real), en cuatro planes (rutas del HOME, nombres de repos y planes del cliente, la otra
  máquina del usuario) y en el agente del repo.
- **Historial:** ~44 líneas con el nombre del cliente en `git log --all -p`, más el plan real.
- **Autor de los commits:** el usuario, con su correo propio (decisión: se publica tal cual).
- Los SHAs citados en `plans/` quedan viejos tras reescribir (PLAN-002 de orq, DD-5).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-020** | Limpiar árbol: fixtures, tests, comentarios, planes, agente; `LICENSE` MIT; README de instalación | claude-dashboard-worker | sonnet | — | `done` |
| **TASK-021** | Reescribir historial y publicar (gate del usuario antes del push) | orquestador | — | TASK-020 | `done` |

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
- **Cerrado:** 2026-10-09
- **Shipeó:** https://github.com/snaven10/claude-dashboard (público, MIT). TASK-020 (árbol limpio, LICENSE, README de instalación) y TASK-021 (historial reescrito, README nuevo, publicación).
- **Verificación:** ver el Resultado de TASK-021: 0 restos en todo el historial, tests en verde en el repo reescrito, servicio reinstalado desde el repo publicado (HTTP 200).
- **Quedó afuera:** CI, releases con binarios, README en inglés (§4).
- **Continúa en:** —
