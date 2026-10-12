# TASK-034 — Notificador en el poller y endpoints `/api/push/*`

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** TASK-031 (gate/cookie), TASK-033 (sender + suscripciones)
- **Estado:** `done`

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

> **Nota de la revisión de TASK-033 (2026-10-11):** el `webpush.Sender` de producción tiene que usar su
> cliente por defecto (guard SSRF: sin redirecciones, solo IPs públicas al conectar, sin proxy de entorno).
> **No inyectes un `http.Client` propio** en el cableado del daemon: uno inyectado se saltea el guard de IP.

## Pasos
- [x] **Paso 1 — `notify.go`.** `Notifier{Now, Cooldown, Content, Enqueue func(payload []byte, topic string)}`
  con `Observe(sessions []agents.Session)`: estado por `sessionId` (último status, ciclos seguidos
  en waiting, último aviso). Reglas de DD-52 en este orden: primer ciclo → solo registra;
  abandonada → ignora; 2 ciclos seguidos waiting tras no-waiting → candidata; cooldown; > 3
  candidatas → un aviso agrupado. Olvida sesiones ausentes.
- [x] **Paso 2 — payload.** Builder de DD-53 por modo (`full`|`title`|`none`), truncado a 180 runas
  con "…", JSON `{"v":1,...}` ≤ 3000 bytes (si se pasa, recortar `body`). `url` = `/?s=<sessionId>`
  solo si el id cumple `^[A-Za-z0-9_-]{1,128}$`; si no, `/`.
- [x] **Paso 3 — enganche.** `Refresh` llama `s.Notifier.Observe(sessions)` si `Notifier != nil`,
  fuera de `s.mu`. El envío es asíncrono (cola de TASK-033): `Observe` nunca bloquea el poller.
- [x] **Paso 4 — endpoints.** `GET /api/push/key` → `{key}`; `POST /api/push/subscribe` con el JSON
  de `PushSubscription.toJSON()` (`endpoint`, `keys.p256dh`, `keys.auth`) → valida (DD-51) y guarda
  con el `deviceId` de la cookie (o `"local"` en el listener local); `POST /api/push/unsubscribe`
  `{endpoint}`. Anti-CSRF de DD-48. Revocar un dispositivo borra sus suscripciones.
- [x] **Paso 5 — flags.** `-push-content` (`full|title|none`, default `title`, decisión del usuario) y `-push-cooldown` (default `2m`, ≥ 0). Valores inválidos cortan el arranque.

## Criterios de aceptación
- [x] Test: primer `Observe` con sesiones ya en waiting → 0 avisos.
- [x] Test: busy → waiting (1 ciclo) → 0 avisos; segundo ciclo waiting → 1 aviso; tercer ciclo → 0.
- [x] Test: waiting un solo ciclo y vuelve a busy → 0 avisos (antirrebote).
- [x] Test: avisa, sale y vuelve a waiting antes del cooldown → 0; después del cooldown → 1 (reloj falso).
- [x] Test: `blocked` cuenta como waiting; waiting con `lastActivityAt` de hace 25 h → 0 avisos.
- [x] Test: 5 sesiones pasan a la vez → 1 aviso "5 sesiones te esperan".
- [x] Test por modo: `full` incluye `waitingFor` truncado; `title` NO incluye `waitingFor` ni `ask.text`
  (grep del payload); `none` no incluye título, proyecto ni id.
- [x] Test: una sesión que desaparece se borra del estado interno (el mapa no crece); si reaparece
  en waiting, avisa recién al cumplir el antirrebote (2 ciclos), como cualquier transición nueva.
- [x] Test HTTP: `subscribe` por el remoto sin cookie → 401; con cookie → 200 y queda con su
  `deviceId`; endpoint `http://` o IP literal → 400; revocar el dispositivo borra su suscripción.
- [x] Test integración con `-race`: `Refresh` con un `List` falso que cambia de estado + `Enqueue`
  falso → el aviso se encola una sola vez y el poller no se bloquea si `Enqueue` tarda.
- [x] Sin `PublicURL`: `Notifier == nil`, `/api/push/*` → 404, tests previos intactos.
- [x] `gofmt`, `go vet`, `go test -race ./...` en verde y pegados.

## Resultado
- **Estado final:** `done`
- **Resumen:** `Notifier` en `internal/server` con las reglas de DD-52 (primer ciclo solo registra, antirrebote de 2 ciclos con transición previa, cooldown por sesión, abandonadas > 24 h ignoradas, > 3 candidatas → un aviso agrupado, olvida ausentes) y payload de DD-53 por modo (`full|title|none`, 180 runas, JSON ≤ 3000 B, `url` solo con id válido). `Refresh` llama `Observe` en cada ciclo antes del corte por hash y fuera de `s.mu`; la entrega a `Enqueue` va por cola propia con worker (el poller no se bloquea aunque `Enqueue` tarde). Endpoints `GET /api/push/key`, `POST /api/push/subscribe` y `/unsubscribe` (solo con `-public-url`; sin él 404). Revocar un dispositivo borra sus suscripciones. Flags `-push-content` (default `title`) y `-push-cooldown` (default `2m`) con validación al arrancar. El `webpush.Sender` usa su cliente por defecto (guard SSRF), sin `http.Client` inyectado.
- **Archivos tocados:** `internal/server/notify.go` (nuevo), `internal/server/notify_test.go` (nuevo), `internal/server/poller.go`, `internal/server/server.go`, `internal/server/remote.go`, `cmd/claude-dashboard/main.go`. Commit de código: `3224445`.
- **Verificado por:** `gofmt -l .` → vacío; `go vet ./...` → sin salida; `go test -race ./...` → todos los paquetes `ok` (server también con `-count=3`). Cubiertos por test: los 11 criterios de aceptación (primer ciclo, debounce 1→aviso→0, rebote, cooldown con reloj falso, blocked/abandonada, agrupado de 5, modos con grep de fugas, olvido de ausentes, HTTP 401/200/400/revocar, integración `Refresh` con `Enqueue` bloqueado, sin `PublicURL` → 404). NO verificado: envío real contra un servicio de push ni el arranque del daemon con `-public-url` (no se levantó el server; `main.go` no tiene tests).
- **Desviaciones:** (1) `main.go` de TASK-031 declaraba `pairing`/`devices` pero nunca los construía, así que el modo remoto no se activaba en producción; los cableé acá junto con `Subscriptions`, claves VAPID (`vapid.json`, `Subject = -public-url`) y `Sender`. (2) Validación de endpoint/claves duplicada en el handler (usa `remote.ValidateEndpoint` + largo de claves) para distinguir 400 de 500, porque `Subscriptions.Add` no tipa sus errores. (3) `unsubscribe` desde un dispositivo remoto solo borra suscripciones propias (idempotente, 204). (4) `Notifier` expone `Flush()` y `Len()` para tests; un `Content` desconocido se trata como `none`.
- **Riesgos abiertos / siguiente:** el commit pidió atribución de AI por el harness y el hook del usuario la bloquea: el commit va sin ella, como manda la regla del usuario. El aviso agrupado en `full`/`title` lista solo proyectos. Un aviso de sesión con cooldown vencido pero racha ya > 2 no dispara hasta una nueva transición (consistente con DD-52). Falta el gate real en teléfono (TASK-038) y que TASK-036 consuma `/api/push/*`.
