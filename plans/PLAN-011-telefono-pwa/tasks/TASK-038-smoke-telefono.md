# TASK-038 — Smoke end-to-end y gate del usuario con su teléfono

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** orquestador (lo automatizable) + **usuario (gate con el teléfono real)**
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), `main` con TASK-030..037 mergeadas
- **Depende de:** TASK-037
- **Estado:** `done`

---

## Objetivo
El flujo completo funciona en la máquina y el teléfono reales del usuario: QR → panel en el
teléfono → PWA instalada → avisos activados → push con la app cerrada cuando una sesión pasa a
"esperándote" → revocación corta el acceso.

## Contexto verificado
- Master §2: Tailscale corre en WSL y está `Running`, pero HTTPS de la tailnet no está activado y
  no hay teléfono en la tailnet: los pasos 1-2 son del usuario.
- README (TASK-037) tiene la guía paso a paso; este smoke la sigue tal cual (también la valida).

## Archivos
- Ninguno de código. Solo se llena `## Resultado` de esta task y `§8 Cierre` del master.

## Pasos
- [ ] **Paso 1 — usuario: Tailscale.** Activar HTTPS Certificates en la consola; instalar Tailscale
  en el teléfono con la misma cuenta; correr `tailscale serve --bg --https=443 http://127.0.0.1:7778`.
- [ ] **Paso 2 — orquestador (con permiso del usuario para reinstalar el servicio):** override de
  `ExecStart` con `-public-url`, `make install`; comprobar en la PC: `curl` a `127.0.0.1:7777/api/sessions`
  → 200; a `127.0.0.1:7778/api/sessions` sin cookie → 401; a `/api/remote` por el 7778 → `paired:false`;
  `~/.config/claude-dashboard/` en 0700 y sus archivos en 0600.
- [ ] **Paso 3 — usuario: Android y/o iOS.** Escanear el QR del panel local; ver el panel en el
  teléfono; agregar a pantalla de inicio; abrir la PWA (anotar si en iOS pidió el código: resuelve
  la duda de cookies de DD-47); activar avisos.
- [ ] **Paso 4 — push real.** Con la PWA cerrada y el teléfono bloqueado, provocar una sesión que
  pase a "esperándote" (una sesión de prueba que haga una pregunta). Anotar latencia aproximada y
  que el texto del aviso corresponde al modo de `-push-content`. Tocar el aviso → abre la PWA en esa sesión.
- [ ] **Paso 5 — SSE por `serve`.** Con la PWA abierta, el panel se actualiza solo (sin recargar)
  cuando cambia una sesión. Si no, activar el plan B del master §6.
- [ ] **Paso 6 — negativos.** Desde otro dispositivo de la tailnet NO emparejado: la URL muestra
  la pantalla de emparejar y `/api/sessions` da 401. Reusar un QR ya canjeado → falla. Revocar el
  teléfono desde la PC → la PWA vuelve a la pantalla de emparejar y no llegan más push.
- [ ] **Paso 7 — sin flag.** Quitar `-public-url`, reinstalar: el panel local es el de antes,
  `127.0.0.1:7778` no escucha, `/api/pair/new` → 404. `tailscale serve reset` si el usuario no lo
  quiere dejar.

## Criterios de aceptación
- [ ] Pasos 2-7 con resultado anotado (ok / falla con detalle). **Gate del usuario:** los pasos 3-6
  los confirma el usuario con su teléfono; sin esa confirmación el PLAN no se cierra.
- [ ] Ningún dato real (URL de la tailnet, nombre de la PC, capturas con prompts) entra al repo:
  el `Resultado` usa los ejemplos genéricos.

## Resultado
- **Estado final:** done (2026-10-11), con el usuario y su teléfono Android.
- **Resumen:** el flujo completo anda en un teléfono real: QR → panel emparejado → PWA instalada → avisos push activados → aviso recibido con el teléfono bloqueado.
- **Archivos tocados:** ninguno del código en esta task. Fuera del repo: override `~/.config/systemd/user/claude-dashboard.service.d/remoto.conf` (`-public-url`) y `tailscale serve --bg --https=443 http://127.0.0.1:7778` (tailnet only).
- **Verificado por:**
  - Paso 1: HTTPS de la tailnet activado y Tailscale en el teléfono (usuario). `serve` en modo "tailnet only".
  - Paso 2: local `/` y `/api/sessions` → 200; `:7778/api/sessions` sin cookie → 401; `:7778/api/remote` → `{"enabled":true,"local":false,"paired":false}`; `POST :7778/api/pair/new` → 404; por la URL de la tailnet `/api/sessions` → 401; `~/.config/claude-dashboard` 0700, archivos 0600.
  - Paso 3 (Android, Chrome): QR escaneado, canje OK (sin 403 de Origin), PWA instalada, avisos activados; `subscriptions.json` con 1 suscripción contra el servicio de push de Google.
  - Paso 4: con el teléfono bloqueado, una sesión pasó a "esperándote" (pregunta de prueba) y el aviso llegó (confirmado por el usuario).
  - Paso 6 (parcial): dispositivo de prueba emparejado desde Chromium por la URL de la tailnet → ve el panel; revocado → 204 y fuera de la lista. Sin cookie → 401.
  - Hallazgos del gate → TASK-039 (vista teléfono) y renombre de la máquina en Tailscale (la URL dejaba ver el nombre de un cliente; ahora es un nombre genérico). Tras renombrar hubo que rehacer `tailscale serve` (queda atado al nombre viejo) y re-emparejar.
- **No verificado:** iOS; tocar el aviso → abre la sesión (no confirmado); latencia exacta (el servicio no loguea envíos exitosos); SSE por `serve` medido (el panel cargó con datos, sin medir actualizaciones en vivo); reusar un QR ya canjeado; Paso 7 (el usuario deja el modo remoto activo).
- **Riesgos abiertos / siguiente:** si WSL apaga la VM no hay panel ni avisos (receta en el README); el botón de avisos quedó dentro de "Filtros" en el teléfono.
