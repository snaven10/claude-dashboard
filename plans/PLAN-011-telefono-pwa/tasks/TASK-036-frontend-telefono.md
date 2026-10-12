# TASK-036 — Frontend: conectar teléfono, pantalla de emparejar y avisos push

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-031, TASK-034, TASK-035 · **PLAN-010 mergeado** (pisa `app.js`/`app.css`/`index.html`)
- **Estado:** `done`

---

## Objetivo
Desde el panel local se empareja un teléfono con un QR (o código) y se ven/revocan los
dispositivos; en el teléfono, la PWA se empareja, muestra el panel y activa los avisos push.

## Contexto verificado
- Design **DD-55** (los tres modos de la UI), DD-46/DD-47 (contrato de `/api/remote`, `/api/pair/*`,
  `/api/devices*`), DD-51/DD-53 (`/api/push/key`, `/api/push/subscribe`).
- `web/app.js` (versión de `main` antes de PLAN-010 — **releé tras el merge de PLAN-010**):
  `qrMatrix(url, factory)` y `drawQr(canvas, rows)` (DD-19, solo `addData/make/getModuleCount/isDark`);
  avisos dentro de la pestaña en `updateNotifyBtn`/`toggleNotify`/`fireNotifications` con el botón
  `#btn-notify` (no se cambian); `connect()` abre `EventSource('/api/events')`; lógica pura exportada
  para `node --test` en el bloque `// ---- Lógica pura`.
- Regla del repo: texto del server con `textContent`, nunca `innerHTML`; usable a 360 px y en oscuro.

## Archivos
- **Modificar:** `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js`, `web/fixture.json` (solo si hace falta un caso)

## Pasos
- [x] **Paso 1 — arranque.** Antes de `connect()`, `GET /api/remote` (si falla o 404 → modo local
  sin remoto, igual que hoy). Con `?fixture` no se llama.
- [x] **Paso 2 — local con remoto activo.** Botón "📱 conectar teléfono" en `.filters` → panel
  (mismo patrón de overlay que `#history`, `role=dialog`, Esc cierra, foco devuelto) con: QR de
  `url` dibujado con `qrMatrix`/`drawQr`, código `ABCD-EFGH`, cuenta regresiva hasta `expiresAt`
  (al vencer: "venció — generar otro"), pasos cortos (1 escaneá, 2 abrí, 3 agregá a inicio, 4
  activá avisos), y lista de dispositivos (nombre, alta, último uso) con "revocar" + `confirm()`.
- [x] **Paso 3 — remoto sin emparejar.** No se abre SSE. Pantalla con: si hay `#pair=` → canje
  automático y `history.replaceState` para borrar el fragmento; si no o si falla → campo de código
  + nombre del dispositivo. Al canjear OK → recargar el estado y seguir como emparejado.
- [x] **Paso 4 — remoto emparejado.** Registrar `sw.js` (si `serviceWorker` e `isSecureContext`).
  Botón "🔔 avisos en este teléfono": permiso dentro del gesto → `pushManager.subscribe` con la clave
  (b64url → `Uint8Array`) → `POST /api/push/subscribe`. Estado del botón: activados / bloqueados /
  no soportado. En iOS fuera de standalone: el texto de DD-55 en vez del botón.
- [x] **Paso 5 — deep link.** Si la URL trae `?s=<id>` (desde el aviso), seleccionar esa sesión al
  llegar el primer snapshot (validar el id con la misma regex de TASK-034).
- [x] **Paso 6 — 401 en remoto** (cookie revocada): volver a la pantalla de emparejar.
- [x] **Paso 7 — tests node** de la lógica pura.

## Criterios de aceptación
- [x] `node --test web/`: `uiMode({enabled,local,paired})` → `local-off` | `local-remote` | `pair` | `paired`;
  `pairTokenFromHash('#pair=abc')` → `abc` y `''` para hashes ajenos; `normalizeCode('abcd efgh')` → `ABCDEFGH`;
  `b64urlToBytes` con un vector conocido; `isIOSNeedsInstall(ua, standalone)` para iOS/Android/escritorio;
  `deepLinkSession('?s=…')` acepta ids válidos y rechaza `../x`, `<b>`.
- [x] `grep` en `web/app.js`: sin `innerHTML`, sin `createSvgTag|createImgTag|createTableTag`.
- [x] Con `?fixture`: el panel "conectar teléfono" se puede abrir con datos falsos (QR, código,
  2 dispositivos) y se ve en claro/oscuro a 1280 y 360 px sin scroll horizontal (verificación visual
  del orquestador si el worker no abre navegador: decirlo en `Verificado por`).
- [x] Sin modo remoto (`/api/remote` 404): el DOM es el de hoy (sin botón 📱 nuevo ni 🔔 push) — test node o comprobación con fixture.
- [x] `node --check web/app.js`, `node --test web/`, `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Riesgos
- Conflictos con PLAN-010 si se ejecuta antes de su merge: NO arrancar hasta que esté en `main`.
- `pushManager.subscribe` en iOS solo funciona en la PWA instalada y dentro de un gesto.

## Resultado
- **Estado final:** `done`
- **Resumen:** `web/app.js` arranca con `GET /api/remote` y decide el modo (`uiMode`): `local-off` (DOM de siempre), `local-remote` (botón "conectar teléfono" → panel con QR vía `qrMatrix`/`drawQr`, código `ABCD-EFGH`, cuenta regresiva, pasos y dispositivos con revocar+`confirm()`), `pair` (sin SSE; canje automático de `#pair=` con `history.replaceState` previo al envío, o campo de código + nombre) y `paired` (registra `sw.js`, botón de avisos push con permiso dentro del gesto, `pushManager.subscribe` con la clave b64url, `POST /api/push/subscribe`; en iOS fuera de standalone, el texto de DD-55). Deep link `?s=<id>` validado con la regex de TASK-034 y consumido en el primer snapshot. 401 en remoto (fetch o SSE cerrado + `/api/remote` paired:false) vuelve a la pantalla de emparejar. Todo DOM nuevo se crea desde JS solo con remoto activo; `index.html` no cambió.
- **Archivos tocados:** `web/app.js`, `web/app.css`, `web/app.test.js` (commit `7abc707`). `index.html` y `fixture.json` sin cambios.
- **Verificado por:** `node --check web/app.js` ok; `node --test web/` 96 pass / 0 fail (8 tests nuevos: uiMode, pairTokenFromHash, normalizeCode/formatCode, b64urlToBytes, isIOSNeedsInstall, deepLinkSession, pairCountdown, ausencia de innerHTML/createSvgTag|createImgTag|createTableTag); `gofmt -l .` vacío; `go vet ./...` ok; `go test -race ./...` todo ok. `grep innerHTML|createSvgTag|createImgTag|createTableTag web/app.js` sin resultados. Visual con chromium headless (iframe de ancho fijo, `?fixture`): panel abierto con QR, código, cuenta regresiva y 2 dispositivos; `scrollWidth == clientWidth` a 1280, 1000 y 360 px, con y sin remoto; pantalla de emparejar a 360 px vista en captura. NO verificado: modo oscuro (solo vía variables CSS existentes), flujo real contra el server con cookie, push real en teléfono (gate de TASK-038).
- **Desviaciones:** (1) El botón usa el ícono SVG propio `phone` (y `bell` nuevo) en vez de los emoji 📱/🔔: DD-42 los reemplazó porque se veían como ▯. (2) `.filters` pasa a `flex-wrap: wrap` en escritorio: con el botón nuevo la barra desbordaba 137 px a 1280; solo baja de línea cuando antes ya desbordaba. (3) Con `?fixture` se puede forzar el modo con `&remote=off|pair|paired` (default `local-remote`); los datos falsos están en `app.js`, no en `fixture.json`. (4) Se agregó `pairCountdown` y `formatCode` a la lógica pura (no pedidos, testeados).
- **Riesgos abiertos / siguiente:** el `Origin` que manda el navegador en el POST de canje debe coincidir con `-public-url` (DD-48): si `tailscale serve` cambia el host visible, el canje da 403 → verificar en TASK-038. En iOS, el almacén de cookies de la PWA puede ser distinto del de Safari: por eso el campo de código dentro de la PWA. Push solo se prueba en el gate del usuario.
