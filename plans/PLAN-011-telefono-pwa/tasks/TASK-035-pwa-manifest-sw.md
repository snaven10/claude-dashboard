# TASK-035 — PWA: manifest, íconos y service worker

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-030 (headers de `sw.js` y manifest) · **blanda:** PLAN-010 si este toca `web/index.html` o `web/embed.go`
- **Estado:** `done`

---

## Objetivo
El panel es instalable como PWA ("agregar a pantalla de inicio") en Android e iOS y tiene un
service worker que recibe push y muestra el aviso con la app cerrada, sin cachear datos de sesiones.

## Contexto verificado
- Design **DD-54** (manifest, íconos, reglas del SW). DD-53 (formato del payload que el SW parsea).
- `web/embed.go`: `//go:embed index.html app.js app.css fixture.json fixture-timeline.json fixture-history.json vendor/qrcode.js`.
- `web/index.html`: `<head>` con favicon SVG inline (círculo `#d97757`), `app.css`; al final
  `vendor/qrcode.js` y `app.js`.
- Tras TASK-030 el server ya sirve `*.webmanifest` con su `Content-Type` y `/sw.js` con `no-cache`.
- Esta task **no toca** `web/app.js` ni `web/app.css` (el registro del SW lo hace TASK-036).

## Archivos
- **Crear:** `web/manifest.webmanifest`, `web/sw.js`, `web/sw.test.js`, `web/icons/icon-192.png`, `web/icons/icon-512.png`, `web/icons/icon-maskable-512.png`, `web/icons/apple-touch-icon.png`
- **Modificar:** `web/embed.go`, `web/index.html` (solo `<head>`)

## Pasos
- [ ] **Paso 1 — íconos.** Programa Go descartable en el scratchpad (NO en el repo) con `image`,
  `image/color`, `image/png`: círculo `#d97757` sobre transparente (192/512/180) y sobre fondo
  sólido con margen de seguridad del 20 % (maskable). Versionar solo los PNG; anotar en `Resultado`
  cómo se generaron.
- [ ] **Paso 2 — manifest** según DD-54.
- [ ] **Paso 3 — `sw.js`.** Vanilla, sin dependencias. `install`: precache del shell en
  `cd-shell-v1`; `activate`: borra cachés `cd-shell-*` viejas y `clients.claim()`; `fetch`: ignora
  todo lo que no sea GET mismo origen, **no intercepta `/api/*` ni `fixture*.json`**, shell
  network-first con caída a caché. `push`: `event.waitUntil(showNotification(...))` SIEMPRE, con
  payload genérico si el JSON no parsea. `notificationclick`: `url` solo si empieza con `/` y no
  con `//`; enfoca ventana existente o `clients.openWindow`. Lógica pura (parseo de payload,
  validación de url, decisión de cachear) en funciones exportables para `node --test` (patrón de
  `app.js`: `if (typeof module !== 'undefined') module.exports = …`).
- [ ] **Paso 4 — `index.html`.** `<link rel="manifest" href="manifest.webmanifest">`,
  `<link rel="apple-touch-icon" href="icons/apple-touch-icon.png">`, `<meta name="theme-color">`
  (claro/oscuro con `media`), `<meta name="apple-mobile-web-app-capable" content="yes">`.
- [ ] **Paso 5 — embed.** Agregar `manifest.webmanifest sw.js icons/*.png` al `//go:embed`
  (`sw.test.js` NO).

## Criterios de aceptación
- [ ] `node --test web/`: `shouldHandle('/api/sessions')` y `/api/events` y `fixture.json` → false;
  `/`, `/app.js` → true; payload inválido → aviso genérico; `url` `//evil.example` o
  `https://evil.example` → `/`; `url` `/?s=abc` se respeta.
- [ ] `grep` en `web/sw.js`: no aparece `/api` en la lista de precache ni `cache.put` para rutas `/api`.
- [ ] Test Go (en `server_test.go` existente o `web`): `GET /manifest.webmanifest` 200 JSON válido con
  `display: standalone` y 3 íconos que existen en el FS embebido; `GET /sw.js` 200 con `no-cache`;
  los 4 PNG decodifican con `image/png` y tienen el tamaño declarado.
- [ ] `node --check web/sw.js`, `gofmt`, `go vet`, `go test -race ./...`, `node --test web/` en verde y pegados.
- [ ] No verificado acá (va a TASK-038): instalación real en Android/iOS.

## Riesgos
- iOS ignora parte del manifest y usa `apple-touch-icon` y las metas `apple-*`: por eso van las dos cosas.

## Resultado
- **Estado final:** `done`
- **Resumen:** El panel es instalable como PWA: manifest (standalone, 3 íconos), 4 PNG, `sw.js` con
  precache del shell `cd-shell-v1`, network-first, push que SIEMPRE muestra aviso (genérico si el
  JSON no parsea) y `notificationclick` que solo acepta `url` que empieza con `/` y no con `//`.
  `/api/*` y `fixture*.json` no se interceptan. El registro del SW queda para TASK-036.
- **Archivos tocados:** commit `a17178b` — creados `web/manifest.webmanifest`, `web/sw.js`,
  `web/sw.test.js`, `web/embed_test.go`, `web/icons/{icon-192,icon-512,icon-maskable-512,apple-touch-icon}.png`;
  modificados `web/embed.go`, `web/index.html` (solo `<head>`), `internal/server/server_test.go`
  (test con el FS real embebido).
- **Verificado por:** en el worktree: `gofmt -l .` vacío; `go vet ./...` limpio;
  `go test -race ./...` todo `ok` (agents, history, orq, plans, remote, server, transcript, webpush, web);
  `node --check web/sw.js` ok; `node --test web/` 88 pass / 0 fail. `grep` en `sw.js`: no hay
  `cache.put` ni `/api` en el precache (solo el guard de `shouldHandle` y comentarios).
- **Desviaciones:** (1) `embed_test.go` en `web/` además del test en `server_test.go` (cubre
  tamaño de PNG y que `sw.test.js` no se embebe). (2) `shouldHandle(url, method?, origin?)`: la
  task lo invoca con un solo argumento. (3) Bloqueé también `/\` en `safeUrl` y excluí `/sw.js`
  del fetch handler. (4) Maskable y apple-touch con fondo sólido `#121418` (iOS pinta la
  transparencia de negro).
- **Íconos:** generados con un programa Go descartable (`image`, `image/png`, supermuestreo 4x4)
  en `/tmp/cd-icons-gen`, fuera del repo: círculo `#d97757`; 192/512 transparentes (radio 0.875,
  como el favicon), maskable 512 con círculo al 60 % del lado sobre `#121418`, apple 180 al 75 %.
- **Riesgos abiertos / siguiente:** instalación real en Android/iOS NO verificada (TASK-038).
  `theme_color` del manifest es fijo claro (`#f4f5f7`); el `<meta>` sí varía por esquema.
  TASK-036 debe registrar el SW (`serviceWorker` + `isSecureContext`). PLAN-010 ya estaba
  mergeado: sin conflicto en `index.html`.
