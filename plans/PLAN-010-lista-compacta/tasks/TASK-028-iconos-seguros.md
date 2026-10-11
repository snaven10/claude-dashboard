# TASK-028 — Íconos que no dependan de una fuente de emoji

- **Plan:** PLAN-010 — Lista compacta
- **Especialista:** claude-dashboard-worker (sonnet)
- **Proyecto:** claude-dashboard (raíz del repo), rama `feature/plan-10-task-028` desde `main` (con TASK-027 mergeada)
- **Depende de:** TASK-027
- **Estado:** `pending`

---

## Objetivo
Ningún ícono del panel se ve como `▯`, en ningún navegador razonable, sin agregar fuentes ni
dependencias. Implementa DD-42 del master.
El usuario decidió (2026-10-11) hacerlo sin verificación previa, por robustez.

## Contexto verificado
- Caracteres en uso (master §2.4):
  - `web/index.html:34` `📊 historial`.
  - `web/app.js:478` `STATUS_ICON = { waiting: '⏳', blocked: '⏳', busy: '⚙', idle: '💤', done: '✔' }`
    (píldora del detalle, `app.js:1203`).
  - `web/app.js:1213` `📡 remoto`; `app.js:1275` `🔢 … tokens`; `app.js:949` `🔢` en el encabezado
    de plan; `app.js:1047`, `1312`, `1326` y `242` `📱` (bandeja, botón del detalle, ayuda, aviso
    de `--resume`).
  - `web/app.js:880` `⚡ ritmo inusual`; `⚠` en `app.js:121`, `159`, `882`, `884`.
  - `web/app.js:917`, `1031`, `1220` `⎇` (rama; tras TASK-024 puede quedar solo en el detalle/tooltip).
  - Controles `✕ ▾ ▸ ✓ “ −`: se quedan (fuentes de sistema comunes los tienen).
- `📊 📡 🔢 📱 ⏳ 💤 ⚡` son de presentación emoji; `⎇` (U+2387) falta en muchas fuentes. El `▯` lo
  vio el orquestador en Chromium headless (WSL, probablemente sin fuente de emoji de color).
- Regla del repo: nada de `innerHTML`; SVG con `document.createElementNS('http://www.w3.org/2000/svg', …)`.

## Archivos
- **Modificar:** `web/app.js`, `web/index.html`, `web/app.css`, `web/app.test.js` (solo si se
  agrega lógica pura testeable)

## Pasos
- [ ] **Paso 1 — Helper `icon(name)`.** Devuelve un `<svg>` de 1em, `aria-hidden="true"`,
  `focusable="false"`, `fill`/`stroke` = `currentColor`, con paths propios mínimos (dibujados a mano,
  sin copiar sets con licencia incompatible) para: `history` (barras), `remote` (antena/onda),
  `tokens` (#), `phone`, `branch`. Ningún texto de sesión pasa por el helper.
- [ ] **Paso 2 — Reemplazos.** Usar `icon()` donde estaban `📡 🔢 📱 ⎇`; quitar el ícono de la
  píldora de estado (queda el texto); `⚡`/`⚠` → `'⚡︎'`/`'⚠︎'` o SVG
  (`warn`, `bolt`). Los textos que se usan en `title`/avisos (no DOM) llevan palabras, no íconos.
- [ ] **Paso 3 — `index.html`.** El botón de historial lleva el SVG inline en el HTML estático +
  la palabra "historial".
- [ ] **Paso 4 — Tests.** Ajustar los tests que comparen cadenas con esos caracteres
  (`errorsView`, `formatContext`, `resumeWarning`) a lo nuevo.

## Criterios de aceptación
- [ ] `rg -n "📊|📡|🔢|📱|⏳|💤|⎇" web/` vacío (o solo en comentarios que expliquen el cambio).
- [ ] `node --check web/app.js`, `node --test web/` en verde; `rg -n innerHTML web/app.js` vacío.
- [ ] `gofmt -l .`, `go vet ./...`, `go test -race ./...` en verde.
- [ ] Captura en Chromium headless (el mismo que vio `▯`) con `?fixture=lista`, claro y oscuro: sin
  `▯`; los SVG toman el color del texto en ambos temas.
- [ ] Lectores de pantalla: los íconos son `aria-hidden` y cada botón conserva su nombre accesible.

## Resultado
<!-- SE LLENA AL CERRAR (estado done/skipped). Vacío mientras esté pending. -->
- **Estado final:**
- **Resumen:**
- **Archivos tocados:**
- **Verificado por:**
- **Desviaciones:**
- **Riesgos abiertos / siguiente:**
