<div align="center">

# 🛰️ claude-dashboard

**La torre de control de tus sesiones de Claude Code.**

Tenés 15 sesiones abiertas. Una te está preguntando algo hace 20 minutos.
Este panel te dice **cuál**, **qué quiere** y **desde cuándo** — sin que abras una sola terminal.

![Go](https://img.shields.io/badge/Go-solo%20stdlib-00ADD8?logo=go&logoColor=white)
![JS](https://img.shields.io/badge/frontend-vanilla%20JS-F7DF1E?logo=javascript&logoColor=black)
![Deps](https://img.shields.io/badge/dependencias-0-success)
![Local](https://img.shields.io/badge/escucha-127.0.0.1-informational)
![License](https://img.shields.io/badge/licencia-MIT-blue)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/img/readme-oscuro.png">
  <img alt="Panel con la bandeja 'te esperan', la lista de sesiones y el detalle de una sesión" src="docs/img/readme-claro.png" width="900">
</picture>

</div>

---

## El problema

Claude Code te deja correr muchas sesiones en paralelo: interactivas, en background, workers que
levanta otro agente. Lo que no te da es **una vista de pájaro**. Y la sesión que más importa — la que
está **trabada esperando tu permiso** — es justo la que no hace ruido.

`claude-dashboard` es un daemon chiquito que mira tus sesiones cada 3 segundos y te muestra, en el
navegador, **primero lo que te necesita**.

## Qué vas a ver

**📥 Bandeja "te esperan".** Arriba de todo, las sesiones que están paradas por vos, con el motivo
en una línea: la pregunta que te hizo (`¿Creo el tag v2.4.0?` *+2 preguntas*), el comando que pide
permiso (`Bash: git push origin main`) o lo último que dijo.

**🧭 Todas las sesiones, ordenadas por urgencia.** Esperándote → trabajando → idle → terminadas. Cada
fila trae chips con lo que importa:

| Chip | Qué te dice |
|---|---|
| ⚠ `contexto 850k` | la ventana se está llenando: se viene una compactación |
| `2 subagentes` | cuántos subagentes están corriendo ahora mismo |
| `PR #87` | el último PR que abrió, con link |
| `+412 −96` | líneas agregadas y borradas en la sesión |
| `turno de 42 min` | lleva mucho en el mismo turno: ¿build largo o algo trabado? |
| `acceptEdits` · `auto` | el modo de permisos, si no es el default |
| `6 errores en 10 min` | ráfaga de errores de herramientas: algo se está rompiendo en loop |

**🔍 Detalle de cada sesión.** Último prompt, línea de tiempo de los últimos 20 eventos (prompt →
herramienta → respuesta), el resumen *"mientras no estabas"* que deja Claude cuando volvés, la rama
de git, el avance del plan (`PLAN-42 · 14/28 done · 4 blocked`) y los links a PRs y artifacts.

**📈 Historial de 30 días.** Tokens por día, por modelo y por proyecto. Sin dólares: solo uso.

**🔔 Avisos y favicon con contador.** El título de la pestaña dice `(5) esperándote` y el favicon
muestra el número; activá las notificaciones del navegador y te avisa cuando una sesión pasa a
esperarte.

**📱 Seguí en el teléfono.** Si la sesión tiene Remote Control, el panel dibuja un QR (localmente,
nada sale de tu máquina) para abrirla en el celular.

<div align="center">
<img alt="El panel en un teléfono" src="docs/img/readme-telefono.png" width="260">
</div>

**⌨️ Todo con teclado.** `j`/`k` para moverte, `Enter` para abrir, `1` `2` `3` para filtrar por
estado, `/` para buscar (también por `PR #N` o por repo), `Esc` para volver.

## Instalación

Necesitás **Linux o WSL con systemd `--user`**, **Go** (la versión de `go.mod`) para compilar y
**[Claude Code](https://claude.com/claude-code)** con `claude` en el `PATH`.

```bash
git clone https://github.com/snaven10/claude-dashboard.git
cd claude-dashboard
make install
```

Listo: abrí **<http://localhost:7777>**. El servicio arranca solo con tu sesión de usuario.

```bash
systemctl --user status claude-dashboard   # ¿está vivo?
make uninstall                             # lo saca todo y deja la máquina como estaba
```

¿Querés ver la interfaz sin instalar nada? Serví `web/` con cualquier servidor estático y abrí
`index.html?fixture`: carga datos de ejemplo.

## Ajustes

Todos son flags del binario (editá `ExecStart` en `deploy/claude-dashboard.service` si querés
cambiarlos de forma permanente):

| Flag | Default | Para qué |
|---|---|---|
| `-addr` | `127.0.0.1:7777` | dónde escucha (solo loopback, salvo `-allow-remote`) |
| `-interval` | `3s` | cada cuánto mira las sesiones |
| `-stale` | `10m` | una sesión trabajando sin actividad por más de esto se marca |
| `-context-warn` | `800000` | tokens de contexto a partir de los cuales avisa |
| `-rate-output` / `-rate-calls` | `20000` / `25` | umbrales de "ritmo inusual" en 5 minutos |
| `-errors-warn` | `5` | errores de herramientas en 10 minutos para marcar la sesión |
| `-history-interval` | `60s` | cada cuánto recalcula el historial (`0` lo apaga) |
| `-allow-remote` | `false` | acepta un `-addr` que no sea loopback (expone tus prompts: no lo uses) |
| `-public-url` | vacío | origen `https://` del panel en tu tailnet; **activa el modo remoto** ([guía](#en-el-teléfono-tailscale--pwa)) |
| `-remote-addr` | `127.0.0.1:7778` | listener loopback al que apunta `tailscale serve` (solo con `-public-url`; siempre loopback) |
| `-state-dir` | vacío (`~/.config/claude-dashboard`) | dónde guarda dispositivos, suscripciones y claves del modo remoto |
| `-push-content` | `title` | qué lleva el aviso al teléfono: `full`, `title` o `none` |
| `-push-cooldown` | `2m` | no repite el aviso de una misma sesión antes de este tiempo (`0` lo apaga) |

Para pasarle flags al servicio **sin tocar la unidad versionada**:

```bash
systemctl --user edit claude-dashboard
```

y en el override dejá un `ExecStart` vacío seguido del tuyo (el primero limpia el de la unidad):

```ini
[Service]
ExecStart=
ExecStart=%h/.local/bin/claude-dashboard -public-url https://mi-pc.tailnet-ejemplo.ts.net
```

Después `systemctl --user restart claude-dashboard`.

## En el teléfono (Tailscale + PWA)

Por defecto el panel **no** sale de tu máquina. Con `-public-url` abre un segundo listener, también
en loopback, y `tailscale serve` lo publica **solo dentro de tu tailnet privada**, con HTTPS. El
teléfono lo instala como PWA y recibe avisos push cuando una sesión te espera.

1. **Tailscale en la PC y en el teléfono**, con la misma cuenta. En la consola de administración
   activá **MagicDNS** y **HTTPS Certificates**.
2. *(Opcional)* Permití `serve` sin `sudo`: `sudo tailscale set --operator=$USER`.
3. Publicá el listener remoto y anotá la URL que imprime:

   ```bash
   tailscale serve --bg --https=443 http://127.0.0.1:7778
   # → https://mi-pc.tailnet-ejemplo.ts.net
   ```

   ⚠️ **Nunca uses `tailscale funnel`**: eso lo publica en Internet. Acá solo sirve `serve`.
4. Arrancá el panel con esa URL: `-public-url https://mi-pc.tailnet-ejemplo.ts.net` (ver
   [Ajustes](#ajustes)). Sin ese flag todo sigue exactamente como antes.
5. En el panel de la PC (`http://localhost:7777`) tocá **📱 conectar teléfono**: aparece un QR y un
   código `ABCD-EFGH` que vencen a los 5 minutos y sirven una sola vez. Escaneá el QR con el
   teléfono, **agregá la página a la pantalla de inicio**, abrí la PWA (en iOS, si pide un código,
   tecleá el de la PC) y tocá **🔔 avisos**.
6. **Revocar**: en la PC, la lista de dispositivos tiene **revocar** (corta la sesión de ese
   teléfono y borra sus avisos). Para apagar todo: `tailscale serve reset` y quitá `-public-url`.

### Qué se expone y qué no

- El daemon **nunca** escucha fuera de loopback; lo único que entra es lo que `tailscale serve`
  le reenvía desde tu tailnet.
- Cada teléfono se empareja una vez (cookie de 90 días). Sin cookie, el listener remoto no entrega
  ni sesiones ni eventos ni historial.
- Quien no está en tu tailnet no llega ni a la pantalla de emparejar.
- Sigue siendo **solo lectura**: desde el teléfono no se ejecuta nada sobre tus sesiones.
- El estado nuevo (claves, dispositivos, suscripciones; permisos `0700`/`0600`) vive en
  `~/.config/claude-dashboard/`, nunca en `~/.claude/`.

### Contenido del aviso (`-push-content`)

| Modo | Título | Cuerpo |
|---|---|---|
| `title` (default) | título de la sesión + "— esperándote" | el proyecto |
| `full` | igual | `proyecto: ` + la pregunta o lo que espera |
| `none` | "claude-dashboard" | "Una sesión te espera" |

Si en un mismo ciclo te esperan más de 3 sesiones, sale **un** aviso "N sesiones te esperan".

### WSL2

- **Recomendado:** instalá Tailscale **dentro** de la distro WSL. `tailscale serve` y el panel
  comparten red y `127.0.0.1:7778` funciona directo.
- Si Tailscale corre en **Windows**: el reenvío de localhost de WSL (modo NAT, activo por defecto)
  suele dejar que Windows alcance `127.0.0.1:7778` de WSL, y en modo `mirrored` la loopback es
  compartida. No corras Tailscale en los dos lados a la vez.
- ⚠️ **WSL apaga la VM cuando no hay terminales abiertas**: sin VM no hay panel ni avisos. Dos
  salidas: dejar una terminal abierta, o en `%UserProfile%\.wslconfig` poner
  `[wsl2]` + `vmIdleTimeout=-1` (o una tarea programada de Windows que arranque la distro al
  iniciar sesión).
- Para que el servicio de usuario arranque sin sesión interactiva: `loginctl enable-linger $USER`.

### Limitaciones del teléfono

- **iOS:** los avisos push solo funcionan con la PWA **instalada** en la pantalla de inicio y
  **iOS ≥ 16.4**. La PWA instalada puede tener sus cookies separadas de Safari: por eso el
  emparejamiento se puede pedir de nuevo dentro de la PWA, con el código.
- **Android (Chrome):** funciona directo, instalada o no.
- Los push salen de tu PC hacia los servidores de push de Apple/Google/Mozilla, **cifrados de punta
  a punta** (RFC 8291): ellos ven destino, tamaño y hora, no el texto. Eso **no** les da acceso al
  panel.
- Con la VPN de Tailscale apagada en el teléfono el aviso llega igual, pero el panel no carga.

## Cómo funciona

```
claude agents --json --all  ─┐
~/.claude/sessions/*.json   ─┼─► un solo poller (3 s) ─► snapshot ─► ¿cambió? ─► SSE ─► navegador
~/.claude/projects/**.jsonl ─┘        (lee solo la cola de cada transcript, incremental)
```

- **El estado** sale de `claude agents --json --all`, la interfaz pública de Claude Code.
- **El detalle** (pregunta pendiente, tokens, errores, PRs, subagentes…) sale de los transcripts
  locales, leyendo de forma incremental: un transcript de 50 MB no se relee en cada ciclo.
- **Un solo poller** para todos los navegadores abiertos, y solo emite si algo cambió.
- **Go stdlib + JS vanilla**, embebidos en un único binario. Cero dependencias, cero build de frontend.

La referencia completa de campos y endpoints está en **[docs/API.md](docs/API.md)**; el contrato
del modo remoto, en [Modo remoto](docs/API.md#modo-remoto-plan-011).

## Promesas

- 🔒 **Solo lectura.** No toca tus sesiones, no escribe en `~/.claude/`, no tiene botones que
  ejecuten nada. A lo sumo copia un comando al portapapeles.
- 🏠 **Solo local, salvo que actives el modo remoto.** Los dos listeners siguen en `127.0.0.1`
  porque el panel muestra tus prompts. Un `-addr` no loopback se rechaza salvo que pases
  `-allow-remote` a propósito. Con `-public-url` el panel se ve **por tu tailnet privada** vía
  `tailscale serve` — nunca público — y cada teléfono se empareja por separado. El estado nuevo va
  a `~/.config/claude-dashboard/`, no a `~/.claude/`.
- 💸 **Sin costos en dólares.** Muestra tokens, no plata.
- 🧯 **Degrada, no se cae.** Los archivos de `~/.claude/` son formato interno de Claude Code: si
  cambian, el panel pierde detalle pero sigue mostrando el estado.

## Limitaciones honestas

- Latencia de hasta ~3 s: es polling, no hooks.
- Un permiso pendiente **no queda escrito** en el transcript: en una sesión esperándote se ve lo
  último que dijo Claude, no el diálogo de permiso exacto.
- Solo sesiones de tu máquina.
- El modo remoto necesita la PC encendida (en WSL, con la VM viva) y Tailscale; los avisos en iOS
  exigen la PWA instalada y iOS ≥ 16.4. Detalle en [En el teléfono](#en-el-teléfono-tailscale--pwa).

## Cómo se construyó

Este proyecto se hizo **con** Claude Code, plan por plan: cada funcionalidad tiene su diseño, sus
tareas y su verificación en [`plans/`](plans/). Si querés ver cómo se decidió algo — por qué no hay
hooks, por qué el contexto es absoluto y no un porcentaje — está ahí.

> Los SHAs citados en `plans/` son de antes de publicar el repo (el historial se reescribió para
> quitar datos privados) y ya no existen acá.

## Contribuir

Issues y PRs bienvenidos. Antes de mandar un cambio:

```bash
make test                     # go test -race ./...
node --test web/app.test.js   # tests del frontend
```

Una regla dura: **los fixtures de test son sintéticos**. Nunca pegues un transcript real de
`~/.claude/`: tiene tus prompts y los de tu trabajo.

## Licencia

[MIT](LICENSE) © 2026 snaven10
