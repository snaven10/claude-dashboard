# PLAN-011 — Diseño técnico: panel en el teléfono (Tailscale + PWA + Web Push)

> Fuente de verdad de las decisiones DD-43..DD-55. Las tasks las citan por id.
> Numeración: PLAN-010 usa DD-37..DD-42; este plan, DD-43..DD-55.

## 1. Arquitectura

```
 teléfono (PWA instalada, en la tailnet)
     │  HTTPS  https://mi-pc.tailnet-ejemplo.ts.net   (cert de Tailscale, solo tailnet)
     ▼
 tailscaled en WSL  ──  `tailscale serve`  ──►  127.0.0.1:7778   listener REMOTO (exige cookie)
                                                127.0.0.1:7777   listener LOCAL  (como hoy)
                                                     │
                                         claude-dashboard (un proceso, un poller)
                                                     │  POST cifrado (aes128gcm) + VAPID
                                                     ▼
                                   servicio de push del navegador (Apple / Google / Mozilla)
                                                     │
                                                     ▼  push al teléfono, aunque la PWA esté cerrada
```

El daemon **nunca** escucha fuera de loopback. Lo único que llega de afuera es lo que
`tailscale serve` le reenvía, y solo desde la tailnet privada del usuario.

## 2. Decisiones

### DD-43 — Dos listeners loopback; el listener decide quién es remoto
Con `-public-url` el daemon abre un **segundo** listener, `-remote-addr` (default
`127.0.0.1:7778`), y `tailscale serve` apunta SOLO a ese puerto. Cada handler sabe por qué
listener entró la petición (valor en el `context` puesto por `http.Server.BaseContext` /
`ConnContext` de cada listener, o dos `http.Handler` envolviendo el mismo mux).

**Por qué no detectar por cabeceras o por `RemoteAddr`:** `tailscale serve` es un reverse proxy
local, así que TODA petición proxyada llega con `RemoteAddr` 127.0.0.1 — indistinguible del
navegador de escritorio. `Host` y `X-Forwarded-*` los puede poner cualquiera. Las cabeceras
`Tailscale-User-Login`/`-Name` las agrega `serve` para usuarios de la tailnet (no para nodos con
tag), y la doc de Tailscale dice que `serve` descarta las que mande el cliente; aun así son una
señal de identidad, no de ORIGEN: un proceso local puede mandarlas al 7777. Separar por listener
no depende de nada falsificable: lo que entra por el 7778 es remoto por construcción.

- `-remote-addr` se valida con la misma regla que `-addr` pero **sin** escape: tiene que ser
  loopback siempre (`-allow-remote` no lo afecta).
- `Tailscale-User-Login`, si viene, se guarda en el registro del dispositivo al emparejar (para
  que la lista diga de quién es). No se usa para autorizar.

### DD-44 — Modo remoto opt-in; sin flag, idéntico a hoy
Se activa con `-public-url https://<nombre>.ts.net` (tiene que ser `https://`, sin path, sin
query; se normaliza sin `/` final). Sin el flag:
- no se abre el segundo listener, no se crea el directorio de estado, no existen las rutas
  `/api/pair*`, `/api/devices*`, `/api/push*`, `/api/remote` (404 como cualquier ruta inexistente);
- el poller no ejecuta el notificador.
Los archivos estáticos de la PWA (manifest, íconos, `sw.js`) se sirven siempre: no tienen datos.

Flags nuevos: `-public-url`, `-remote-addr`, `-state-dir`, `-push-content`, `-push-cooldown`.

### DD-45 — Estado en `~/.config/claude-dashboard/` (0700 / 0600), nunca en `~/.claude/`
Directorio = `-state-dir`, default `filepath.Join(os.UserConfigDir(), "claude-dashboard")`.
Se crea con `0700`; cada archivo con `0600`. Escritura atómica: temp en el mismo dir + `fsync` +
`rename`. Al leer, si un archivo tiene permisos más abiertos que `0600`, se loguea un aviso y se
corrigen con `chmod` (no se aborta).

| Archivo | Contenido |
|---|---|
| `vapid.json` | clave privada ECDSA P-256 (32 bytes, b64url) + pública sin comprimir (65 bytes, b64url) + `createdAt` |
| `devices.json` | `[{id, name, login, cookieHash, createdAt, lastSeenAt}]` — `cookieHash` = SHA-256 de la cookie; la cookie en claro nunca se guarda |
| `subscriptions.json` | `[{deviceId, endpoint, p256dh, auth, createdAt}]` |

Esto cumple la promesa "no escribe en `~/.claude/`".

### DD-46 — Emparejamiento: token de un solo uso + código corto, canjeados por cookie de dispositivo
1. En el panel LOCAL, "📱 conectar teléfono" hace `POST /api/pair/new` (solo listener local).
2. El server genera:
   - `token`: 32 bytes de `crypto/rand`, b64url. Va en el QR como
     `<public-url>/#pair=<token>` — en el **fragmento**, para que no viaje en la línea de petición,
     ni en logs del proxy, ni en `Referer`.
   - `code`: 8 caracteres de un alfabeto sin ambiguos (Crockford base32, 40 bits), mostrado
     debajo del QR como `ABCD-EFGH`, para teclearlo a mano (caso iOS de DD-47).
   - Los dos vencen a los **5 minutos** y comparten un solo uso: canjear uno invalida el otro.
     Hay **un** emparejamiento pendiente a la vez: pedir uno nuevo invalida el anterior.
3. El teléfono (por el listener remoto) hace `POST /api/pair/redeem` con `{token}` o `{code}`
   y `{name}` (texto libre ≤ 40, default derivado del User-Agent).
4. Si es válido: se crea el dispositivo, se responde `Set-Cookie: cd_device=<32 bytes b64url>;
   Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=7776000` (90 días, decisión del usuario 2026-10-11; al vencer se re-empareja con el QR).
5. Comparación de token/código/cookie con `subtle.ConstantTimeCompare` sobre hashes.
6. **Límite de intentos:** 5 canjes fallidos invalidan el pendiente (hay que generar otro desde el
   panel local). Con 40 bits, 5 minutos y 5 intentos, adivinar el código es despreciable.
7. Estado del pendiente: solo en memoria (reiniciar el daemon lo anula; está bien).

### DD-47 — Qué exige cada listener
| Ruta | Local (7777) | Remoto (7778) |
|---|---|---|
| estáticos (`/`, `app.js`, `sw.js`, manifest, íconos, vendor) | sí | sí, sin cookie (no tienen datos) |
| `GET /api/sessions`, `/api/events`, `/api/sessions/{id}/timeline`, `/api/history` | sí, sin cookie (como hoy) | **cookie válida** o 401 |
| `GET /api/remote` | sí | sí, sin cookie (solo dice `{enabled, local, paired}`) |
| `POST /api/pair/new`, `GET /api/devices`, `DELETE /api/devices/{id}` | sí | **404** (no existen en remoto) |
| `POST /api/pair/redeem` | 404 | sí (con límite de DD-46) |
| `GET /api/push/key`, `POST /api/push/subscribe`, `POST /api/push/unsubscribe` | sí (escritorio, `deviceId:"local"`) | cookie válida o 401 |

La cookie válida actualiza `lastSeenAt` del dispositivo (a lo sumo 1 escritura por minuto por
dispositivo, para no reescribir el archivo en cada SSE).

**iOS:** la PWA instalada en la pantalla de inicio puede tener su propio almacén de cookies,
separado del de Safari. Por eso la PWA, si arranca sin cookie, muestra la pantalla de
emparejamiento con el campo del código: el usuario empareja **dentro** de la PWA. No está
verificado qué versión de iOS copia las cookies al instalar: se resuelve en el gate de TASK-038.

### DD-48 — Anti-CSRF en las rutas que cambian estado
Toda ruta `POST`/`DELETE` nueva exige `Content-Type: application/json` (fuerza preflight CORS en
un ataque cross-site) y, si viene `Origin`, que coincida con el origen del listener
(`http://127.0.0.1:<puerto>` / `http://localhost:<puerto>` en el local, `-public-url` en el
remoto). Si viene `Sec-Fetch-Site` y no es `same-origin` ni `none`, 403. No se agrega CORS.

### DD-49 — Revocación
`DELETE /api/devices/{id}` (solo local) borra el dispositivo y **todas** sus suscripciones push.
Su cookie deja de valer en la siguiente petición; una conexión SSE abierta de ese dispositivo se
corta (el server guarda `deviceId` por suscriptor SSE remoto y la cierra al revocar). No toca
ninguna sesión de Claude Code. La UI pide confirmación ("este teléfono deja de ver el panel").

### DD-50 — Web Push en stdlib: cifrado RFC 8291 + RFC 8188 (aes128gcm)
Paquete `internal/webpush`. Go del repo: `go 1.27` en `go.mod` → `crypto/ecdh` (1.20),
`crypto/hkdf` (1.24), `crypto/ecdsa` con `PublicKey.Bytes`/`ParseUncompressedPublicKey` (1.25)
están todos en la stdlib. Cero dependencias.

Algoritmo (RFC 8291 §3.4 + RFC 8188 §2):
```
ua_public  = p256dh de la suscripción (65 bytes, se valida con ecdh.P256().NewPublicKey)
auth       = auth de la suscripción (16 bytes, si no son 16 → error)
as_priv    = ecdh.P256().GenerateKey(rand)          # NUEVO por mensaje
salt       = 16 bytes de crypto/rand                # NUEVO por mensaje
ecdh_secret = as_priv.ECDH(ua_public)
key_info   = "WebPush: info" || 0x00 || ua_public || as_public
IKM        = HKDF(salt=auth, ikm=ecdh_secret, info=key_info, L=32)
CEK        = HKDF(salt=salt, ikm=IKM, info="Content-Encoding: aes128gcm" || 0x00, L=16)
NONCE      = HKDF(salt=salt, ikm=IKM, info="Content-Encoding: nonce" || 0x00, L=12)
record     = AES-128-GCM(CEK, NONCE, plaintext || 0x02)      # un solo registro, delimitador 0x02 = último
body       = salt(16) || rs(4 bytes BE = 4096) || idlen(1 = 65) || as_public(65) || record
```
- Firma interna testeable: `encrypt(plaintext, uaPublic, auth []byte, salt []byte, asPriv *ecdh.PrivateKey) ([]byte, error)`;
  la pública `Encrypt(plaintext, sub)` genera `salt` y `asPriv` y llama a la interna.
- Tope: plaintext ≤ 3993 bytes (4096 − 86 de cabecera − 16 de tag − 1 de delimitador); más → error
  (el notificador ya trunca antes, DD-53).
- Test con el ejemplo del **RFC 8291 §5 / Apéndice A** (claves, salt, plaintext
  "When I grow up, I want to be a watermelon" y cuerpo resultante conocidos): `encrypt` con esas
  entradas tiene que dar el cuerpo exacto byte a byte, y las claves intermedias (IKM, CEK, NONCE)
  también. Los valores se copian **literalmente** del RFC (rfc-editor.org), no de memoria.

### DD-51 — VAPID (RFC 8292) con ES256 en stdlib
- Clave: `ecdsa.GenerateKey(elliptic.P256(), rand)` la primera vez que el modo remoto arranca;
  se guarda en `vapid.json` (DD-45) y se reusa siempre (cambiarla invalida todas las suscripciones).
- JWT: header `{"typ":"JWT","alg":"ES256"}`, claims `{"aud": <scheme://host del endpoint>,
  "exp": now+12h, "sub": <-public-url>}`. `sub` = la URL pública (`https://…`): Apple rechaza
  `sub` inválidos o `localhost`, y no hace falta pedir un mail. Firma: `ecdsa.Sign` → **r||s de
  32+32 bytes** (NO DER: `ecdsa.SignASN1` está mal para JWS). b64url sin padding.
- Cache del JWT por `aud` hasta 1 h antes de vencer.
- Petición: `POST <endpoint>` con `Authorization: vapid t=<jwt>, k=<pública b64url>`,
  `Content-Encoding: aes128gcm`, `Content-Type: application/octet-stream`, `TTL: 600`,
  `Urgency: high`, `Topic: <22 chars b64url del SHA-256(sessionId)>` (un aviso nuevo de la misma
  sesión reemplaza al no entregado).
- Respuestas: 201/202 ok; **404/410** → borrar la suscripción; **413** → log (bug de tamaño);
  **429/5xx** → no reintentar ese aviso (respetar `Retry-After` para ese host: no enviar antes);
  otros → log. `http.Client{Timeout: 10s}`; los envíos van en una goroutine con cola acotada
  (64) y nunca bloquean el poller; si la cola está llena se descarta y se loguea.
- El endpoint de la suscripción se valida al suscribir: `https://`, host no vacío, longitud ≤ 1024.
  No se valida contra una lista de hosts (cambian por navegador), pero se rechazan hosts que sean
  IP literal o loopback (evita usar el daemon para pegarle a la red local, SSRF).

### DD-52 — Disparador: transición a "esperándote" en el poller
Misma semántica que los avisos del navegador (`notifyTargets` + DD-30 en `web/app.js`), ahora en
Go, en `internal/server` (o `internal/notify`), invocado desde `Refresh` en **cada** ciclo
(antes del corte por hash, porque el antirrebote necesita ver ciclos sin cambios):
- **waiting** = `status ∈ {waiting, blocked}`.
- **Primer ciclo tras arrancar:** solo registra estados, no avisa (igual que DD-12 en el front).
- **Antirrebote:** una sesión avisa cuando se la ve waiting en **2 ciclos seguidos** (~3-6 s) y
  el ciclo anterior a esos no era waiting. Evita avisos por estados de un ciclo.
- **Cooldown por sesión:** `-push-cooldown` (default `2m`): si avisó hace menos, no vuelve a
  avisar aunque haya salido y vuelto a waiting.
- **Abandonadas no avisan:** `lastActivityAt` (o `startedAt`) de hace más de 24 h (DD-30).
- **Agrupar:** si en un mismo ciclo avisan más de 3 sesiones, sale UN aviso "N sesiones te
  esperan" en vez de N.
- Sesiones que desaparecen del snapshot se olvidan (sin crecer memoria).
- Sin suscripciones, no hace nada (no genera JWT ni cifra).

### DD-53 — Contenido del aviso: `-push-content`
El payload SIEMPRE va cifrado de punta a punta (DD-50): el servicio de push de Apple/Google ve
el destino, el tamaño y la hora, nunca el texto.

| Modo | `title` | `body` |
|---|---|---|
| `full` | título de la sesión (o nombre) + " — esperándote" | `proyecto: waitingFor` o `ask.text` truncado a 180 runas |
| `title` | igual que `full` | `proyecto` solamente |
| `none` | "claude-dashboard" | "Una sesión te espera" / "N sesiones te esperan" |

Payload JSON: `{"v":1,"title":…,"body":…,"tag":"cd-<hash corto de la sesión>","url":"/?s=<sessionId>"}`.
En `none` el `url` es `/` y el `tag` es fijo. Default `title` (decisión del usuario 2026-10-11).

### DD-54 — PWA
- `web/manifest.webmanifest`: `name` "Claude Dashboard", `short_name` "Claude", `start_url` "/",
  `scope` "/", `display` "standalone", `background_color`/`theme_color` del tema, íconos
  PNG 192 y 512 (`purpose: any`) + 512 `maskable`. Servido con
  `Content-Type: application/manifest+json`.
- `web/icons/`: `icon-192.png`, `icon-512.png`, `icon-maskable-512.png`, `apple-touch-icon.png`
  (180). Mismo dibujo que el favicon (círculo naranja). Se generan UNA vez con un programa Go
  descartable (stdlib `image/png`) fuera del repo; se versionan los PNG.
- `index.html`: `<link rel="manifest">`, `<link rel="apple-touch-icon">`,
  `<meta name="theme-color">`, `<meta name="apple-mobile-web-app-capable" content="yes">`.
- `web/sw.js`, servido en `/sw.js` (scope `/`) con `Cache-Control: no-cache`:
  - **Precache solo del shell** (`/`, `app.js`, `app.css`, `vendor/qrcode.js`, manifest, íconos)
    en una caché con versión (`cd-shell-v<N>`); al activar borra las versiones viejas.
  - **Nunca** cachea `/api/*` ni los fixtures: `fetch` de `/api/*` va directo a la red (el SW no
    lo intercepta). Los datos de sesiones no quedan en disco del teléfono más allá de lo que el
    navegador haga con la memoria de la página.
  - Shell con estrategia **network-first** (si hay red, la versión nueva; sin red, la cacheada
    para mostrar "sin conexión a la tailnet").
  - `push`: parsea el JSON (si falla, aviso genérico) y llama SIEMPRE a
    `self.registration.showNotification` (iOS revoca la suscripción si un push no muestra aviso).
  - `notificationclick`: cierra el aviso, enfoca una ventana abierta o abre `data.url`
    (solo si es una ruta relativa que empieza con `/`).
- El SW se registra solo si `'serviceWorker' in navigator` y `isSecureContext` (en el teléfono:
  HTTPS de `serve`; en escritorio: `localhost` es contexto seguro).

### DD-55 — Frontend
- **Local** (escritorio, listener 7777, `GET /api/remote` → `{enabled:true, local:true}`):
  botón "📱 conectar teléfono" → panel con QR (reusa `qrMatrix`/`drawQr` de DD-19), código
  `ABCD-EFGH`, cuenta regresiva, y la lista de dispositivos con "revocar" (confirmación). Si
  `enabled:false` el botón no aparece.
- **Remoto sin cookie** (`{enabled:true, local:false, paired:false}`): pantalla de emparejar. Si
  la URL trae `#pair=<token>`, lo canjea solo y limpia el fragmento con `history.replaceState`;
  si no, campo para el código. Nada del panel se pide hasta estar emparejado.
- **Remoto emparejado:** el panel de siempre + botón "🔔 avisos en este teléfono" que hace
  `Notification.requestPermission()` dentro del gesto, `pushManager.subscribe({userVisibleOnly:true,
  applicationServerKey})` con la clave de `GET /api/push/key`, y `POST /api/push/subscribe`.
  En iOS sin `navigator.standalone`/`display-mode: standalone`: texto "Agregá el panel a la
  pantalla de inicio (Compartir → Agregar a inicio) para recibir avisos — requiere iOS 16.4+".
- Todo texto que venga del server con `textContent`, nunca `innerHTML` (regla del repo).
- El botón existente "activar avisos" (Notification API dentro de la pestaña) sigue igual.
