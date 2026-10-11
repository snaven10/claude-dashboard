# TASK-037 — README, docs/API.md y guía de Tailscale/WSL

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-034, TASK-036 (documenta el contrato y la UI finales)
- **Estado:** `pending`

---

## Objetivo
Un desconocido que clona el repo entiende qué cambia con el modo remoto, cómo activarlo con
Tailscale paso a paso (incluido WSL2), qué se expone y qué no, y las limitaciones de iOS/Android.

## Contexto verificado
- `README.md`: secciones "Ajustes" (tabla de flags con `-addr` y `-allow-remote`), "Promesas"
  ("🏠 Solo local. Escucha en 127.0.0.1…"), "Limitaciones honestas"; badge `escucha-127.0.0.1`.
- `docs/API.md`: "Campos por sesión", "De dónde saca los datos", "Abrir una sesión en el teléfono
  (PLAN-004)" que dice "El panel sigue en 127.0.0.1: se abre la SESIÓN en el teléfono, no el panel".
- `deploy/claude-dashboard.service`: `ExecStart=%h/.local/bin/claude-dashboard` sin flags.
- Master §2 (hallazgos de Tailscale/WSL) y design completo.

## Archivos
- **Modificar:** `README.md`, `docs/API.md`

## Pasos
- [ ] **Paso 1 — Promesas.** "🏠 Solo local, salvo que actives el modo remoto": los dos listeners
  siguen en loopback; con `-public-url` el panel se ve por tu tailnet privada vía `tailscale serve`;
  nunca público; emparejamiento por dispositivo. Mantener "solo lectura" y "sin costos". Aclarar que
  el estado nuevo va a `~/.config/claude-dashboard/` (no a `~/.claude/`).
- [ ] **Paso 2 — Ajustes.** Filas para `-public-url`, `-remote-addr`, `-state-dir`, `-push-content`,
  `-push-cooldown`. Cómo pasarlos al servicio: `systemctl --user edit claude-dashboard` con un
  override de `ExecStart` (sin tocar la unidad versionada).
- [ ] **Paso 3 — sección nueva "En el teléfono (Tailscale + PWA)".** Pasos del usuario:
  1. Tailscale en la PC y en el teléfono, misma cuenta; MagicDNS y **HTTPS Certificates** activados
     en la consola de administración.
  2. Permitir `serve` sin sudo: `sudo tailscale set --operator=$USER` (opcional).
  3. `tailscale serve --bg --https=443 http://127.0.0.1:7778` y anotar la URL `https://<pc>.<tailnet>.ts.net`
     (ejemplo: `https://mi-pc.tailnet-ejemplo.ts.net`). **Nunca `tailscale funnel`** (eso es público).
  4. Arrancar el panel con `-public-url https://mi-pc.tailnet-ejemplo.ts.net`.
  5. En el panel de la PC: "📱 conectar teléfono" → escanear el QR → en el teléfono "agregar a
     pantalla de inicio" → abrir la PWA (en iOS, si pide código, teclear el de la PC) → "🔔 avisos".
  6. Revocar: en la PC, lista de dispositivos → revocar. Para apagar todo: `tailscale serve reset`
     y quitar `-public-url`.
- [ ] **Paso 4 — WSL2.** Recomendado: Tailscale dentro de la distro WSL (así `serve` y el panel
  comparten red y `127.0.0.1` funciona directo). Si Tailscale está en Windows: el reenvío
  localhost de WSL (modo NAT, `localhostForwarding`, default activo) suele permitir que Windows
  alcance `127.0.0.1:7778` de WSL, y en modo `mirrored` la loopback es compartida; no correr
  Tailscale en los dos lados a la vez. WSL apaga la VM sin terminales abiertas → sin panel ni push
  Decisión del usuario: advertencia **más** la receta para mantener la VM viva (`vmIdleTimeout=-1` en `.wslconfig` o tarea programada de Windows).
  `loginctl enable-linger` para el servicio de usuario.
- [ ] **Paso 5 — Limitaciones honestas.** iOS: push solo con la PWA instalada e iOS ≥ 16.4; el
  emparejamiento puede pedirse dentro de la PWA. Android Chrome: directo. Los push salen de la PC
  hacia los servidores de push de Apple/Google/Mozilla, cifrados de punta a punta (RFC 8291):
  ellos ven destino, tamaño y hora, no el texto; eso NO abre el panel. Con la VPN de Tailscale
  apagada en el teléfono el aviso llega igual pero el panel no carga. Qué incluye el aviso según
  `-push-content`.
- [ ] **Paso 6 — docs/API.md.** Sección "Modo remoto (PLAN-011)": tabla de DD-47 (ruta × listener ×
  requisito), contrato JSON de `/api/remote`, `/api/pair/new`, `/api/pair/redeem`, `/api/devices`,
  `DELETE /api/devices/{id}`, `/api/push/key`, `/api/push/subscribe`, `/api/push/unsubscribe`; la
  cookie `cd_device`; archivos de estado y permisos; payload del push por modo. Actualizar la frase
  de PLAN-004 ("el panel sigue en 127.0.0.1…") para remitir al modo remoto.

## Criterios de aceptación
- [ ] Toda flag nueva de `main.go` aparece en la tabla de Ajustes con su default real (comparado contra las llamadas `flag.*` de `main.go`).
- [ ] Toda ruta registrada en `internal/server` aparece en `docs/API.md` (comparar con `grep HandleFunc`).
- [ ] `git grep -nE '([0-9]{1,3}\.){3}[0-9]{1,3}'` en README/docs solo devuelve `127.0.0.1` (y rangos de documentación si se usan); ningún nombre de tailnet, máquina, usuario ni ruta del HOME real.
- [ ] La palabra "funnel" aparece solo en la advertencia de NO usarlo.
- [ ] Links internos de README a docs/API.md resuelven (anclas existentes).

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
