# TASK-022 — Fixture de lista (43 sesiones sintéticas) y medición de partida

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-022` desde `main`
- **Depende de:** — (primera del plan; en paralelo con TASK-023)
- **Estado:** `pending`

---

## Objetivo
Existe `index.html?fixture=lista`, que reproduce con datos sintéticos el escenario real que motivó
el plan (18 vivas + 21 terminadas + 4 abandonadas, repetidos, una espera sin `ask`), y queda
anotado cuántas filas entran hoy, para comparar después.

## Contexto verificado
- `web/app.js:469`: `var FIXTURE = new URLSearchParams(location.search).has('fixture');` y en
  `init()` (`app.js:1481-1486`) se hace `fetch('fixture.json')`. `?fixture` sin valor debe seguir
  cargando `fixture.json` (lo usan el README y las tasks anteriores).
- `web/embed.go`: `//go:embed index.html app.js app.css fixture.json fixture-timeline.json
  fixture-history.json vendor/qrcode.js`. Un fixture nuevo **tiene** que agregarse ahí o el binario
  no lo sirve.
- `web/fixture.json`: forma del snapshot `{ generatedAt, sessions: [...] }`; con `?fixture` el
  "ahora" es `generatedAt` (`app.js:803`), así los cortes de 24 h no caducan. Usá el mismo
  `generatedAt` como referencia para todas las horas del fixture nuevo.
- `web/fixture-timeline.json` tiene 5 eventos (`events[]` con `at`, `kind` = `prompt`|`tool`|`text`,
  `text`); es el mismo para cualquier sesión seleccionada.
- Campos que lee el frontend: `id`, `sessionId`, `pid`, `kind`, `name`, `cwd`, `project`, `status`,
  `waitingFor`, `startedAt`, `lastActivityAt`, `remote`, `remoteUrl`, `title`, `lastPrompt`,
  `lastAction`, `stale`, `orq`, `tokens`, `context`, `rate`, `ask`, `errors`, `branch`, `plan`,
  `away`, `prs`, `artifacts`, `lines`, `permissionMode`, `subagents`, `turnStartedAt`
  (ver `internal/agents/agents.go`, struct `Session`).
- Valores reales de `waitingFor` en F1/F2: `"input needed"`, `"permission prompt"`, `"dialog open"`
  (master §2.1).

## Archivos
- **Crear:** `web/fixture-lista.json`
- **Modificar:** `web/fixture-timeline.json`, `web/embed.go`, `web/app.js` (solo el cargador de fixture)

## Pasos
- [ ] **Paso 1 — Cargador.** `?fixture=lista` → `fetch('fixture-lista.json')`; `?fixture` (vacío u
  otro valor) → `fixture.json` como hoy. Cambio mínimo en `FIXTURE`/`init()`; el resto del código
  sigue tratando `FIXTURE` como booleano.
- [ ] **Paso 2 — `web/fixture-lista.json`** con 43 sesiones, todas sintéticas (`/home/user/...`,
  proyectos `api-backend`, `web-app`, `infra`, `docs`, `mobile-app`, ramas tipo `feature/plan-42-x`):
  - **3 esperando, no abandonadas:** (a) `waiting` con `ask` `question` y `more: 1`;
    (b) `waiting` **sin `ask`**, `waitingFor: "input needed"`, con `lastAction` tool (`Bash: npm test`);
    (c) `waiting` **sin `ask`**, `waitingFor: "permission prompt"`, sin `lastAction` pero con `lastPrompt`.
    Una de ellas con `remoteUrl` válido (`https://claude.ai/code/session_` + 24 alfanuméricos).
  - **7 `busy`:** incluye un **par con mismo `cwd` + mismo título y `sessionId` distintos** (no se
    debe colapsar), una con `errors.high`, una con `context.warn`, una `stale`, una con `turnStartedAt`
    de hace 20 min, una con `subagents.active`.
  - **8 `idle`:** incluye (1) un **par con el mismo `sessionId` y PIDs distintos** (gemelos);
    (2) un **par mismo `cwd` + mismo título, `sessionId` distintos**; (3) un **par mismo título en
    `cwd` distintos** (no se colapsa); (4) una **sin título** (`title: ""`, con `name`).
  - **21 `done`:** la mayoría con `lastActivityAt` de más de 24 h; un grupo de **3 con mismo
    `cwd` + título**; 2 sin título en el mismo `cwd` (no se colapsan); 2 con `kind: "background"`.
  - **4 abandonadas:** `blocked` de fondo sin transcript (sin `title`, sin `lastAction`,
    `startedAt` de hace 90-110 d) y una `waiting` interactiva de hace 3 d.
  - Al menos 6 sesiones con `plan` (`number: 42`, `path`, `total`, `done`, `unmerged`) para probar
    "agrupar por plan", y títulos largos (> 80 caracteres) en 3 filas para probar la elipsis.
  - Una sesión con título y `ask` hostiles (`<img src=x onerror=alert(1)>`) para probar que todo es texto.
- [ ] **Paso 3 — `web/fixture-timeline.json`** a 20 eventos mezclando `prompt`/`tool`/`text`
  (comandos de varias líneas incluidos).
- [ ] **Paso 4 — `web/embed.go`**: agregar `fixture-lista.json` al `//go:embed` y al comentario.
- [ ] **Paso 5 — Medición de partida** (master §3 "Medición estándar") con el layout actual, sin
  cambiar CSS: filas visibles a 1440×900 y a 390×844. Anotarlo en `## Resultado` (es la línea base).

## Criterios de aceptación
- [ ] `node -e 'const d=require("./web/fixture-lista.json"); console.log(d.sessions.length)'` → 43;
  los conteos por estado coinciden con el Paso 2 (pegar el script de conteo en `Verificado por`).
- [ ] `rg -n "innerHTML" web/app.js` vacío; `node --check web/app.js`; `node --test web/` en verde
  (los tests existentes no cambian).
- [ ] `gofmt -l .` vacío, `go vet ./...` y `go test -race ./...` en verde (embed nuevo compila).
- [ ] `index.html?fixture` sigue mostrando el fixture original; `index.html?fixture=lista` muestra 43.
- [ ] Revisión de privacidad: `rg -n -i "home/[a-z]+/" web/fixture-lista.json` solo devuelve
  `/home/user/`; ningún nombre propio, repo de trabajo, URL interna ni IP.
- [ ] Línea base de filas visibles anotada (o "no pude abrir navegador: lo mide el orquestador").

## Riesgos
- Si el fixture no trae `generatedAt`, los cortes de 24 h se calculan contra `Date.now()` y las
  "abandonadas"/"terminadas viejas" cambian con el tiempo. Revisarlo.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
