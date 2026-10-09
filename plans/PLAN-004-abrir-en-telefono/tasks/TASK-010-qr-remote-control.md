# TASK-010 — remoteUrl, QR para el teléfono e indicaciones de Remote Control

- **Plan:** PLAN-004 — Abrir una sesión en el teléfono
- **Especialista:** claude-dashboard-worker
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama `main`
- **Depende de:** —
- **Estado:** `done`

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
- **Estado final:** done (criterio visual sin ejecutar, ver abajo)
- **Resumen:** `Session.remoteUrl` (`*string`) sale de `bridgeSessionId` solo si cumple `^session_[A-Za-z0-9]{10,64}$` (DD-18); `remote` queda igual. Frontend: botón "📱 abrir en el teléfono" → panel con QR en `<canvas>` (fondo blanco fijo, margen de 4 módulos), link `target=_blank rel=noopener` y copiar link; interactiva sin link muestra `/remote-control` (`/rc`); background sin link no muestra nada; "copiar comando" en interactivas no terminadas avisa de la segunda copia y sugiere el QR. El cliente revalida la URL con regex antes de usarla en href/QR. Se usan solo `qrcode(0,'M')`, `addData`, `make`, `getModuleCount`, `isDark`. README documenta el flujo y menciona `remoteControlAtStartup`.
- **Archivos tocados:** `internal/agents/agents.go`, `internal/agents/agents_test.go`, `web/embed.go`, `web/index.html`, `web/app.js`, `web/app.css`, `web/app.test.js`, `web/fixture.json`, `README.md`. Commit de código: `e9e8443`.
- **Verificado por:** en el worktree: `gofmt -l .` (vacío), `go vet ./...` (ok), `go test -race ./...` (agents, orq, server, transcript: ok), `node --check web/app.js` (ok), `node --test web/` (21 pass, 0 fail). Cubren: `remoteUrl` válido/vacío/`/`/`?`/espacios/salto/corto/largo (Go); `phoneMode` por tipo de sesión, `validRemoteUrl`, `resumeWarning`, tamaño de la matriz == `getModuleCount()`, fixture con los 3 casos, y que `app.js` no contiene `createSvgTag`/`createImgTag`/`createTableTag`/`innerHTML` (JS). **NO verificado:** el criterio visual de `?fixture` (claro/oscuro, 1280 y 360 px, sin scroll horizontal): no abrí un navegador; el CSS usa `width:min(100%,320px)`, `overflow-wrap:anywhere` y fondo blanco fijo, pero falta mirarlo. Tampoco escaneé el QR con un teléfono.
