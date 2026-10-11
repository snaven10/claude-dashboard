# TASK-034 — Notificador en el poller y endpoints `/api/push/*`

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-031 (gate/cookie), TASK-033 (sender + suscripciones)
- **Estado:** `pending`

---

## Objetivo
Cuando una sesión pasa a "esperándote", el daemon manda un push cifrado a cada suscripción, con
antirrebote, cooldown, sin abandonadas y agrupando ráfagas. Los dispositivos pueden suscribirse y
desuscribirse. Sin `-public-url` el notificador no existe.

## Contexto verificado
- Design **DD-52** (reglas del disparador), **DD-53** (contenido y `-push-content`), DD-47 (permisos
  de `/api/push/*`), DD-49 (revocar borra suscripciones).
- `internal/server/poller.go`: `Refresh` arma `sessions` con `build`, calcula hash y, si no cambió,
  retorna temprano. El notificador se llama con `sessions` en CADA ciclo, antes del corte por hash.
- Semántica a replicar del front (`web/app.js`): `isWaiting` (`waiting`|`blocked`), `notifyTargets`
  (no avisa en el primer snapshot), `isAbandoned` (> 24 h desde `lastActivityAt` o `startedAt`, DD-30),
  texto del aviso de `fireNotifications` (título + " — esperándote", `proyecto: waitingFor`).
- `agents.Session` trae `Title`, `Name`, `Project`, `Status`, `WaitingFor *string`, `Ask *Ask`,
  `LastActivityAt`, `StartedAt`, `SessionID`.

## Archivos
- **Crear:** `internal/server/notify.go`, `internal/server/notify_test.go`
- **Modificar:** `internal/server/poller.go` (llamada al notificador), `internal/server/server.go`
  (rutas `/api/push/*`, campo `Notifier`), `internal/server/remote.go` (revocar → `RemoveDevice`),
  `cmd/claude-dashboard/main.go` (flags `-push-content`, `-push-cooldown`, cableado)

## Pasos
- [ ] **Paso 1 — `notify.go`.** `Notifier{Now, Cooldown, Content, Enqueue func(payload []byte, topic string)}`
  con `Observe(sessions []agents.Session)`: estado por `sessionId` (último status, ciclos seguidos
  en waiting, último aviso). Reglas de DD-52 en este orden: primer ciclo → solo registra;
  abandonada → ignora; 2 ciclos seguidos waiting tras no-waiting → candidata; cooldown; > 3
  candidatas → un aviso agrupado. Olvida sesiones ausentes.
- [ ] **Paso 2 — payload.** Builder de DD-53 por modo (`full`|`title`|`none`), truncado a 180 runas
  con "…", JSON `{"v":1,...}` ≤ 3000 bytes (si se pasa, recortar `body`). `url` = `/?s=<sessionId>`
  solo si el id cumple `^[A-Za-z0-9_-]{1,128}$`; si no, `/`.
- [ ] **Paso 3 — enganche.** `Refresh` llama `s.Notifier.Observe(sessions)` si `Notifier != nil`,
  fuera de `s.mu`. El envío es asíncrono (cola de TASK-033): `Observe` nunca bloquea el poller.
- [ ] **Paso 4 — endpoints.** `GET /api/push/key` → `{key}`; `POST /api/push/subscribe` con el JSON
  de `PushSubscription.toJSON()` (`endpoint`, `keys.p256dh`, `keys.auth`) → valida (DD-51) y guarda
  con el `deviceId` de la cookie (o `"local"` en el listener local); `POST /api/push/unsubscribe`
  `{endpoint}`. Anti-CSRF de DD-48. Revocar un dispositivo borra sus suscripciones.
- [ ] **Paso 5 — flags.** `-push-content` (`full|title|none`, default `title`, decisión del usuario) y `-push-cooldown` (default `2m`, ≥ 0). Valores inválidos cortan el arranque.

## Criterios de aceptación
- [ ] Test: primer `Observe` con sesiones ya en waiting → 0 avisos.
- [ ] Test: busy → waiting (1 ciclo) → 0 avisos; segundo ciclo waiting → 1 aviso; tercer ciclo → 0.
- [ ] Test: waiting un solo ciclo y vuelve a busy → 0 avisos (antirrebote).
- [ ] Test: avisa, sale y vuelve a waiting antes del cooldown → 0; después del cooldown → 1 (reloj falso).
- [ ] Test: `blocked` cuenta como waiting; waiting con `lastActivityAt` de hace 25 h → 0 avisos.
- [ ] Test: 5 sesiones pasan a la vez → 1 aviso "5 sesiones te esperan".
- [ ] Test por modo: `full` incluye `waitingFor` truncado; `title` NO incluye `waitingFor` ni `ask.text`
  (grep del payload); `none` no incluye título, proyecto ni id.
- [ ] Test: una sesión que desaparece se borra del estado interno (el mapa no crece); si reaparece
  en waiting, avisa recién al cumplir el antirrebote (2 ciclos), como cualquier transición nueva.
- [ ] Test HTTP: `subscribe` por el remoto sin cookie → 401; con cookie → 200 y queda con su
  `deviceId`; endpoint `http://` o IP literal → 400; revocar el dispositivo borra su suscripción.
- [ ] Test integración con `-race`: `Refresh` con un `List` falso que cambia de estado + `Enqueue`
  falso → el aviso se encola una sola vez y el poller no se bloquea si `Enqueue` tarda.
- [ ] Sin `PublicURL`: `Notifier == nil`, `/api/push/*` → 404, tests previos intactos.
- [ ] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
