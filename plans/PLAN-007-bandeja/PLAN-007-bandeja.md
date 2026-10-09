# PLAN-007 — Interfaz tipo bandeja: te esperan, lista compacta, detalle y abandonadas

**Fecha:** 2026-10-09
**Fase:** Aprobado — en ejecución
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** auditoría de la interfaz con datos reales + maqueta aprobada por el usuario ("se ve bien podes seguir")
**Referencia visual:** [`maqueta.html`](./maqueta.html) (datos de ejemplo; también publicada como artifact privado)

## 1. Qué resuelve
| # | Problema medido (2026-10-08, 39 sesiones reales) | Cambio |
|---|---|---|
| 1 | 4 de 5 "esperándote" son sesiones de fondo `blocked` con 89-107 días sin actividad: inflan contador, favicon y avisos | **Abandonadas** (DD-30) |
| 2 | Tarjetas de ~510 px (máx 599); entran 4 de 39 en pantalla; la página mide 5.346 px | **Lista compacta + detalle** (DD-31) |
| 3 | Lo urgente pesa igual que una idle de hace una semana | **Franja "Te esperan"** (DD-32) |
| 4 | El aviso de `--resume` y la ayuda de `/remote-control` repetidos en 18 tarjetas | Ayuda una sola vez, en el detalle (DD-31) |
| 5 | Markdown crudo (`**…**`) y comandos Bash de 6 líneas | Texto limpio y en una línea (DD-33) |
| 6 | Solo mouse | Teclado (DD-34) |

## 2. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-017** | Frontend: bandeja (abandonadas, franja, lista + detalle, limpieza, teclado, teléfono) | claude-dashboard-worker | sonnet | — | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-017** | Solo `web/`. Corre en paralelo con TASK-018 (PLAN-008, solo backend) |

### Decisiones
**DD-30 — Abandonadas: presentación, nunca borrado.** Una sesión `waiting`/`blocked` cuya última
actividad (`lastActivityAt`, o `startedAt` si no hay) tiene más de 24 h es "abandonada": NO cuenta en
el contador, el favicon, el título de la pestaña, los avisos ni la franja. Se lista colapsada al final
("N abandonadas: esperan hace más de 24 h. No cuentan…"). El panel no ofrece borrarlas (DD-15 sigue).

**DD-31 — Lista compacta + detalle.** Una fila por sesión: punto de estado, título, una línea
secundaria (proyecto · rama, o el `ask` corto en idle), chips de alertas (errores, ritmo, contexto,
sin actividad) y a la derecha antigüedad + mini barra de contexto. Agrupación por plan con avance como
hoy. Terminadas viejas colapsadas como hoy. Al seleccionar una fila, el panel de detalle (derecha,
`position: sticky`) muestra TODO lo que hoy está en la tarjeta: meta, `ask`, último prompt, última
acción, tokens, contexto, ritmo, errores, orq, línea de tiempo (la del endpoint actual, cargada al
seleccionar), acciones (copiar comando con su aviso, QR del teléfono en línea). La ayuda de
`--resume`/`/remote-control` aparece UNA vez, en el detalle. La selección se recuerda (localStorage,
con try/catch) y sobrevive a cada snapshot SSE sin saltar el scroll. Teléfono (≤ 760 px): lista y
detalle en pantallas separadas con "← volver". Las tarjetas actuales desaparecen.

**DD-32 — Franja "Te esperan".** Arriba, solo `waiting`/`blocked` no abandonadas: título, proyecto ·
rama, antigüedad, `ask` destacado, "ver detalle" y "📱" si hay `remoteUrl`. Sin ninguna: "Nadie te
espera". Al lado de los contadores: "hoy X output · N sesiones" del último día de `/api/history`
(oculto si `ready:false`), recargado cada 60 s.

**DD-33 — Texto limpio.** Función pura `plainText`: saca `**`, `__`, backticks, `#` iniciales y
deja `[texto](url)` como `texto`, colapsa espacios. Se aplica a `ask`, `lastPrompt` y título. Los
comandos (`lastAction`) van en una línea con elipsis y el texto completo en `title`.

**DD-34 — Teclado.** `j`/`k` y flechas mueven la selección, `Enter` abre el detalle (en teléfono
cambia de pantalla), `Esc` vuelve / cierra paneles, `1`/`2`/`3` filtran por estado (toggle), `/`
busca. Nada se dispara mientras el foco está en un input. Indicación visible en escritorio.

## 3. Fuera de alcance
- Los datos nuevos (resumen "mientras no estabas", PRs, subagentes, líneas, turno, permisos): PLAN-008.
- Borrar o archivar sesiones.

## 4. Cierre
