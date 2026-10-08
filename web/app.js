(function () {
  'use strict';

  var BASE_TITLE = 'Claude Dashboard';
  var STORE_KEY = 'claude-dashboard.filters';
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

  function visible(list) {
    return list.filter(function (s) {
      var st = statusOf(s);
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

    var foot = el('div', 'card-foot');
    var btn = el('button', null, 'copiar comando');
    btn.type = 'button';
    btn.addEventListener('click', function () { copyText(copyCommand(s), btn); });
    foot.appendChild(btn);
    c.appendChild(foot);
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
    grid.replaceChildren.apply(grid, list.map(card));
    $('empty').hidden = list.length > 0;
  }

  function setSnapshot(data) {
    sessions = data && Array.isArray(data.sessions) ? data.sessions : [];
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
