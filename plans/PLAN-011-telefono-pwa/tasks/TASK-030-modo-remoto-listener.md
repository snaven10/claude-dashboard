# TASK-030 — Flags del modo remoto, segundo listener loopback y store de estado

- **Plan:** PLAN-011 — El panel en el teléfono
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (`~/personal/claude-dashboard`), rama de la task desde `main`
- **Depende de:** — (primera del plan; paralela con TASK-032)
- **Estado:** `pending`

---

## Objetivo
Con `-public-url` el daemon abre un segundo listener loopback (`-remote-addr`) que comparte poller
y rutas con el local, y cada handler sabe por cuál entró la petición. Existe un store de estado en
`~/.config/claude-dashboard/` con permisos 0700/0600 y escritura atómica. Sin el flag, nada cambia.

## Contexto verificado
- Leé el master y `PLAN-011-design.md`: DD-43, DD-44, DD-45, DD-54 (solo la parte de servir estáticos).
- `cmd/claude-dashboard/main.go`: `run()` parsea flags, llama `server.ValidateAddr(*addr, *allowRemote)`
  y arma `server.Server{...}`; termina en `s.ListenAndServe(ctx, *addr)`.
- `internal/server/server.go`: `ValidateAddr` (DD-7), `Handler()` arma el `ServeMux` con
  `GET /api/sessions`, `/api/events`, `/api/sessions/{sessionId}/timeline`, `/api/history` y
  `http.FileServer(http.FS(s.Static))` en `/`. `ListenAndServe` abre UN `net.Listen`, arranca
  `Poll` y hace shutdown cerrando suscriptores SSE.
- `web/embed.go` lista los archivos a mano en `//go:embed` (no se toca acá: lo hace TASK-035).

## Archivos
- **Crear:** `internal/remote/store.go`, `internal/remote/store_test.go`, `internal/remote/config.go`, `internal/remote/config_test.go`
- **Modificar:** `cmd/claude-dashboard/main.go`, `internal/server/server.go`, `internal/server/server_test.go`

## Pasos
- [ ] **Paso 1 — `internal/remote/config.go`.** `ParsePublicURL(s string) (string, error)`: exige
  `https`, host no vacío, sin path (salvo `/`), sin query ni fragmento; devuelve el origen normalizado
  sin `/` final. `ValidateRemoteAddr(addr)`: loopback obligatorio (sin escape).
- [ ] **Paso 2 — `internal/remote/store.go`.** `Dir(stateDir string) (string, error)` (default
  `os.UserConfigDir()/claude-dashboard`), crea con `0700`. Helpers genéricos
  `readJSON(path, v)` / `writeJSON(path, v)`: temp en el mismo dir → `Write` → `Sync` → `Chmod 0600`
  → `Rename`. Al leer, si el modo es más abierto que `0600`, `Chmod 0600` y log. Archivo
  inexistente = valor cero sin error. Mutex por store.
- [ ] **Paso 3 — flags en `main.go`.** `-public-url` (vacío = apagado), `-remote-addr`
  (`127.0.0.1:7778`), `-state-dir` (vacío = default). Si `-public-url` está vacío, los otros se
  ignoran y NO se crea el directorio. Errores de validación cortan el arranque con mensaje claro.
- [ ] **Paso 4 — dos listeners en `server`.** `Server` gana `RemoteAddr string` y `PublicURL string`.
  `ListenAndServe` abre el segundo listener solo si `RemoteAddr != ""`; un solo poller. Marcá el
  origen con un tipo propio en el `context` (p. ej. `http.Server.BaseContext` por listener) y un
  helper `isRemote(r *http.Request) bool`. Shutdown limpio de los dos.
- [ ] **Paso 5 — estáticos de la PWA.** En el handler estático: `*.webmanifest` →
  `Content-Type: application/manifest+json`; `/sw.js` → `Cache-Control: no-cache`. Probá con
  `fstest.MapFS` (los archivos reales los crea TASK-035).
- [ ] **Paso 6 — tests.**

## Criterios de aceptación
- [ ] Test: `ParsePublicURL` acepta `https://mi-pc.tailnet-ejemplo.ts.net` y `…/`; rechaza `http://`,
  con path, con query, vacío de host.
- [ ] Test: `ValidateRemoteAddr` rechaza `0.0.0.0:7778`, `192.0.2.1:7778` (TEST-NET) y acepta `127.0.0.1:7778`, `[::1]:7778`.
- [ ] Test: `writeJSON` deja el archivo en `0600` y el dir en `0700` (en `t.TempDir()`); un archivo
  preexistente en `0644` queda en `0600` tras leerlo; escritura concurrente con `-race` sin corrupción.
- [ ] Test con `httptest`/listeners reales en puertos efímeros: una petición por el listener remoto
  ve `isRemote == true`, por el local `false`; los dos sirven el mismo snapshot.
- [ ] Test: sin `RemoteAddr`, `ListenAndServe` abre un solo listener (comportamiento de hoy) y
  ningún test previo de `server_test.go` cambia.
- [ ] Test: `manifest.webmanifest` sale con `application/manifest+json`; `sw.js` con `no-cache`.
- [ ] `go.mod` sigue sin `require`. `gofmt -l .` vacío, `go vet ./...`, `go test -race ./...` en verde y pegados.

## Riesgos
- `BaseContext` es por `http.Server`: con dos listeners hacen falta dos `http.Server` (mismo
  handler). Revisá que el shutdown espere a los dos.

## Resultado
<!-- SE LLENA AL CERRAR -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
