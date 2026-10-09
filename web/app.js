(function () {
  'use strict';

  // ---- Lógica pura (sin DOM): exportada para `node --test` ----

  function isWaiting(st) { return st === 'waiting' || st === 'blocked'; }

  function sessionKey(s) { return s.sessionId || s.id; }

  // Sesiones a notificar entre dos snapshots: las que pasaron a waiting/blocked.
  // prev === null/undefined = primer snapshot de la página: no notifica nada (DD-12).
  function notifyTargets(prev, next) {
    if (!Array.isArray(prev) || !Array.isArray(next)) return [];
    var before = {};
    prev.forEach(function (s) { before[sessionKey(s)] = s.status; });
    return next.filter(function (s) {
      return isWaiting(s.status) && !isWaiting(before[sessionKey(s)]);
    });
  }

  // 1234 -> "1.2k", 1200000 -> "1.2M"
  function formatTokens(n) {
    if (typeof n !== 'number' || !isFinite(n) || n < 0) return '0';
    if (n < 1000) return String(Math.round(n));
    var units = [[1e9, 'G'], [1e6, 'M'], [1e3, 'k']];
    for (var i = 0; i < units.length; i++) {
      if (n >= units[i][0]) {
        var v = Math.round(n / units[i][0] * 10) / 10;
        // 999.96k redondea a 1000k: subimos de unidad
        if (v >= 1000 && i > 0) return formatTokens(units[i - 1][0]);
        return v.toFixed(1).replace(/\.0$/, '') + units[i][1];
      }
    }
    return String(n);
  }

  function totalTokens(t) {
    if (!t) return 0;
    return (t.input || 0) + (t.output || 0) + (t.cacheRead || 0) + (t.cacheCreation || 0);
  }

  // Agrupa por plan de orq. Las que no son de orq van al grupo final (plan = null).
  function groupByPlan(list) {
    var map = {};
    var order = [];
    list.forEach(function (s) {
      var plan = s.orq && s.orq.plan ? String(s.orq.plan) : null;
      var k = plan === null ? '' : plan;
      if (!Object.prototype.hasOwnProperty.call(map, k)) {
        map[k] = { plan: plan, sessions: [], waiting: 0 };
        order.push(k);
      }
      map[k].sessions.push(s);
      if (isWaiting(s.status)) map[k].waiting++;
    });
    var groups = order.map(function (k) { return map[k]; });
    groups.sort(function (a, b) {
      if (a.plan === null) return 1;
      if (b.plan === null) return -1;
      return (parseInt(a.plan, 10) || 0) - (parseInt(b.plan, 10) || 0);
    });
    return groups;
  }

  function orqLabel(o) {
    if (!o) return '';
    var parts = [];
    if (o.plan) parts.push('PLAN-' + o.plan);
    if (o.task) parts.push('TASK-' + o.task);
    if (o.specialist) parts.push(o.specialist);
    return parts.join(' · ');
  }

  // ---- PLAN-003 ----

  var DAY_MS = 24 * 60 * 60 * 1000;

  // DD-13: "contexto 412k · compactó 2×"; con warn, aviso explícito. null => ''.
  function formatContext(c) {
    if (!c || typeof c.current !== 'number') return '';
    if (c.warn) return '⚠ contexto ' + formatTokens(c.current) + ', cerca de compactar';
    var t = 'contexto ' + formatTokens(c.current);
    if (c.compactions > 0) t += ' · compactó ' + c.compactions + '×';
    return t;
  }

  function contextTitle(c) {
    if (!c) return '';
    var t = 'compactaciones: ' + (c.compactions || 0);
    if (typeof c.lastPreTokens === 'number') t += ' · la última vez compactó en ' + formatTokens(c.lastPreTokens);
    return t;
  }

  function rateTitle(r) {
    if (!r) return '';
    return 'En ' + (r.windowMin || 5) + ' min: ' + formatTokens(r.output || 0) + ' tokens de output y ' +
      (r.calls || 0) + ' llamadas';
  }

  // DD-16: suma por plan; tokens null cuenta como 0.
  function planTotals(sessions) {
    var all = 0;
    var out = 0;
    sessions.forEach(function (s) {
      all += totalTokens(s.tokens);
      out += s.tokens && s.tokens.output ? s.tokens.output : 0;
    });
    return { total: all, output: out };
  }

  // DD-15: done y última actividad (o inicio) de hace más de 24 h.
  function isOldDone(s, now) {
    if (!s || s.status !== 'done') return false;
    var t = s.lastActivityAt || s.startedAt;
    if (!t) return false;
    return now - t > DAY_MS;
  }

  function splitOldDone(list, now) {
    var recent = [];
    var old = [];
    list.forEach(function (s) { (isOldDone(s, now) ? old : recent).push(s); });
    return { recent: recent, old: old };
  }

  // DD-17: minúsculas y sin tildes.
  function normalizeText(x) {
    if (x == null) return '';
    return String(x).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
  }

  function matchesQuery(s, q) {
    var n = normalizeText(q).trim();
    if (!n) return true;
    var o = s.orq || {};
    var fields = [s.name, s.title, s.lastPrompt, s.project, o.plan, o.task];
    // "plan 3" / "PLAN-3" también deberían encontrar: sumamos las etiquetas
    fields.push(orqLabel(s.orq));
    return fields.some(function (f) { return normalizeText(f).indexOf(n) !== -1; });
  }

  // ---- PLAN-004 ----

  // DD-18 en el cliente también: el link va a un href y a un QR, no se confía en el server.
  var REMOTE_URL_RE = /^https:\/\/claude\.ai\/code\/session_[A-Za-z0-9]{10,64}$/;

  function validRemoteUrl(u) { return typeof u === 'string' && REMOTE_URL_RE.test(u); }

  // DD-20: 'qr' (botón con QR), 'enable' (interactiva sin link: indicar /remote-control)
  // o 'none' (background sin link: no se promete nada).
  function phoneMode(s) {
    if (!s) return 'none';
    if (validRemoteUrl(s.remoteUrl)) return 'qr';
    if (s.kind === 'interactive') return 'enable';
    return 'none';
  }

  // DD-20: --resume sobre una interactiva abierta crea una segunda copia de la conversación.
  function resumeWarning(s) {
    if (!s || s.kind !== 'interactive' || s.status === 'done') return '';
    var t = 'Ojo: si esta sesión sigue abierta, --resume abre una segunda copia de la conversación.';
    if (validRemoteUrl(s.remoteUrl)) t += ' Usá el QR (📱) para continuarla en el teléfono.';
    return t;
  }

  // Matriz de módulos oscuros (n x n) a partir de qrcode-generator. Solo usa
  // addData/make/getModuleCount/isDark: nunca los generadores de HTML de la librería (DD-19).
  function qrMatrix(url, factory) {
    var f = factory || (typeof qrcode !== 'undefined' ? qrcode : null);
    if (!f) return [];
    var qr = f(0, 'M');
    qr.addData(url);
    qr.make();
    var n = qr.getModuleCount();
    var rows = [];
    for (var r = 0; r < n; r++) {
      var row = [];
      for (var c = 0; c < n; c++) row.push(!!qr.isDark(r, c));
      rows.push(row);
    }
    return rows;
  }

  if (typeof document === 'undefined') {
    if (typeof module !== 'undefined' && module.exports) {
      module.exports = {
        notifyTargets: notifyTargets, formatTokens: formatTokens, totalTokens: totalTokens,
        groupByPlan: groupByPlan, orqLabel: orqLabel,
        formatContext: formatContext, contextTitle: contextTitle, rateTitle: rateTitle,
        planTotals: planTotals, isOldDone: isOldDone, splitOldDone: splitOldDone,
        normalizeText: normalizeText, matchesQuery: matchesQuery,
        validRemoteUrl: validRemoteUrl, phoneMode: phoneMode, resumeWarning: resumeWarning, qrMatrix: qrMatrix
      };
    }
    return;
  }

  var BASE_TITLE = 'Claude Dashboard';
  var STORE_KEY = 'claude-dashboard.filters';
  var NOTIFY_KEY = 'claude-dashboard.notify';
  var GROUP_KEY = 'claude-dashboard.groupByPlan';
  var OLD_KEY = 'claude-dashboard.showOldDone';
  var FIXTURE = new URLSearchParams(location.search).has('fixture');

  var STATUS_LABEL = {
    waiting: 'esperándote',
    blocked: 'esperándote',
    busy: 'trabajando',
    idle: 'idle',
    done: 'done'
  };
  var STATUS_ICON = { waiting: '⏳', blocked: '⏳', busy: '⚙', idle: '💤', done: '✔' };
  var RANK = { waiting: 0, blocked: 0, busy: 1, idle: 2, done: 3 };

  var $ = function (id) { return document.getElementById(id); };
  var grid = $('grid');
  var selProject = $('f-project');
  var chkDone = $('f-done');
  var chkIdle = $('f-idle');

  var sessions = [];
  var prevSessions = null; // null = todavía no hubo primer snapshot
  var groupByPlanOn = readFlag(GROUP_KEY);
  var showOldOn = readFlag(OLD_KEY);
  var query = '';
  var fixtureNow = null; // con ?fixture el "ahora" es generatedAt, para que el corte de 24 h no caduque
  var notifyOn = readFlag(NOTIFY_KEY);
  var lastFocus = null;
  var expanded = new Set();
  var filters = loadFilters();

  function loadFilters() {
    var f = { project: '', hideDone: false, hideIdle: false };
    try {
      var raw = localStorage.getItem(STORE_KEY);
      if (raw) {
        var p = JSON.parse(raw);
        f.project = typeof p.project === 'string' ? p.project : '';
        f.hideDone = !!p.hideDone;
        f.hideIdle = !!p.hideIdle;
      }
    } catch (e) { /* storage no disponible: seguimos con defaults */ }
    return f;
  }

  function readFlag(key) {
    try { return localStorage.getItem(key) === '1'; } catch (e) { return false; }
  }

  function writeFlag(key, on) {
    try { localStorage.setItem(key, on ? '1' : '0'); } catch (e) { /* ignorar */ }
  }

  function saveFilters() {
    try { localStorage.setItem(STORE_KEY, JSON.stringify(filters)); } catch (e) { /* ignorar */ }
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function ago(ms) {
    if (!ms) return '—';
    var s = Math.max(0, Math.floor((Date.now() - ms) / 1000));
    if (s < 60) return 'hace ' + s + ' s';
    var m = Math.floor(s / 60);
    if (m < 60) return 'hace ' + m + ' min';
    var h = Math.floor(m / 60);
    if (h < 24) return 'hace ' + h + ' h';
    return 'hace ' + Math.floor(h / 24) + ' d';
  }

  function shQuote(s) {
    return "'" + String(s).replace(/'/g, "'\\''") + "'";
  }

  function copyCommand(s) {
    if (s.kind === 'background') return 'claude attach ' + s.id;
    return 'cd ' + shQuote(s.cwd || '.') + ' && claude --resume ' + s.sessionId;
  }

  function copyText(text, btn) {
    var done = function () {
      var old = btn.textContent;
      btn.textContent = 'copiado ✓';
      setTimeout(function () { btn.textContent = old; }, 1500);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () { fallbackCopy(text, done); });
    } else {
      fallbackCopy(text, done);
    }
  }

  function fallbackCopy(text, done) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try { if (document.execCommand('copy')) done(); } catch (e) { /* ignorar */ }
    document.body.removeChild(ta);
  }

  function statusOf(s) {
    return Object.prototype.hasOwnProperty.call(STATUS_LABEL, s.status) ? s.status : 'idle';
  }

  function sorted(list) {
    return list.slice().sort(function (a, b) {
      var d = RANK[statusOf(a)] - RANK[statusOf(b)];
      if (d !== 0) return d;
      return (b.lastActivityAt || b.startedAt || 0) - (a.lastActivityAt || a.startedAt || 0);
    });
  }

  function nowMs() { return FIXTURE && fixtureNow ? fixtureNow : Date.now(); }

  function visible(list) {
    return list.filter(function (s) {
      var st = statusOf(s);
      if (!matchesQuery(s, query)) return false;
      if (filters.project && s.project !== filters.project) return false;
      if (filters.hideDone && st === 'done') return false;
      if (filters.hideIdle && st === 'idle') return false;
      return true;
    });
  }

  function card(s) {
    var st = statusOf(s);
    var c = el('article', 'card s-' + st);

    var head = el('div', 'card-head');
    head.appendChild(el('div', 'title', s.title || s.name || s.id));
    head.appendChild(el('span', 'badge s-' + st, STATUS_ICON[st] + ' ' + STATUS_LABEL[st]));
    c.appendChild(head);

    var meta = el('div', 'meta');
    meta.appendChild(el('span', null, s.project || '—'));
    if (s.name && s.name !== s.title) meta.appendChild(el('span', null, s.name));
    meta.appendChild(el('span', null, ago(s.lastActivityAt || s.startedAt)));
    if (s.remote) {
      var r = el('span', null, '📡 remoto');
      r.title = 'Sesión con bridge remoto';
      meta.appendChild(r);
    }
    c.appendChild(meta);

    if (s.stale) {
      c.appendChild(el('span', 'badge stale', '⚠ sin actividad ' + ago(s.lastActivityAt)));
    }

    if (s.orq) {
      var ol = orqLabel(s.orq);
      if (ol) {
        var ob = el('div', 'orq', ol);
        ob.title = 'Worker de orq' + (s.orq.ws ? ' · ws ' + s.orq.ws : '') + (s.orq.job ? ' · job ' + s.orq.job : '');
        c.appendChild(ob);
      }
    }

    if (s.waitingFor) c.appendChild(el('div', 'waiting-for', s.waitingFor));

    if (s.lastPrompt) {
      var p = el('p', 'prompt' + (expanded.has(s.id) ? ' open' : ''), s.lastPrompt);
      p.title = 'Click para expandir/colapsar';
      p.addEventListener('click', function () {
        if (expanded.has(s.id)) expanded.delete(s.id); else expanded.add(s.id);
        p.classList.toggle('open');
      });
      c.appendChild(p);
    }

    if (s.lastAction && s.lastAction.text) {
      var prefix = s.lastAction.kind === 'tool' ? '▸ ' : '“ ';
      c.appendChild(el('div', 'action', prefix + s.lastAction.text));
    }

    if (s.tokens) {
      var tk = s.tokens;
      var tt = el('div', 'tokens', '🔢 ' + formatTokens(totalTokens(tk)) + ' tokens' + (tk.model ? ' · ' + tk.model : ''));
      tt.title = 'input ' + (tk.input || 0) + ' · output ' + (tk.output || 0) +
        ' · cache leído ' + (tk.cacheRead || 0) + ' · cache creado ' + (tk.cacheCreation || 0);
      c.appendChild(tt);
    }

    var ctxText = formatContext(s.context);
    if (ctxText) {
      var cx = el('div', 'context' + (s.context.warn ? ' warn' : ''), ctxText);
      cx.title = contextTitle(s.context);
      c.appendChild(cx);
    }

    if (s.rate && s.rate.high) {
      var rb = el('span', 'badge rate', '⚡ ritmo inusual');
      rb.title = rateTitle(s.rate);
      c.appendChild(rb);
    }

    var mode = phoneMode(s);
    if (mode === 'enable') {
      c.appendChild(el('div', 'remote-hint', '📱 Para abrirla en el teléfono, activalo escribiendo /remote-control (o /rc) en esa sesión; el QR aparece solo en ~3 s.'));
    }

    var foot = el('div', 'card-foot');
    if (mode === 'qr') {
      var ph = el('button', null, '📱 abrir en el teléfono');
      ph.type = 'button';
      ph.addEventListener('click', function (ev) { ev.stopPropagation(); openPhone(s, ph); });
      foot.appendChild(ph);
    }
    var tl = el('button', null, 'línea de tiempo');
    tl.type = 'button';
    tl.addEventListener('click', function (ev) { ev.stopPropagation(); openTimeline(s, tl); });
    foot.appendChild(tl);
    var btn = el('button', null, 'copiar comando');
    btn.type = 'button';
    btn.addEventListener('click', function () { copyText(copyCommand(s), btn); });
    foot.appendChild(btn);
    c.appendChild(foot);
    var warn = resumeWarning(s);
    if (warn) c.appendChild(el('div', 'resume-warn', warn));
    c.addEventListener('click', function (ev) {
      if (ev.target.closest('button, .prompt, a')) return;
      openTimeline(s, null);
    });
    return c;
  }

  function updateProjects() {
    var names = Array.from(new Set(sessions.map(function (s) { return s.project; }).filter(Boolean))).sort();
    if (filters.project && names.indexOf(filters.project) === -1) names.push(filters.project);
    selProject.replaceChildren();
    var all = el('option', null, 'Todos');
    all.value = '';
    selProject.appendChild(all);
    names.forEach(function (n) {
      var o = el('option', null, n);
      o.value = n;
      selProject.appendChild(o);
    });
    selProject.value = filters.project;
  }

  function render() {
    var counts = { waiting: 0, busy: 0, idle: 0 };
    sessions.forEach(function (s) {
      var st = statusOf(s);
      if (st === 'waiting' || st === 'blocked') counts.waiting++;
      else if (st === 'busy') counts.busy++;
      else if (st === 'idle') counts.idle++;
    });
    $('n-waiting').textContent = counts.waiting;
    $('n-busy').textContent = counts.busy;
    $('n-idle').textContent = counts.idle;
    document.title = counts.waiting > 0 ? '(' + counts.waiting + ') esperándote — ' + BASE_TITLE : BASE_TITLE;

    var list = sorted(visible(sessions));
    var parts = splitOldDone(list, nowMs());
    var nodes = [];
    if (groupByPlanOn) {
      groupByPlan(parts.recent).forEach(function (g) {
        var sec = el('section', 'group');
        var h = el('h2', 'group-head', g.plan === null ? 'Sin plan' : 'PLAN-' + g.plan);
        h.appendChild(el('span', 'group-count', g.sessions.length + (g.waiting ? ' · ' + g.waiting + ' esperándote' : '')));
        var pt = planTotals(g.sessions);
        var tot = el('span', 'group-tokens', '🔢 ' + formatTokens(pt.total) + ' tokens · output ' + formatTokens(pt.output));
        tot.title = 'Suma de las sesiones que Claude todavía lista; las borradas con `claude rm` no cuentan.';
        h.appendChild(tot);
        sec.appendChild(h);
        var inner = el('div', 'grid');
        g.sessions.forEach(function (s) { inner.appendChild(card(s)); });
        sec.appendChild(inner);
        nodes.push(sec);
      });
      grid.className = 'groups';
    } else {
      grid.className = 'grid';
      nodes = parts.recent.map(card);
    }
    if (parts.old.length) {
      var old = el('section', 'old-done');
      old.style.gridColumn = '1 / -1';
      var tg = el('button', 'old-toggle', (showOldOn ? '▾ ' : '▸ ') + parts.old.length + ' terminadas hace más de 1 día');
      tg.type = 'button';
      tg.setAttribute('aria-expanded', showOldOn ? 'true' : 'false');
      tg.addEventListener('click', function () {
        showOldOn = !showOldOn;
        writeFlag(OLD_KEY, showOldOn);
        render();
      });
      old.appendChild(tg);
      if (showOldOn) {
        var og = el('div', 'grid');
        parts.old.forEach(function (s) { og.appendChild(card(s)); });
        old.appendChild(og);
      }
      nodes.push(old);
    }
    grid.replaceChildren.apply(grid, nodes);
    $('empty').hidden = list.length > 0;
  }

  // ---- Avisos (DD-12) ----

  function notifySupported() { return typeof Notification !== 'undefined'; }

  function updateNotifyBtn() {
    var b = $('btn-notify');
    if (!notifySupported()) { b.hidden = true; return; }
    var denied = Notification.permission === 'denied';
    b.disabled = denied;
    b.textContent = denied ? 'avisos bloqueados' : (notifyOn ? 'avisos: activados' : 'activar avisos');
    b.setAttribute('aria-pressed', notifyOn ? 'true' : 'false');
    if (denied) b.title = 'El navegador bloqueó las notificaciones para este sitio';
  }

  function toggleNotify() {
    if (!notifySupported()) return;
    if (notifyOn) {
      notifyOn = false;
      writeFlag(NOTIFY_KEY, false);
      updateNotifyBtn();
      return;
    }
    // el permiso se pide dentro del gesto del click
    Promise.resolve(Notification.requestPermission()).then(function (p) {
      notifyOn = p === 'granted';
      writeFlag(NOTIFY_KEY, notifyOn);
      updateNotifyBtn();
    });
  }

  function fireNotifications(targets) {
    if (!notifySupported() || !notifyOn || Notification.permission !== 'granted') return;
    targets.forEach(function (s) {
      try {
        var n = new Notification((s.title || s.name || s.id) + ' — esperándote', {
          body: (s.project ? s.project + ': ' : '') + (s.waitingFor || (s.lastAction && s.lastAction.text) || 'necesita tu atención'),
          tag: 'claude-dashboard-' + sessionKey(s)
        });
        n.onclick = function () { window.focus(); n.close(); };
      } catch (e) { /* ignorar */ }
    });
  }

  // ---- Abrir en el teléfono (DD-19/DD-20) ----

  var phoneFocus = null;

  function closePhone() {
    var p = $('phone');
    if (p.hidden) return;
    p.hidden = true;
    if (phoneFocus && phoneFocus.isConnected) phoneFocus.focus();
    phoneFocus = null;
  }

  // Pinta la matriz en un canvas: fondo blanco SIEMPRE y margen de 4 módulos (quiet zone).
  function drawQr(canvas, rows) {
    var quiet = 4;
    var cell = 6;
    var n = rows.length;
    var size = (n + quiet * 2) * cell;
    canvas.width = size;
    canvas.height = size;
    var ctx = canvas.getContext('2d');
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, size, size);
    ctx.fillStyle = '#000000';
    for (var r = 0; r < n; r++) {
      for (var c = 0; c < n; c++) {
        if (rows[r][c]) ctx.fillRect((c + quiet) * cell, (r + quiet) * cell, cell, cell);
      }
    }
  }

  function openPhone(s, opener) {
    if (!validRemoteUrl(s.remoteUrl)) return;
    var rows;
    try { rows = qrMatrix(s.remoteUrl); } catch (e) { rows = []; }
    var body = $('phone-body');
    body.replaceChildren();
    $('phone-title').textContent = 'Abrir en el teléfono — ' + (s.title || s.name || s.id);
    if (rows.length) {
      var canvas = el('canvas', 'qr');
      canvas.setAttribute('role', 'img');
      canvas.setAttribute('aria-label', 'Código QR con el link de la sesión');
      drawQr(canvas, rows);
      body.appendChild(canvas);
    } else {
      body.appendChild(el('p', 'empty', 'No se pudo generar el QR; usá el link.'));
    }
    body.appendChild(el('p', 'phone-help', 'Escaneá el QR con la cámara del teléfono, o abrí el link.'));
    var a = el('a', 'phone-link', s.remoteUrl);
    a.href = s.remoteUrl;
    a.target = '_blank';
    a.rel = 'noopener';
    body.appendChild(a);
    var cp = el('button', null, 'copiar link');
    cp.type = 'button';
    cp.addEventListener('click', function () { copyText(s.remoteUrl, cp); });
    body.appendChild(cp);

    phoneFocus = opener || document.activeElement;
    $('phone').hidden = false;
    $('phone-close').focus();
  }

  // ---- Línea de tiempo ----

  function closeTimeline() {
    var p = $('timeline');
    if (p.hidden) return;
    p.hidden = true;
    if (lastFocus && lastFocus.isConnected) lastFocus.focus();
    lastFocus = null;
  }

  function openTimeline(s, opener) {
    var p = $('timeline');
    lastFocus = opener || document.activeElement;
    $('timeline-title').textContent = s.title || s.name || s.id;
    var body = $('timeline-body');
    body.replaceChildren(el('p', 'empty', 'Cargando…'));
    p.hidden = false;
    $('timeline-close').focus();

    var url = FIXTURE ? 'fixture-timeline.json'
      : '/api/sessions/' + encodeURIComponent(s.sessionId) + '/timeline';
    fetch(url)
      .then(function (r) { if (!r.ok) throw new Error('http ' + r.status); return r.json(); })
      .then(function (d) {
        if (p.hidden || $('timeline-title').textContent !== (s.title || s.name || s.id)) return;
        var evs = d && Array.isArray(d.events) ? d.events : [];
        if (!evs.length) { body.replaceChildren(el('p', 'empty', 'Sin eventos.')); return; }
        var ol = el('ol', 'events');
        evs.forEach(function (ev) {
          var kind = ev && (ev.kind === 'prompt' || ev.kind === 'tool') ? ev.kind : 'text';
          var li = el('li', 'ev ev-' + kind);
          var when = el('time', 'ev-at', ev && ev.at ? new Date(ev.at).toLocaleTimeString() : '—');
          var tag = el('span', 'ev-kind', kind === 'prompt' ? 'prompt' : kind === 'tool' ? 'herramienta' : 'texto');
          li.appendChild(when);
          li.appendChild(tag);
          li.appendChild(el('div', 'ev-text', ev && ev.text != null ? String(ev.text) : ''));
          ol.appendChild(li);
        });
        body.replaceChildren(ol);
      })
      .catch(function () {
        if (!p.hidden) body.replaceChildren(el('p', 'empty', 'No se pudo cargar la línea de tiempo.'));
      });
  }

  function setSnapshot(data) {
    var next = data && Array.isArray(data.sessions) ? data.sessions : [];
    fireNotifications(notifyTargets(prevSessions, next));
    prevSessions = next;
    sessions = next;
    if (FIXTURE && data && typeof data.generatedAt === 'number') fixtureNow = data.generatedAt;
    updateProjects();
    render();
  }

  function setConn(on) {
    var c = $('conn');
    c.textContent = on ? (FIXTURE ? 'fixture' : 'conectado') : 'desconectado';
    c.className = 'conn ' + (on ? 'conn-on' : 'conn-off');
  }

  function connect() {
    var es = new EventSource('/api/events');
    es.onopen = function () { setConn(true); };
    es.addEventListener('snapshot', function (e) {
      try { setSnapshot(JSON.parse(e.data)); setConn(true); } catch (err) { /* evento inválido: ignorar */ }
    });
    es.onerror = function () {
      setConn(false);
      // EventSource reintenta solo salvo que el estado sea CLOSED.
      if (es.readyState === EventSource.CLOSED) {
        es.close();
        setTimeout(connect, 3000);
      }
    };
  }

  function init() {
    chkDone.checked = filters.hideDone;
    chkIdle.checked = filters.hideIdle;
    selProject.addEventListener('change', function () { filters.project = selProject.value; saveFilters(); render(); });
    chkDone.addEventListener('change', function () { filters.hideDone = chkDone.checked; saveFilters(); render(); });
    chkIdle.addEventListener('change', function () { filters.hideIdle = chkIdle.checked; saveFilters(); render(); });

    var chkGroup = $('f-group');
    chkGroup.checked = groupByPlanOn;
    chkGroup.addEventListener('change', function () { groupByPlanOn = chkGroup.checked; writeFlag(GROUP_KEY, groupByPlanOn); render(); });
    $('btn-notify').addEventListener('click', toggleNotify);
    updateNotifyBtn();
    $('phone-close').addEventListener('click', closePhone);
    $('phone').addEventListener('click', function (ev) { if (ev.target === $('phone')) closePhone(); });
    $('timeline-close').addEventListener('click', closeTimeline);
    $('timeline').addEventListener('click', function (ev) { if (ev.target === $('timeline')) closeTimeline(); });
    var search = $('f-search');
    search.addEventListener('input', function () { query = search.value; render(); });
    document.addEventListener('keydown', function (ev) {
      if (ev.key === 'Escape') {
        if (!$('phone').hidden) { closePhone(); return; }
        if (!$('timeline').hidden) { closeTimeline(); return; }
        if (search.value) { search.value = ''; query = ''; render(); }
        if (document.activeElement === search) search.blur();
        return;
      }
      if (ev.key === '/' && !ev.ctrlKey && !ev.metaKey && !ev.altKey) {
        var tag = document.activeElement && document.activeElement.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        ev.preventDefault();
        search.focus();
      }
    });

    // refresca los "hace X min" sin esperar un snapshot
    setInterval(render, 30000);

    if (FIXTURE) {
      fetch('fixture.json')
        .then(function (r) { return r.json(); })
        .then(function (d) { setSnapshot(d); setConn(true); })
        .catch(function () { setConn(false); $('empty').textContent = 'No se pudo cargar fixture.json (¿servido por HTTP?)'; $('empty').hidden = false; });
      return;
    }

    fetch('/api/sessions')
      .then(function (r) { return r.json(); })
      .then(setSnapshot)
      .catch(function () { /* el SSE traerá el snapshot */ });
    connect();
  }

  init();
})();
