# PLAN-004 — Abrir una sesión en el teléfono (QR de Remote Control)

**Fecha:** 2026-10-08
**Fase:** Cerrado
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario: un comando o un QR para abrir la sesión desde el teléfono

## 1. Qué resuelve
Las sesiones con Remote Control activo ya se ven como "📡 remoto", pero no hay forma de abrirlas en
el teléfono desde el panel. Ahora: botón "📱 abrir en el teléfono" con QR y link.

## 2. Hallazgos (verificados 2026-10-08)
- **El link es `https://claude.ai/code/<bridgeSessionId>`.** Comprobado con esta misma sesión: su
  `bridgeSessionId` (F2) es `session_01PheFY7bAAutvpmHjzYFAFy`, el mismo id de su link de claude.ai.
  La doc oficial solo dice que el id es "the part of the session's URL at claude.ai/code between
  `/code/` and any `?`" → el formato `session_…` NO está documentado: se valida (DD-18).
- Activar Remote Control dentro de una sesión: **`/remote-control`** (alias **`/rc`**), según
  https://code.claude.com/docs/en/remote-control. Correrlo otra vez muestra el link y un QR.
- **Sesiones `--bg`: no documentado** si admiten Remote Control → el panel no promete nada para ellas.
- Setting `remoteControlAtStartup` (o `/config` → "Enable Remote Control for all sessions") activa
  Remote Control en toda sesión interactiva nueva. Es decisión del usuario; el panel solo lo menciona.
- `claude --resume` sobre una interactiva que sigue abierta abre una SEGUNDA copia de la conversación
  → el botón "copiar comando" lo advierte y ofrece el QR (DD-20).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-010** | `remoteUrl` validado + botón con QR (canvas) + indicación `/remote-control` + aviso en copiar | claude-dashboard-worker | sonnet | — | `done` |

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-010** | Una sola task: backend y frontend son chicos y van juntos |

### Decisiones
**DD-18 — `remoteUrl` solo si el id tiene el formato visto.** Backend: `bridgeSessionId` que cumpla
`^session_[A-Za-z0-9]{10,64}$` → `remoteUrl = "https://claude.ai/code/" + id`; si no, `null` (y
`remote` sigue como hoy). Nunca se arma una URL con un valor sin validar: va a un QR y a un `href`.

**DD-19 — QR local y sin HTML de la librería.** `web/vendor/qrcode.js` (qrcode-generator 1.4.4, MIT,
Kazuhiko Arase, sha256 `18ae399f…`, sin red/eval/innerHTML; copiado tal cual con su cabecera de
licencia). Se usa SOLO `qrcode(0,'M')`, `addData`, `make`, `getModuleCount`, `isDark` para pintar en
un `<canvas>`. Prohibido `createSvgTag`/`createImgTag`/`createTableTag` (devuelven HTML como string).
Nada sale de la máquina para dibujar el QR.

**DD-20 — Qué ve cada sesión.**
- Con `remoteUrl`: botón "📱 abrir en el teléfono" → panel con QR, link (`<a target=_blank rel=noopener>`)
  y botón copiar link.
- Interactiva sin `remoteUrl`: texto "activalo escribiendo `/remote-control` (o `/rc`) en esa sesión;
  el QR aparece solo en ~3 s".
- Background sin `remoteUrl`: nada sobre Remote Control (no documentado).
- "Copiar comando" en una interactiva NO terminada: aviso de que `--resume` abre una segunda copia; si
  hay `remoteUrl`, sugerir el QR.

## 4. Fuera de alcance
- Abrir el PANEL desde el teléfono (sigue en 127.0.0.1; exponerlo exige autenticación).
- Activar Remote Control desde el panel (solo lectura) o tocar `remoteControlAtStartup`.

## 5. Cierre
- **Cerrado:** 2026-10-08
- **Shipeó:** rama `main` (sin remoto), merge de TASK-010. Servicio reinstalado.
- **Verificación:** `go test -race` y 21 tests de node en `main`. QR de una sesión real decodificado con un decodificador independiente (jsQR) = su link exacto. 26/26 ids reales aceptados por DD-18. Casos de DD-20 vistos en el navegador. Sin bugs en la QA.
- **NO verificado:** escanear el QR con un teléfono real (lo hace el usuario); `/remote-control` en sesiones `--bg` (no documentado, por eso el panel no lo ofrece).
- **Quedó afuera:** lo de §4.
- **Continúa en:** —
