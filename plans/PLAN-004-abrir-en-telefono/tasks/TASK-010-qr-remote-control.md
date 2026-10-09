# TASK-010 — remoteUrl, QR para el teléfono e indicaciones de Remote Control

- **Plan:** PLAN-004 — Abrir una sesión en el teléfono
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** —
- **Estado:** `pending`

---

## Objetivo
Cada sesión con Remote Control trae `remoteUrl` y la UI ofrece un QR para abrirla en el teléfono (DD-18..DD-20 del master).

## Contexto verificado
- Leé el master entero, sobre todo §2 y DD-18..DD-20.
- `bridgeSessionId` llega por F2 (`~/.claude/sessions/<pid>.json`) y hoy solo alimenta `remote: bool` en `internal/agents/agents.go`.
- `web/vendor/qrcode.js` YA está en el repo (no lo descargues ni lo modifiques). API: `var qr = qrcode(0, 'M'); qr.addData(url); qr.make(); qr.getModuleCount(); qr.isDark(fila, col)`.
- `web/embed.go` lista los archivos a mano: agregá `vendor/qrcode.js`.

## Archivos
- **Modificar:** `internal/agents/agents.go`, `internal/agents/agents_test.go`, `web/embed.go`, `web/index.html`, `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json`, `README.md`

## Pasos
- [ ] **Paso 1** — Backend: `remoteUrl` con la validación de DD-18 (+ tests: válido, vacío, con `/`, con `?`, con espacios, demasiado largo).
- [ ] **Paso 2** — Embed y `<script src="vendor/qrcode.js">` antes de `app.js`.
- [ ] **Paso 3** — Frontend DD-20: botón y panel con QR en `<canvas>` (margen blanco de 4 módulos, legible en modo oscuro: fondo blanco SIEMPRE), link y copiar; texto `/remote-control` para interactivas sin link; aviso en "copiar comando".
- [ ] **Paso 4** — Fixture: una sesión con `remoteUrl`, una interactiva sin él, una background sin él.
- [ ] **Paso 5** — README: cómo abrir una sesión en el teléfono y el setting `remoteControlAtStartup` (solo mencionarlo).

## Criterios de aceptación
- [ ] Test Go: `remoteUrl` solo para ids válidos; cualquier otro valor → `null`.
- [ ] `node --test`: qué ve cada tipo de sesión (DD-20) y que la matriz del QR tenga el tamaño de `getModuleCount()`.
- [ ] `grep` en `web/app.js`: ninguna llamada a `createSvgTag`/`createImgTag`/`createTableTag` ni `innerHTML`.
- [ ] Con `?fixture`: el panel del QR se ve en claro y oscuro, a 1280 y 360 px, sin scroll horizontal.
- [ ] `gofmt`, `go vet`, `go test -race ./...`, `node --check web/app.js`, `node --test web/` en verde y pegados en `Verificado por`.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
