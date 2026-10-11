# PLAN-011 — El panel en el teléfono: Tailscale + PWA + avisos push

**Fecha:** 2026-10-11
**Fase:** 2 (Ejecución) — aprobado por el usuario 2026-10-11
**Diseño:** [`PLAN-011-design.md`](./PLAN-011-design.md)
**Fase anterior:** [PLAN-004](../PLAN-004-abrir-en-telefono/PLAN-004-abrir-en-telefono.md) (QR de Remote Control; dejó "abrir el PANEL desde el teléfono" fuera de alcance)
**Proyectos:** claude-dashboard (`~/personal/claude-dashboard`)
**Origen:** pedido del usuario — ver el panel desde el teléfono y recibir un push cuando una sesión pasa a "esperándote", aunque la app esté cerrada

## 1. Qué resuelve
Hoy el panel solo se ve en la PC (`127.0.0.1:7777`) y los avisos solo llegan si la pestaña está
abierta. El usuario quiere: el panel muestra un QR → el teléfono lo escanea → abre el panel →
"agregar a pantalla de inicio" (PWA) → activa avisos → le llega un push cuando una sesión lo
espera, con la app cerrada.

**Nada queda público en internet.** El panel muestra prompts con datos de trabajo: el acceso es
solo por la tailnet privada del usuario (`tailscale serve`, nunca `funnel`) y, encima, cada
teléfono tiene que estar emparejado con un token de un solo uso. Los push salen de la PC hacia
los servidores de push de Apple/Google, cifrados de punta a punta: eso es tráfico de salida y no
expone el panel.

## 2. Hallazgos que reorientan el plan (verificados 2026-10-11)
- **Tailscale corre DENTRO de WSL**, no en Windows: `tailscale` 1.102 en el PATH de la distro,
  `tailscaled` activo por systemd, backend `Running`, MagicDNS con sufijo `ts.net`. No hay
  instalación en `C:\Program Files\Tailscale`. Consecuencia: `tailscale serve` y el daemon
  comparten la red de la VM de WSL → `serve` alcanza `127.0.0.1:<puerto>` directo. **El gotcha
  de reenvío localhost Windows↔WSL no aplica** a esta máquina (sí aplicaría si Tailscale estuviera
  del lado Windows: ver README de TASK-037).
- **HTTPS de la tailnet NO está habilitado todavía** (la lista de dominios de certificado está
  vacía): el usuario tiene que activar "HTTPS Certificates" en la consola de administración de
  Tailscale antes de `tailscale serve --https`. Paso manual del usuario.
- **No hay teléfono en la tailnet** (los 2 pares son Linux): el usuario instala la app de
  Tailscale en el teléfono con la misma cuenta. Paso manual del usuario.
- **No hay `serve` configurado** y WSL está en modo de red `nat`.
- **`RemoteAddr` no sirve para saber si una petición es remota:** `tailscale serve` es un proxy
  local, todo llega desde 127.0.0.1. Se resuelve con un **segundo listener loopback** solo para
  `serve` (DD-43). El daemon sigue escuchando únicamente en loopback: la promesa se mantiene.
- **Go 1.27** en `go.mod` (toolchain local 1.27.1): `crypto/ecdh`, `crypto/hkdf`,
  `ecdsa.PublicKey.Bytes` están en la stdlib → Web Push sin dependencias (DD-50, DD-51).
- La detección de "pasó a esperándote" existe **solo en el front** (`notifyTargets` + filtro de
  abandonadas DD-30 en `web/app.js`). El push necesita su gemelo en Go en el poller (DD-52);
  `Refresh` corta por hash, así que el notificador se engancha antes del corte.
- `web/vendor/qrcode.js` + `qrMatrix`/`drawQr` (PLAN-004, DD-19) ya pintan QR en canvas sin red:
  el QR de emparejamiento los reusa.
- WSL apaga la VM cuando no queda ninguna terminal abierta (aunque haya servicios systemd): con la
  VM apagada no hay panel ni push. Limitación a documentar con sus salidas (TASK-037).

## 3. Tasks y orden
| Task | Qué | Especialista | Modelo | Depende de | Estado |
|------|-----|--------------|--------|------------|--------|
| **TASK-030** | Flags del modo remoto, segundo listener loopback, store de estado 0600, rutas estáticas de la PWA | claude-dashboard-worker | sonnet | — | `done` |
| **TASK-031** | Emparejamiento (token + código), cookie de dispositivo, gate del listener remoto, dispositivos y revocación | claude-dashboard-worker | sonnet | TASK-030 | `pending` |
| **TASK-032** | Cifrado aes128gcm de Web Push (RFC 8291/8188) con el vector del RFC | claude-dashboard-worker | **opus** | — | `pending` |
| **TASK-033** | Claves VAPID + JWT ES256 + envío HTTP al servicio de push + store de suscripciones | claude-dashboard-worker | sonnet | TASK-030, TASK-032 | `pending` |
| **TASK-034** | Notificador en el poller (transición, antirrebote, cooldown, agrupado, `-push-content`) + endpoints `/api/push/*` | claude-dashboard-worker | sonnet | TASK-031, TASK-033 | `pending` |
| **TASK-035** | PWA: manifest, íconos, `sw.js`, tags en `index.html` | claude-dashboard-worker | sonnet | TASK-030 · blanda: PLAN-010 (`index.html`) | `pending` |
| **TASK-036** | Frontend: "conectar teléfono" (QR, código, dispositivos), pantalla de emparejar, botón de avisos push | claude-dashboard-worker | sonnet | TASK-031, TASK-034, TASK-035 · **PLAN-010 cerrado** | `pending` |
| **TASK-037** | README + docs/API.md + guía Tailscale/WSL + limitaciones iOS/Android | claude-dashboard-worker | sonnet | TASK-034, TASK-036 | `pending` |
| **TASK-038** | Smoke end-to-end + **gate del usuario con su teléfono real** | orquestador + usuario | — | TASK-037 | `pending` |

**Modelo de TASK-032 = opus.** Es poco código, pero el vector del RFC solo prueba la derivación
de claves con entradas fijas: un error que lo pasa igual — reusar `salt` o la clave efímera entre
mensajes, no validar la clave pública del navegador, delimitador de padding equivocado en el
camino real — rompe la confidencialidad sin que ningún test falle. Ese razonamiento adversarial
es donde opus rinde; con pocas líneas el costo extra es chico. TASK-033 (JWT) tiene un oráculo
fuerte (`ecdsa.Verify` con la clave pública y el formato r||s) y queda en sonnet.

### Paralelismo

| Lote | Tasks | Condición |
|------|-------|-----------|
| 1 | **TASK-030**, **TASK-032** | Sin dependencias entre sí: 030 toca `cmd/`, `internal/server/`, `internal/remote/`; 032 solo `internal/webpush/ece*.go` |
| 2 | **TASK-031**, **TASK-033**, **TASK-035** | Tras lote 1. 031 toca `internal/server/` + `internal/remote/`; 033 solo `internal/webpush/` (+ `internal/remote/` en archivo propio `subscriptions.go`); 035 solo `web/` (no `app.js`/`app.css`). 035 espera a PLAN-010 si este toca `index.html` |
| 3 | **TASK-034** | Tras 031 y 033 (toca `internal/server/` y `cmd/`, que 031 también tocó) |
| 4 | **TASK-036** | Tras 031, 034, 035 **y con PLAN-010 mergeado**: pisa `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js` |
| 5 | **TASK-037** | Tras 034 y 036 (documenta el contrato final) |
| 6 | **TASK-038** | Tras todo; requiere los pasos manuales del usuario en Tailscale |

**Dependencia blanda con PLAN-010** (rediseño de la lista, `web/app.js`/`app.css`):
- **TASK-036** pisa `web/app.js`, `web/app.css`, `web/index.html`, `web/app.test.js` → se ejecuta
  DESPUÉS de que PLAN-010 esté mergeado, sobre su versión.
- **TASK-035** toca `web/index.html` (4 líneas en `<head>`) y `web/embed.go` → si PLAN-010 toca
  esos archivos, va después; si no, puede ir en el lote 2.
- TASK-030..034 son solo Go: corren en paralelo con PLAN-010 sin conflicto.

## 4. Restricciones duras
- **Go stdlib + JS vanilla, cero dependencias, cero build de frontend.** `go.mod` sin `require`.
- **El daemon solo escucha en loopback** (los dos listeners). `-remote-addr` no acepta otra cosa ni
  con `-allow-remote`. Nunca `tailscale funnel`.
- **Sin `-public-url`, comportamiento idéntico al actual** (DD-44): mismas rutas de API, ningún
  archivo nuevo en disco, ningún listener extra.
- **Solo lectura:** ningún botón que actúe sobre sesiones, nunca `claude rm`, sin costos en
  dólares. Revocar un dispositivo no toca sesiones.
- **Nada en `~/.claude/`:** el estado nuevo vive en `~/.config/claude-dashboard/` (DD-45).
- **Fixtures y vectores de test SINTÉTICOS.** Repo público: ni el plan ni el código ni los tests
  llevan nombres de clientes, repos de trabajo, URLs internas, IPs, nombres de máquinas, nombres
  de la tailnet real ni rutas del HOME. Ejemplos: `mi-pc.tailnet-ejemplo.ts.net`,
  `https://push.ejemplo.test/…`. Los vectores del RFC 8291 son públicos y se pueden versionar.
- Texto del server en el DOM solo con `textContent`, nunca `innerHTML`.
- Cada task del worker cierra con `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` y,
  si toca `web/`, `node --check web/*.js` y `node --test web/`, pegados en `Verificado por`.
- No `make install`, no levantar el server contra sesiones reales, no tocar la configuración de
  Tailscale de la máquina: los pasos de Tailscale los hace el usuario (TASK-038).

## 5. Fuera de alcance
- Exponer el panel fuera de la tailnet (Funnel, port-forwarding, túneles públicos).
- Acciones desde el teléfono (responder a la sesión, aprobar permisos, attach). Solo lectura.
- Notificar por otros eventos (errores altos, contexto lleno, done): solo "esperándote".
- Varios usuarios / cuentas: un dispositivo emparejado ve todo el panel.
- Chequeo de `Host` contra DNS rebinding en el listener local (riesgo previo a este plan; se
  anota en Riesgos para un plan aparte).
- App nativa, Remote Control de claude.ai (eso ya lo cubre PLAN-004).
- Instalar o configurar Tailscale desde el código o el Makefile.

## 6. Riesgos
- **iOS:** push solo con PWA instalada e iOS ≥ 16.4; almacén de cookies de la PWA posiblemente
  separado del de Safari (por eso el código corto, DD-47). Se valida en el gate.
- **WSL se apaga sin terminales** → sin panel ni push. Mitigación documentada, no resuelta por código.
- **El teléfono fuera de la tailnet** (VPN de Tailscale apagada): el push llega igual, pero tocarlo
  abre una PWA que no carga. Documentado.
- **Proxy SSE de `tailscale serve`:** se asume que no bufferea `text/event-stream`; si lo hiciera,
  el panel remoto se congela. Se comprueba en TASK-038; plan B: el front cae a polling de
  `/api/sessions` cada `interval` si el SSE no entrega en 20 s.
- **DNS rebinding sobre `127.0.0.1:7777`** (ya existía): fuera de alcance, ver §5.

## 7. Decisiones del usuario (2026-10-11)
1. **Contenido por defecto del push:** `title` (título + proyecto, sin la pregunta).
2. **Token / cookie:** token de emparejamiento 5 min; cookie de dispositivo **90 días** (al vencer, re-emparejar con el QR).
3. **WSL apagado:** el README advierte **y** trae la receta para mantener la VM viva. El plan no la ejecuta.

## 8. Cierre
<!-- SE LLENA AL CERRAR EL PLAN -->
- **Cerrado:**
- **Shipeó:**
- **Verificación:**
- **Quedó afuera:**
- **Continúa en:**
