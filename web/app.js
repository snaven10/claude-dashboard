(function () {
  'use strict';

  // ---- Lógica pura (sin DOM): exportada para `node --test` ----

  function isWaiting(st) { return st === 'waiting' || st === 'blocked'; }

  // DD-40: id de la conversación (sin PID). Dos procesos con el mismo sessionId comparten esto.
  function convoKey(s) { return s.sessionId || s.id; }

  // DD-40: clave única de fila: conversación + PID. Background terminadas pueden traer pid 0.
  function rowKey(s) { return convoKey(s) + (s.pid > 0 ? '#' + s.pid : ''); }

  // Resuelve una clave guardada a una sesión. Exacta primero; si la clave vieja no trae '#',
  // cae a la primera sesión con ese sessionId (migración de selecciones guardadas).
  function resolveSession(list, key) {
    if (!Array.isArray(list) || !key) return null;
    for (var i = 0; i < list.length; i++) if (rowKey(list[i]) === key) return list[i];
    if (key.indexOf('#') === -1) {
      for (var j = 0; j < list.length; j++) if (convoKey(list[j]) === key) return list[j];
    }
    return null;
  }

  // Sesiones a notificar entre dos snapshots: las que pasaron a waiting/blocked.
  // prev === null/undefined = primer snapshot de la página: no notifica nada (DD-12).
  function notifyTargets(prev, next) {
    if (!Array.isArray(prev) || !Array.isArray(next)) return [];
    var before = {};
    prev.forEach(function (s) { before[rowKey(s)] = s.status; });
    return next.filter(function (s) {
      return isWaiting(s.status) && !isWaiting(before[rowKey(s)]);
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

  // DD-25: la clave es plan.path si hay `plan`; si no, orq.plan como antes.
  function planKey(s) {
    if (s && s.plan && typeof s.plan.path === 'string' && s.plan.path) return 'path:' + s.plan.path;
    if (s && s.orq && s.orq.plan) return 'orq:' + String(s.orq.plan);
    return '';
  }

  function planNumberOf(s) {
    if (s && s.plan && typeof s.plan.number === 'number') return String(s.plan.number);
    if (s && s.orq && s.orq.plan) return String(s.orq.plan);
    return null;
  }

  // Agrupa por plan. Las sesiones sin plan van al grupo final (plan = null).
  // Cada grupo: { key, plan (número como texto), info (objeto `plan` del server o null), sessions, waiting }.
  function groupByPlan(list) {
    var map = {};
    var order = [];
    list.forEach(function (s) {
      var k = planKey(s);
      if (!Object.prototype.hasOwnProperty.call(map, k)) {
        map[k] = { key: k, plan: k === '' ? null : planNumberOf(s), info: null, sessions: [], waiting: 0 };
        order.push(k);
      }
      var g = map[k];
      if (g.info === null && s.plan && typeof s.plan.path === 'string') g.info = s.plan;
      g.sessions.push(s);
      if (isWaiting(s.status)) g.waiting++;
    });
    var groups = order.map(function (k) { return map[k]; });
    groups.sort(function (a, b) {
      if (a.key === '') return 1;
      if (b.key === '') return -1;
      var d = (parseInt(a.plan, 10) || 0) - (parseInt(b.plan, 10) || 0);
      if (d !== 0) return d;
      return a.key < b.key ? -1 : a.key > b.key ? 1 : 0;
    });
    return groups;
  }

  // "PLAN-42 · 14/28 done · 4 blocked · 2 in_progress"; sin avance, solo "PLAN-N".
  function planHeading(g) {
    var info = g && g.info;
    var head = g && g.plan !== null && g.plan !== undefined ? 'PLAN-' + g.plan : 'Sin plan';
    if (!info || typeof info.total !== 'number') return head;
    var doneTxt = (info.done || 0) + '/' + info.total + ' done';
    if (info.unmerged > 0) doneTxt += ' (' + info.unmerged + ' sin mergear)';
    var parts = [head, doneTxt];
    if (info.blocked > 0) parts.push(info.blocked + ' blocked');
    if (info.inProgress > 0) parts.push(info.inProgress + ' in_progress');
    return parts.join(' · ');
  }

  // Fracción 0..1 para la barra; sin total → 0.
  function planProgress(info) {
    if (!info || typeof info.total !== 'number' || info.total <= 0) return 0;
    return Math.max(0, Math.min(1, (info.done || 0) / info.total));
  }

  // DD-25: cómo mostrar `ask` según el estado. null = no se muestra.
  // mode 'big' (waiting/blocked) o 'small' (idle); busy/done nada.
  function askView(s) {
    if (!s || !s.ask || typeof s.ask.text !== 'string' || !s.ask.text) return null;
    var a = s.ask;
    var more = typeof a.more === 'number' && a.more > 0 ? a.more : 0;
    var moreText = more > 0 ? '+' + more + (more === 1 ? ' pregunta' : ' preguntas') : '';
    if (isWaiting(s.status)) {
      var label = a.kind === 'tool' ? 'pide permiso:' : a.kind === 'question' ? 'pregunta:' : 'último mensaje:';
      return { mode: 'big', label: label, text: a.text, more: moreText };
    }
    if (s.status === 'idle') return { mode: 'small', label: 'último mensaje:', text: a.text, more: moreText };
    return null;
  }

  // DD-41: motivo de espera en castellano. Nunca devuelve el `waitingFor` crudo de F1 si es genérico.
  var WAIT_LABELS = {
    'permission prompt': 'pide permiso en la terminal',
    'dialog open': 'tiene un diálogo abierto en la terminal',
    'input needed': 'espera tu respuesta'
  };
  function waitingLabel(wf) {
    var raw = plainText(wf);
    if (!raw) return 'necesita tu atención';
    return WAIT_LABELS[raw.toLowerCase()] || raw;
  }

  // -> { label, text, context, more }. Con `ask`: como askView en modo big. Sin `ask`: etiqueta por
  // waitingFor y contexto (última acción o último pedido); text queda ''.
  function waitView(s) {
    s = s || {};
    var a = s.ask;
    if (a && typeof a.text === 'string' && a.text) {
      var more = typeof a.more === 'number' && a.more > 0 ? a.more : 0;
      return {
        label: a.kind === 'tool' ? 'pide permiso:' : a.kind === 'question' ? 'pregunta:' : 'último mensaje:',
        text: plainText(a.text),
        context: '',
        more: more > 0 ? '+' + more + (more === 1 ? ' pregunta' : ' preguntas') : ''
      };
    }
    var context = '';
    var la = s.lastAction;
    if (la && typeof la.text === 'string' && oneLine(la.text)) {
      context = 'lo último: ▸ ' + oneLine(la.text);
    } else {
      var lp = plainText(s.lastPrompt);
      if (lp) context = 'tu último pedido: ' + (lp.length > 120 ? lp.slice(0, 119) + '…' : lp);
    }
    return { label: waitingLabel(s.waitingFor), text: '', context: context, more: '' };
  }

  // Texto plano completo (title, aviso): "label texto" o "label · contexto".
  function waitFull(v) {
    if (v.text) return v.label + ' ' + v.text + (v.more ? ' ' + v.more : '');
    return v.label + (v.context ? ' · ' + v.context : '');
  }

  // DD-22/DD-25: insignia solo con `high`; con count > 0 sin high, solo tooltip.
  function errorsView(e) {
    if (!e || typeof e.count !== 'number' || e.count <= 0) return null;
    var win = e.windowMin || 10;
    var title = e.count + (e.count === 1 ? ' error' : ' errores') + ' de herramientas en ' + win + ' min';
    return { badge: e.high ? '⚠ ' + e.count + ' errores en ' + win + ' min' : '', title: title };
  }

  // Número del favicon: '' = original (0), 1..9 tal cual, 10+ = '9+'.
  function faviconLabel(n) {
    if (typeof n !== 'number' || !isFinite(n) || n <= 0) return '';
    return n >= 10 ? '9+' : String(Math.floor(n));
  }

  // Filtro por estado del contador: tocar el activo lo quita. Valores: '', 'waiting', 'busy', 'idle'.
  var STATUS_FILTERS = ['waiting', 'busy', 'idle'];
  function toggleStatusFilter(current, clicked) {
    if (STATUS_FILTERS.indexOf(clicked) === -1) return current || '';
    return current === clicked ? '' : clicked;
  }

  function matchesStatus(s, f) {
    if (!f) return true;
    if (f === 'waiting') return s.status === 'waiting' || s.status === 'blocked';
    return s.status === f;
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
    var fields = [s.name, s.title, s.lastPrompt, s.project, o.plan, o.task, s.branch, s.plan && s.plan.name];
    // "plan 3" / "PLAN-3" también deberían encontrar: sumamos las etiquetas
    fields.push(orqLabel(s.orq));
    // PLAN-008: "PR #87", "87" (vía "#87") y el repo del PR
    validPrs(s).forEach(function (p) { fields.push('PR #' + p.number, 'PR ' + p.number, p.repo); });
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

  // ---- Historial (DD-29) ----

  // Escala de barras: el máximo de `output` del período = 100 %. Los días en 0 no tienen
  // barra (pct 0) pero conservan su lugar. Todo en 0 → todo pct 0 (sin dividir por cero).
  function historyScale(days) {
    var list = Array.isArray(days) ? days : [];
    var max = 0;
    list.forEach(function (d) { if (d && typeof d.output === 'number' && d.output > max) max = d.output; });
    return list.map(function (d) {
      var out = d && typeof d.output === 'number' && d.output > 0 ? d.output : 0;
      return { date: d && d.date ? String(d.date) : '', output: out, pct: max > 0 ? out / max * 100 : 0 };
    });
  }

  // Día elegido: el de `date` si existe; si no, el último (hoy). null si no hay días.
  function selectDay(days, date) {
    if (!Array.isArray(days) || !days.length) return null;
    for (var i = 0; i < days.length; i++) if (days[i] && days[i].date === date) return days[i];
    return days[days.length - 1];
  }

  // Filas de una tabla por modelo/proyecto; `share` = fracción del total del día (0 si total 0).
  function historyRows(list) {
    if (!Array.isArray(list)) return [];
    var sum = 0;
    list.forEach(function (r) { if (r && typeof r.total === 'number') sum += r.total; });
    return list.filter(Boolean).map(function (r) {
      var total = typeof r.total === 'number' ? r.total : 0;
      return { name: r.name != null ? String(r.name) : '', output: typeof r.output === 'number' ? r.output : 0,
        total: total, share: sum > 0 ? total / sum : 0 };
    });
  }

  // ---- PLAN-007: bandeja ----

  // DD-30: waiting/blocked con última actividad (o inicio) de hace MÁS de 24 h. Presentación, no borrado.
  function isAbandoned(s, now) {
    if (!s || !isWaiting(s.status)) return false;
    var t = s.lastActivityAt || s.startedAt;
    if (!t) return false;
    return now - t > DAY_MS;
  }

  // Contadores del encabezado/favicon/título: las abandonadas no cuentan como "esperándote".
  function counts(list, now) {
    var c = { waiting: 0, busy: 0, idle: 0 };
    (Array.isArray(list) ? list : []).forEach(function (s) {
      if (isWaiting(s.status)) { if (!isAbandoned(s, now)) c.waiting++; }
      else if (s.status === 'busy') c.busy++;
      else if (s.status !== 'done') c.idle++;
    });
    return c;
  }

  // DD-32: franja "Te esperan": waiting/blocked no abandonadas, la más reciente primero.
  function inboxItems(list, now) {
    return (Array.isArray(list) ? list : []).filter(function (s) {
      return isWaiting(s.status) && !isAbandoned(s, now);
    }).sort(function (a, b) {
      return (b.lastActivityAt || b.startedAt || 0) - (a.lastActivityAt || a.startedAt || 0);
    });
  }

  function splitAbandoned(list, now) {
    var live = [];
    var abandoned = [];
    list.forEach(function (s) { (isAbandoned(s, now) ? abandoned : live).push(s); });
    return { live: live, abandoned: abandoned };
  }

  // DD-38/DD-39: grupos de la lista por estado. Las waiting/blocked NO abandonadas no entran
  // (viven solo en la bandeja); las abandonadas van a su grupo. Sin grupos vacíos. Dentro de
  // "Terminadas" las recientes (< 24 h) van antes que las viejas, conservando el orden de entrada.
  var GROUP_LABEL = { busy: 'Trabajando', idle: 'Idle', done: 'Terminadas', abandoned: 'Abandonadas' };

  function statusGroups(list, now) {
    var by = { busy: [], idle: [], done: [], abandoned: [] };
    var oldDone = [];
    (Array.isArray(list) ? list : []).forEach(function (s) {
      if (isWaiting(s.status)) {
        if (isAbandoned(s, now)) by.abandoned.push(s);
      } else if (s.status === 'busy') {
        by.busy.push(s);
      } else if (s.status === 'done') {
        (isOldDone(s, now) ? oldDone : by.done).push(s);
      } else {
        by.idle.push(s);
      }
    });
    by.done = by.done.concat(oldDone);
    return ['busy', 'idle', 'done', 'abandoned'].filter(function (id) {
      return by[id].length > 0;
    }).map(function (id) {
      return { id: id, label: GROUP_LABEL[id], sessions: by[id] };
    });
  }

  // DD-40: colapso de repetidos. Devuelve [{ head, copies }] en el orden de entrada (la entrada
  // ocupa el lugar del primer miembro). `copies` son TODOS los miembros del grupo (incluida la
  // head), en orden de entrada; N del chip "×N" = copies.length. head = la de actividad más
  // reciente. Solo colapsa en idle/done/abandoned, con mismo cwd y mismo título NO vacío
  // (s.title, nunca el respaldo name/id) normalizado.
  var COLLAPSIBLE = { idle: true, done: true, abandoned: true };

  function repeatIdentity(s) {
    if (!s || !s.cwd) return '';
    var t = normalizeText(plainText(s.title)).trim();
    return t ? s.cwd + '\u0000' + t : '';
  }

  function collapseRepeats(sessions, groupId) {
    var list = Array.isArray(sessions) ? sessions : [];
    var can = !!COLLAPSIBLE[groupId];
    var out = [];
    var byId = {};
    list.forEach(function (s) {
      var id = can ? repeatIdentity(s) : '';
      if (id && Object.prototype.hasOwnProperty.call(byId, id)) {
        byId[id].copies.push(s);
        return;
      }
      var e = { head: s, copies: [s] };
      out.push(e);
      if (id) byId[id] = e;
    });
    var at = function (s) { return s.lastActivityAt || s.startedAt || 0; };
    out.forEach(function (e) {
      e.copies.forEach(function (c) { if (at(c) > at(e.head)) e.head = c; });
    });
    return out;
  }

  // Id de grupo de estado de una sesión (como statusGroups), o '' si vive solo en la bandeja.
  function groupIdOf(s, now) {
    if (isWaiting(s.status)) return isAbandoned(s, now) ? 'abandoned' : '';
    if (s.status === 'busy') return 'busy';
    if (s.status === 'done') return 'done';
    return 'idle';
  }

  // Colapso sobre una lista que mezcla estados (agrupar por plan): colapsa por estado y
  // devuelve las entradas en el orden de aparición del primer miembro.
  function collapseMixed(sessions, now) {
    var list = Array.isArray(sessions) ? sessions : [];
    var pos = new Map();
    list.forEach(function (s, i) { pos.set(s, i); });
    var buckets = {};
    var order = [];
    list.forEach(function (s) {
      var g = groupIdOf(s, now);
      if (!buckets[g]) { buckets[g] = []; order.push(g); }
      buckets[g].push(s);
    });
    var out = [];
    order.forEach(function (g) {
      collapseRepeats(buckets[g], g).forEach(function (e) {
        out.push({ e: e, i: Math.min.apply(null, e.copies.map(function (c) { return pos.get(c); })) });
      });
    });
    return out.sort(function (a, b) { return a.i - b.i; }).map(function (x) { return x.e; });
  }

  // Números del encabezado de la página: terminadas (todas) y abandonadas.
  function listTotals(list, now) {
    var t = { done: 0, abandoned: 0 };
    (Array.isArray(list) ? list : []).forEach(function (s) {
      if (isAbandoned(s, now)) t.abandoned++;
      else if (s && s.status === 'done') t.done++;
    });
    return t;
  }

  // DD-34: j/k sobre la lista visible; en los bordes se queda (no da la vuelta).
  // Sin selección válida cae en el primero. Lista vacía → null.
  function stepSelection(keys, current, delta) {
    if (!Array.isArray(keys) || !keys.length) return null;
    var i = keys.indexOf(current);
    if (i === -1) return keys[0];
    return keys[Math.max(0, Math.min(keys.length - 1, i + delta))];
  }

  // DD-33: saca el markdown más ruidoso. El resto (incluido HTML hostil) queda como texto literal:
  // quien lo pinta usa textContent.
  function plainText(s) {
    if (s == null) return '';
    return String(s)
      .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/^[ \t]{0,3}#{1,6}[ \t]+/gm, '')
      .replace(/\*\*/g, '')
      .replace(/__/g, '')
      .replace(/`/g, '')
      .replace(/\s+/g, ' ')
      .replace(/(^|\s)#{2,6}\s+/g, '$1') // "## x" en medio: el backend ya juntó las líneas; un "#" suelto no es encabezado
      .trim();
  }

  function oneLine(s) {
    return s == null ? '' : String(s).replace(/\s+/g, ' ').trim();
  }

  // DD-32: "hoy X output · N sesiones" del último día de /api/history; null si no está listo.
  function todayView(d) {
    if (!d || d.ready === false || !Array.isArray(d.days) || !d.days.length) return null;
    var last = d.days[d.days.length - 1];
    if (!last) return null;
    return { output: formatTokens(last.output), sessions: typeof last.sessions === 'number' ? last.sessions : 0 };
  }

  // ---- PLAN-008: datos de sesión (DD-36) ----

  var TURN_WARN_MIN = 15;
  var PR_URL_RE = /^https:\/\/[^\s"'<>\\]+$/;
  var ARTIFACT_URL_RE = /^https:\/\/claude\.ai\/[^\s"'<>\\]*$/;

  // El server ya valida, pero la URL va a un href: se revalida en el cliente.
  function validPrUrl(u) { return typeof u === 'string' && PR_URL_RE.test(u); }
  function validArtifactUrl(u) { return typeof u === 'string' && ARTIFACT_URL_RE.test(u); }

  // PRs con número entero > 0 y URL https válida; el resto se descarta.
  function validPrs(s) {
    var list = s && Array.isArray(s.prs) ? s.prs : [];
    return list.filter(function (p) {
      return p && typeof p.number === 'number' && isFinite(p.number) && p.number > 0 &&
        Math.floor(p.number) === p.number && validPrUrl(p.url);
    });
  }

  function validArtifacts(s) {
    var list = s && Array.isArray(s.artifacts) ? s.artifacts : [];
    return list.filter(function (a) { return a && validArtifactUrl(a.url); });
  }

  // Minutos del turno en curso; null si no está busy o no hay dato.
  function turnMinutes(s, now) {
    if (!s || s.status !== 'busy' || typeof s.turnStartedAt !== 'number' || !isFinite(s.turnStartedAt)) return null;
    return Math.max(0, Math.floor((now - s.turnStartedAt) / 60000));
  }

  // Línea secundaria de la fila: away.text si hay y la sesión no está busy. '' = no aplica.
  function awayLine(s) {
    if (!s || s.status === 'busy' || !s.away || typeof s.away.text !== 'string') return '';
    return plainText(s.away.text);
  }

  function activeSubagents(s) {
    var n = s && s.subagents && s.subagents.active;
    return typeof n === 'number' && n > 0 ? Math.floor(n) : 0;
  }

  function hasLines(s) {
    var l = s && s.lines;
    return !!l && typeof l.added === 'number' && typeof l.removed === 'number' && (l.added > 0 || l.removed > 0);
  }

  function permissionChip(s) {
    var m = s && s.permissionMode;
    return typeof m === 'string' && /^[A-Za-z]{1,32}$/.test(m) && m !== 'default' ? m : '';
  }

  // Chips de datos nuevos, en el orden de la maqueta: subagentes, PR, líneas, turno, permisos.
  // Cada uno: { cls, text, title, lines? }.
  function sessionChips(s, now) {
    var out = [];
    var sub = activeSubagents(s);
    if (sub > 0) {
      out.push({ cls: 'sub', text: sub + (sub === 1 ? ' subagente' : ' subagentes'),
        title: 'Subagentes activos ahora (' + (s.subagents.total || 0) + ' en total)' });
    }
    var prs = validPrs(s);
    if (prs.length) out.push({ cls: 'pr', text: 'PR #' + prs[prs.length - 1].number, title: prs[prs.length - 1].repo || '' });
    if (hasLines(s)) {
      out.push({ cls: 'lines num', text: '+' + s.lines.added + ' −' + s.lines.removed, lines: s.lines,
        title: 'Líneas agregadas y quitadas en la sesión' });
    }
    var tm = turnMinutes(s, now);
    if (tm !== null && tm >= TURN_WARN_MIN) out.push({ cls: 'warn', text: 'turno de ' + tm + ' min', title: 'Hace ' + tm + ' min que Claude trabaja en este turno' });
    var perm = permissionChip(s);
    if (perm) out.push({ cls: '', text: perm, title: 'Modo de permisos' });
    return out;
  }

  // ---- PLAN-010: fila de una línea (DD-37) ----

  var ROW_STATUS_TEXT = { waiting: 'esperándote', blocked: 'esperándote', busy: 'trabajando', idle: 'idle', done: 'done' };

  // "hace 5 min" relativo a `now`; sin fecha, "—".
  function agoFrom(ms, now) {
    if (!ms) return '—';
    var s = Math.max(0, Math.floor((now - ms) / 1000));
    if (s < 60) return 'hace ' + s + ' s';
    var m = Math.floor(s / 60);
    if (m < 60) return 'hace ' + m + ' min';
    var h = Math.floor(m / 60);
    if (h < 24) return 'hace ' + h + ' h';
    return 'hace ' + Math.floor(h / 24) + ' d';
  }

  // Prioridad (menor = primero): errores > contexto warn > ritmo high > stale > turno largo > PR >
  // subagentes > líneas > permisos. Cada candidato: { cls, text, title, lines?, pri }.
  var CHIP_PRI_BY_CLS = { warn: 4, pr: 5, sub: 6, 'lines num': 7, '': 8 };

  function rowChipCandidates(s, now) {
    var out = [];
    var ev = errorsView(s.errors);
    if (ev && ev.badge) out.push({ cls: 'err', text: ev.badge, title: ev.title, pri: 0 });
    if (s.context && s.context.warn) {
      out.push({ cls: 'warn', text: '⚠ contexto ' + formatTokens(s.context.current), title: contextTitle(s.context), pri: 1 });
    }
    if (s.rate && s.rate.high) out.push({ cls: 'warn', text: '⚡ ritmo inusual', title: rateTitle(s.rate), pri: 2 });
    if (s.stale) {
      var when = agoFrom(s.lastActivityAt, now);
      out.push({ cls: 'warn', text: '⚠ sin actividad ' + when, title: 'Sin actividad ' + when, pri: 3 });
    }
    sessionChips(s, now).forEach(function (c) {
      var p = Object.prototype.hasOwnProperty.call(CHIP_PRI_BY_CLS, c.cls) ? CHIP_PRI_BY_CLS[c.cls] : 9;
      var o = { cls: c.cls, text: c.text, title: c.title, pri: p };
      if (c.lines) o.lines = c.lines;
      out.push(o);
    });
    // orden estable por prioridad
    return out.map(function (c, i) { return { c: c, i: i }; })
      .sort(function (a, b) { return a.c.pri - b.c.pri || a.i - b.i; })
      .map(function (x) { return x.c; });
  }

  // Todo lo que la fila de una línea necesita. opts: { now, projectFilter, maxChips (default 2) }.
  // Texto de sesiones siempre pasa por plainText; quien pinta usa textContent.
  function rowView(s, opts) {
    opts = opts || {};
    var now = typeof opts.now === 'number' ? opts.now : Date.now();
    var max = typeof opts.maxChips === 'number' && opts.maxChips >= 0 ? Math.floor(opts.maxChips) : 2;
    var st = Object.prototype.hasOwnProperty.call(ROW_STATUS_TEXT, s.status) ? s.status : 'idle';
    var title = plainText(s.title || s.name || s.id);
    var projText = plainText(s.project);
    var project = opts.projectFilter ? null : (projText || null);

    var all = rowChipCandidates(s, now);
    // DD-40: "×N" entra primero y cuenta para el tope de chips.
    var copies = typeof opts.copies === 'number' ? Math.floor(opts.copies) : 0;
    if (copies > 1) {
      all.unshift({ cls: 'rep', text: '×' + copies, pri: -1,
        title: copies + ' sesiones con el mismo título en esta carpeta' });
    }
    var chips = all.slice(0, max);
    var rest = all.slice(max);

    var av = st === 'idle' ? askView(s) : null;
    var tooltip = awayLine(s) || (av ? plainText(av.text) : '');
    if (!tooltip) tooltip = [projText, s.branch ? '⎇ ' + plainText(s.branch) : null].filter(Boolean).join(' · ');

    var when = agoFrom(s.lastActivityAt || s.startedAt, now);
    return {
      status: st,
      title: title,
      project: project,
      chips: chips,
      extra: rest.length,
      extraTitle: rest.map(function (c) { return c.text; }).join(' · '),
      when: when,
      tooltip: tooltip,
      ariaLabel: title + ', ' + ROW_STATUS_TEXT[st] + ', ' + when
    };
  }

  if (typeof document === 'undefined') {
    if (typeof module !== 'undefined' && module.exports) {
      module.exports = {
        validPrUrl: validPrUrl, validArtifactUrl: validArtifactUrl, validPrs: validPrs, validArtifacts: validArtifacts,
        turnMinutes: turnMinutes, awayLine: awayLine, sessionChips: sessionChips,
        rowView: rowView, agoFrom: agoFrom,
        isAbandoned: isAbandoned, counts: counts, inboxItems: inboxItems, splitAbandoned: splitAbandoned,
        statusGroups: statusGroups, listTotals: listTotals,
        rowKey: rowKey, resolveSession: resolveSession, collapseRepeats: collapseRepeats, collapseMixed: collapseMixed,
        stepSelection: stepSelection, plainText: plainText, oneLine: oneLine, todayView: todayView,
        notifyTargets: notifyTargets, formatTokens: formatTokens, totalTokens: totalTokens,
        groupByPlan: groupByPlan, orqLabel: orqLabel, planKey: planKey, planHeading: planHeading, planProgress: planProgress,
        askView: askView, waitView: waitView, waitingLabel: waitingLabel, waitFull: waitFull, errorsView: errorsView, faviconLabel: faviconLabel,
        toggleStatusFilter: toggleStatusFilter, matchesStatus: matchesStatus,
        formatContext: formatContext, contextTitle: contextTitle, rateTitle: rateTitle,
        planTotals: planTotals, isOldDone: isOldDone, splitOldDone: splitOldDone,
        normalizeText: normalizeText, matchesQuery: matchesQuery,
        historyScale: historyScale, selectDay: selectDay, historyRows: historyRows,
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
  // ?fixture=lista carga fixture-lista.json (43 sesiones); cualquier otro valor, fixture.json.
  var FIXTURE_FILE = new URLSearchParams(location.search).get('fixture') === 'lista' ? 'fixture-lista.json' : 'fixture.json';

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
    var f = { project: '', hideDone: false, hideIdle: false, status: '' };
    try {
      var raw = localStorage.getItem(STORE_KEY);
      if (raw) {
        var p = JSON.parse(raw);
        f.project = typeof p.project === 'string' ? p.project : '';
        f.hideDone = !!p.hideDone;
        f.hideIdle = !!p.hideIdle;
        f.status = STATUS_FILTERS.indexOf(p.status) !== -1 ? p.status : '';
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
    return agoFrom(ms, nowMs());
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
      if (!matchesStatus({ status: st }, filters.status)) return false;
      return true;
    });
  }


  // ---- Favicon con número (DD-25) ----

  var faviconLink = document.querySelector('link[rel="icon"]');
  var faviconOriginal = faviconLink ? faviconLink.getAttribute('href') : '';
  var faviconShown = ''; // etiqueta dibujada; '' = original

  function drawFavicon(label) {
    var cv = document.createElement('canvas');
    cv.width = 64;
    cv.height = 64;
    var ctx = cv.getContext('2d');
    if (!ctx) return null;
    ctx.fillStyle = '#d97757';
    ctx.beginPath();
    ctx.arc(32, 32, 28, 0, Math.PI * 2);
    ctx.fill();
    var w = label.length > 1 ? 40 : 34;
    ctx.fillStyle = '#dc2626';
    ctx.beginPath();
    ctx.arc(64 - w / 2, w / 2, w / 2, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = '#ffffff';
    ctx.font = 'bold ' + (label.length > 1 ? 24 : 28) + 'px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(label, 64 - w / 2, w / 2 + 2);
    return cv.toDataURL('image/png');
  }

  // Solo regenera si el número cambió (render corre cada 30 s y en cada snapshot).
  function updateFavicon(n) {
    if (!faviconLink) return;
    var label = faviconLabel(n);
    if (label === faviconShown) return;
    faviconShown = label;
    if (label === '') { faviconLink.setAttribute('href', faviconOriginal); faviconLink.removeAttribute('type'); return; }
    var url;
    try { url = drawFavicon(label); } catch (e) { url = null; }
    if (url) { faviconLink.setAttribute('type', 'image/png'); faviconLink.setAttribute('href', url); }
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
          body: (s.project ? s.project + ': ' : '') + waitFull(waitView(s)),
          tag: 'claude-dashboard-' + rowKey(s)
        });
        n.onclick = function () { window.focus(); n.close(); };
      } catch (e) { /* ignorar */ }
    });
  }


  // ---- Historial ----

  var historyTimer = null;
  var historyDate = null;
  var historyData = null;

  function closeHistory() {
    var p = $('history');
    if (p.hidden) return;
    p.hidden = true;
    if (historyTimer) { clearTimeout(historyTimer); historyTimer = null; }
    if (lastFocus && lastFocus.isConnected) lastFocus.focus();
    lastFocus = null;
  }

  function historyTable(title, rows) {
    var wrap = el('div', 'hist-table');
    wrap.appendChild(el('h3', null, title));
    if (!rows.length) { wrap.appendChild(el('p', 'empty', 'Sin datos.')); return wrap; }
    var t = el('table');
    var hr = el('tr');
    ['Nombre', 'Output', 'Total'].forEach(function (h) { hr.appendChild(el('th', null, h)); });
    t.appendChild(hr);
    rows.forEach(function (r) {
      var tr = el('tr');
      tr.appendChild(el('td', 'hist-name', r.name));
      tr.appendChild(el('td', 'num', formatTokens(r.output)));
      tr.appendChild(el('td', 'num', formatTokens(r.total) + ' (' + Math.round(r.share * 100) + '%)'));
      t.appendChild(tr);
    });
    wrap.appendChild(t);
    return wrap;
  }

  function renderHistory(d) {
    var body = $('history-body');
    var days = d && Array.isArray(d.days) ? d.days : [];
    if (!d || d.ready === false || !days.length) {
      body.replaceChildren(el('p', 'empty', 'calculando historial…'));
      if (!FIXTURE || (d && d.ready === false)) {
        historyTimer = setTimeout(function () { historyTimer = null; if (!$('history').hidden) loadHistory(); }, 3000);
      }
      return;
    }
    var sel = selectDay(days, historyDate);
    historyDate = sel.date;
    var today = days[days.length - 1].date;
    var scale = historyScale(days);

    var chart = el('div', 'hist-chart');
    chart.setAttribute('role', 'group');
    chart.setAttribute('aria-label', 'Output por día, últimos ' + days.length + ' días');
    scale.forEach(function (b) {
      var btn = el('button', 'hist-bar' + (b.date === sel.date ? ' selected' : '') + (b.date === today ? ' today' : ''));
      btn.type = 'button';
      btn.title = b.date + ': ' + formatTokens(b.output) + ' output' + (b.date === today ? ' (hoy)' : '');
      btn.setAttribute('aria-label', btn.title);
      btn.setAttribute('aria-pressed', b.date === sel.date ? 'true' : 'false');
      var fill = el('span', 'hist-fill');
      fill.style.height = b.pct + '%';
      btn.appendChild(fill);
      btn.addEventListener('click', function () { historyDate = b.date; renderHistory(historyData); });
      chart.appendChild(btn);
    });

    var tot = el('p', 'hist-totals');
    tot.textContent = sel.date + (sel.date === today ? ' (hoy)' : '') + ' · output ' + formatTokens(sel.output) +
      ' · input ' + formatTokens(sel.input) + ' · cache lectura ' + formatTokens(sel.cacheRead) +
      ' · cache creación ' + formatTokens(sel.cacheCreation) + ' · ' + (sel.messages || 0) + ' mensajes · ' +
      (sel.sessions || 0) + ' sesiones';
    var tables = el('div', 'hist-tables');
    tables.appendChild(historyTable('Por modelo', historyRows(sel.byModel)));
    tables.appendChild(historyTable('Por proyecto', historyRows(sel.byProject)));
    body.replaceChildren(chart, tot, tables);
  }

  function loadHistory() {
    var url = FIXTURE ? 'fixture-history.json' : '/api/history';
    fetch(url)
      .then(function (r) { if (!r.ok) throw new Error('http ' + r.status); return r.json(); })
      .then(function (d) { if ($('history').hidden) return; historyData = d; renderHistory(d); })
      .catch(function () {
        if ($('history').hidden) return;
        $('history-body').replaceChildren(el('p', 'empty', 'No se pudo cargar el historial.'));
      });
  }

  function openHistory(opener) {
    lastFocus = opener || document.activeElement;
    historyDate = null;
    historyData = null;
    $('history-body').replaceChildren(el('p', 'empty', 'Cargando…'));
    $('history').hidden = false;
    $('history-close').focus();
    loadHistory();
  }

  function setSnapshot(data) {
    var next = data && Array.isArray(data.sessions) ? data.sessions : [];
    if (FIXTURE && data && typeof data.generatedAt === 'number') fixtureNow = data.generatedAt;
    var now = nowMs();
    // DD-30: las abandonadas no avisan
    fireNotifications(notifyTargets(prevSessions, next).filter(function (s) { return !isAbandoned(s, now); }));
    prevSessions = next;
    sessions = next;
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


  // ---- Bandeja (PLAN-007) ----

  var SEL_KEY = 'claude-dashboard.selected';
  var ABN_KEY = 'claude-dashboard.showAbandoned';
  var showAbandonedOn = readFlag(ABN_KEY);
  var selectedKey = readSelected();
  var phoneDetail = false; // teléfono: pantalla de detalle abierta
  var qrOpenFor = ''; // clave de la sesión con el QR desplegado en el detalle
  var navKeys = []; // claves en el orden en que se ven (bandeja primero, luego filas) para j/k
  var inboxKeys = []; // claves de las tarjetas de la bandeja, en su orden (DD-39)
  var detailSig = '';
  var historyToday = null;
  var timeline = { key: '', at: 0, status: 'idle', events: [], req: 0, version: 0 };

  function readSelected() {
    try { return localStorage.getItem(SEL_KEY) || ''; } catch (e) { return ''; }
  }

  function saveSelected(k) {
    try { localStorage.setItem(SEL_KEY, k); } catch (e) { /* ignorar */ }
  }

  function isPhone() {
    return typeof window.matchMedia === 'function' && window.matchMedia('(max-width: 760px)').matches;
  }

  function titleOf(s) { return plainText(s.title || s.name || s.id); }

  function btn(label, cls) {
    var b = el('button', cls || null, label);
    b.type = 'button';
    return b;
  }

  function chip(cls, text, title) {
    var c = el('span', 'chip ' + cls, text);
    if (title) c.title = title;
    return c;
  }

  // Alertas de una fila. En el detalle el contexto va en su propia línea, no como chip.
  function chipsFor(s, inDetail) {
    var w = el('div', 'chips');
    var ev = errorsView(s.errors);
    if (ev && ev.badge) w.appendChild(chip('err', ev.badge, ev.title));
    if (s.rate && s.rate.high) w.appendChild(chip('warn', '⚡ ritmo inusual', rateTitle(s.rate)));
    if (!inDetail && s.context && s.context.warn) {
      w.appendChild(chip('warn', '⚠ contexto ' + formatTokens(s.context.current), contextTitle(s.context)));
    }
    if (s.stale) w.appendChild(chip('warn', '⚠ sin actividad ' + ago(s.lastActivityAt)));
    sessionChips(s, nowMs()).forEach(function (c) {
      var n;
      if (c.lines) {
        n = el('span', 'chip ' + c.cls);
        n.appendChild(el('span', 'add', '+' + c.lines.added));
        n.appendChild(document.createTextNode(' '));
        n.appendChild(el('span', 'del', '−' + c.lines.removed));
        n.title = c.title;
      } else {
        n = chip(c.cls, c.text, c.title);
      }
      w.appendChild(n);
    });
    return w.childNodes.length ? w : null;
  }

  // Entradas del colapso del último renderList: clave de cualquier miembro → entrada { head, copies }.
  // Solo las que tienen más de una copia. Lo usan el detalle ("Copias") y j/k.
  var collapsedBy = {};

  function noteEntry(e) {
    if (e.copies.length > 1) e.copies.forEach(function (c) { collapsedBy[rowKey(c)] = e; });
  }

  // Clave de la fila visible que representa a `key` (la head si es una copia).
  function visibleKeyOf(key) {
    var e = collapsedBy[key];
    return e ? rowKey(e.head) : key;
  }

  function row(e) {
    var s = e.head;
    var key = rowKey(s);
    var nCopies = e.copies.length;
    var isSel = e.copies.some(function (c) { return rowKey(c) === selectedKey; });
    var st = statusOf(s);
    var r = el('button', 'row s-' + st);
    r.type = 'button';
    r.setAttribute('role', 'option');
    r.dataset.key = key;
    r.setAttribute('aria-selected', isSel ? 'true' : 'false');
    var dot = el('span', 'dot d-' + st);
    dot.setAttribute('aria-hidden', 'true');
    r.appendChild(dot);

    // Un renglón (DD-37): punto · título · proyecto · chips · hora. El estado va en aria-label.
    var v = rowView(s, { now: nowMs(), projectFilter: filters.project, maxChips: isPhone() ? 1 : 2, copies: nCopies });
    r.title = v.tooltip;
    r.setAttribute('aria-label', v.ariaLabel + (nCopies > 1 ? ', ' + nCopies + ' sesiones con el mismo título' : ''));
    r.appendChild(el('span', 'row-title', v.title));
    if (v.project) r.appendChild(el('span', 'row-proj', v.project));
    if (v.chips.length || v.extra) {
      var w = el('span', 'chips');
      v.chips.forEach(function (c) {
        var n;
        if (c.lines) {
          n = el('span', 'chip ' + c.cls);
          n.appendChild(el('span', 'add', '+' + c.lines.added));
          n.appendChild(document.createTextNode(' '));
          n.appendChild(el('span', 'del', '−' + c.lines.removed));
          n.title = c.title;
        } else {
          n = chip(c.cls, c.text, c.title);
        }
        w.appendChild(n);
      });
      if (v.extra) w.appendChild(chip('more', '+' + v.extra, v.extraTitle));
      r.appendChild(w);
    }
    r.appendChild(el('span', 'row-when num', v.when));
    r.addEventListener('click', function () { select(key, true); });
    return r;
  }

  function groupHead(g) {
    var h = el('div', 'group-h');
    var t = el('div');
    t.appendChild(el('strong', null, planHeading(g)));
    if (g.info && g.info.name) h.title = g.info.name;
    t.appendChild(document.createTextNode(' · ' + g.sessions.length + (g.waiting ? ' · ' + g.waiting + ' esperándote' : '')));
    var pt = planTotals(g.sessions);
    var tot = el('span', 'group-tokens', ' · 🔢 ' + formatTokens(pt.total) + ' tokens · output ' + formatTokens(pt.output));
    tot.title = 'Suma de las sesiones que Claude todavía lista; las borradas con `claude rm` no cuentan.';
    t.appendChild(tot);
    h.appendChild(t);
    if (g.info && typeof g.info.total === 'number' && g.info.total > 0) {
      var bar = el('div', 'bar');
      bar.setAttribute('role', 'progressbar');
      bar.setAttribute('aria-label', 'Avance de ' + (g.info.name || planHeading(g)));
      bar.setAttribute('aria-valuemin', '0');
      bar.setAttribute('aria-valuemax', String(g.info.total));
      bar.setAttribute('aria-valuenow', String(g.info.done || 0));
      bar.setAttribute('aria-valuetext', (g.info.done || 0) + ' de ' + g.info.total + ' tareas hechas');
      var fill = el('i');
      fill.style.width = Math.round(planProgress(g.info) * 100) + '%';
      bar.appendChild(fill);
      h.appendChild(bar);
    }
    return h;
  }

  function sessionsLabel(n) { return n + (n === 1 ? ' sesión' : ' sesiones'); }

  // Grupo plegable (terminadas / abandonadas, DD-38): el encabezado es una fila completa con
  // contador, visible aunque esté plegado. `id` es el ancla de los botones del encabezado de página.
  // `entries` = [{ head, copies }] (colapso DD-40); el contador cuenta todas las sesiones.
  function entriesTotal(entries) {
    var n = 0;
    entries.forEach(function (e) { n += e.copies.length; });
    return n;
  }

  function fold(id, label, title, open, onToggle, entries) {
    var total = entriesTotal(entries);
    var sec = el('div', 'fold');
    sec.id = id;
    sec.setAttribute('role', 'group');
    sec.setAttribute('aria-label', label + ', ' + sessionsLabel(total));
    var tg = btn((open ? '▾ ' : '▸ ') + label + ' (' + total + ')', 'fold-toggle');
    tg.setAttribute('aria-expanded', open ? 'true' : 'false');
    if (title) tg.title = title;
    tg.addEventListener('click', onToggle);
    sec.appendChild(tg);
    entries.forEach(noteEntry);
    if (open) entries.forEach(function (e) { navKeys.push(rowKey(e.head)); sec.appendChild(row(e)); });
    return sec;
  }

  // Grupo no plegable (Trabajando / Idle): encabezado con contador + sus filas.
  function statusSection(g) {
    var sec = el('div', 'status-group');
    sec.setAttribute('role', 'group');
    sec.setAttribute('aria-label', g.label + ', ' + sessionsLabel(g.sessions.length));
    var h = el('div', 'group-h status-h');
    h.appendChild(el('strong', null, g.label));
    h.appendChild(document.createTextNode(' (' + g.sessions.length + ')'));
    sec.appendChild(h);
    collapseRepeats(g.sessions, g.id).forEach(function (e) {
      noteEntry(e);
      navKeys.push(rowKey(e.head));
      sec.appendChild(row(e));
    });
    return sec;
  }

  function toggleDone() {
    showOldOn = !showOldOn;
    writeFlag(OLD_KEY, showOldOn);
    render();
  }

  function toggleAbandoned() {
    showAbandonedOn = !showAbandonedOn;
    writeFlag(ABN_KEY, showAbandonedOn);
    render();
  }

  function renderList(now) {
    var vis = sorted(visible(sessions));
    var groups = statusGroups(vis, now);
    var by = {};
    groups.forEach(function (g) { by[g.id] = g; });
    var nodes = [];
    navKeys = inboxKeys.slice(); // DD-39: la bandeja entra primero en j/k
    collapsedBy = {};
    var listed = 0;
    groups.forEach(function (g) { listed += g.sessions.length; });
    var totals = listTotals(vis, now);
    var abn = by.abandoned ? by.abandoned.sessions : [];
    var doneShown = totals.done;

    if (groupByPlanOn) {
      // Por plan: dentro de cada plan, orden por estado (busy, idle, terminadas recientes).
      var doneParts = splitOldDone(by.done ? by.done.sessions : [], now);
      var live = [].concat(by.busy ? by.busy.sessions : [], by.idle ? by.idle.sessions : [], doneParts.recent);
      groupByPlan(live).forEach(function (g) {
        nodes.push(groupHead(g));
        collapseMixed(g.sessions, now).forEach(function (e) {
          noteEntry(e);
          navKeys.push(rowKey(e.head));
          nodes.push(row(e));
        });
      });
      doneShown = doneParts.old.length;
      if (doneParts.old.length) {
        nodes.push(fold('grp-done', 'Terminadas hace más de 1 día', '', showOldOn, toggleDone,
          collapseRepeats(doneParts.old, 'done')));
      }
    } else {
      groups.forEach(function (g) {
        if (g.id === 'busy' || g.id === 'idle') nodes.push(statusSection(g));
        else if (g.id === 'done') {
          nodes.push(fold('grp-done', g.label, '', showOldOn, toggleDone, collapseRepeats(g.sessions, 'done')));
        }
      });
    }
    if (abn.length) {
      nodes.push(fold('grp-abandoned', 'Abandonadas',
        'Esperan hace más de 24 h. No cuentan en el contador ni en los avisos; no se borra nada.',
        showAbandonedOn, toggleAbandoned, collapseRepeats(abn, 'abandoned')));
    }

    // Botones del encabezado de página (no son filtros).
    var bd = $('btn-done');
    bd.hidden = doneShown === 0;
    $('n-done').textContent = doneShown;
    var ba = $('btn-abandoned');
    ba.hidden = totals.abandoned === 0;
    $('n-abandoned').textContent = totals.abandoned;

    var rows = $('rows');
    rows.replaceChildren.apply(rows, nodes);

    // Lista vacía pero con sesiones esperando: están en la bandeja, no es "no hay sesiones" (DD-39).
    var empty = $('empty');
    var waitingVis = inboxKeys.length ? vis.filter(function (s) { return isWaiting(s.status) && !isAbandoned(s, now); }).length : 0;
    if (listed > 0) {
      empty.hidden = true;
    } else if (waitingVis > 0) {
      empty.textContent = waitingVis === 1
        ? 'La sesión que te espera está arriba, en Te esperan.'
        : 'Las ' + waitingVis + ' sesiones que te esperan están arriba, en Te esperan.';
      empty.hidden = false;
    } else {
      empty.textContent = 'No hay sesiones para mostrar.';
      empty.hidden = vis.length > 0;
    }
  }

  function renderInbox(now) {
    var items = inboxItems(sessions, now);
    inboxKeys = items.map(rowKey);
    $('inbox-title').textContent = 'Te esperan' + (items.length ? ' (' + items.length + ')' : '');
    var box = $('inbox-items');
    box.parentNode.classList.toggle('empty', !items.length); // vacía: una sola línea, deja lugar a la lista
    if (!items.length) {
      box.replaceChildren(el('p', 'inbox-empty', 'Nadie te espera.'));
      return;
    }
    box.replaceChildren.apply(box, items.map(function (s) {
      var key = rowKey(s);
      var c = el('article', 'ask-card');
      // Navegable como una fila (DD-39): data-key, foco, selección y click fuera de sus botones.
      c.setAttribute('role', 'option');
      c.tabIndex = 0;
      c.dataset.key = key;
      c.setAttribute('aria-selected', key === selectedKey ? 'true' : 'false');
      c.addEventListener('click', function (ev) {
        if (ev.target.closest && ev.target.closest('button')) return;
        select(key, true);
      });
      // Renglón 1: título · proyecto · hora + acciones a la derecha. Renglón 2: motivo en una línea.
      var h = el('header');
      h.appendChild(el('strong', null, titleOf(s)));
      h.appendChild(el('span', 'meta', [s.project, s.branch ? '⎇ ' + s.branch : null].filter(Boolean).join(' · ')));
      h.appendChild(el('span', 'help', ago(s.lastActivityAt || s.startedAt)));
      var wv = waitView(s);
      var q = el('div', 'ask-q');
      q.appendChild(el('span', 'lbl', wv.label));
      if (wv.text) q.appendChild(document.createTextNode(wv.text));
      else if (wv.context) q.appendChild(document.createTextNode('· ' + wv.context));
      if (wv.more) q.appendChild(el('span', 'lbl', ' ' + wv.more));
      var full = waitFull(wv);
      q.title = full;
      var a = el('div', 'ask-actions');
      var open = btn('ver detalle');
      open.addEventListener('click', function () { select(key, true); });
      a.appendChild(open);
      if (validRemoteUrl(s.remoteUrl)) {
        var ph = btn('📱');
        ph.title = 'Abrir en el teléfono (QR)';
        ph.setAttribute('aria-label', 'Abrir en el teléfono');
        ph.addEventListener('click', function () { qrOpenFor = key; select(key, true); });
        a.appendChild(ph);
      }
      h.appendChild(a);
      c.appendChild(h);
      c.appendChild(q);
      return c;
    }));
  }

  function renderToday() {
    var t = $('today');
    var v = historyToday;
    t.hidden = !v;
    if (!v) return;
    t.replaceChildren(document.createTextNode('hoy '), el('b', 'num', v.output),
      document.createTextNode(' output · '), el('b', 'num', String(v.sessions)),
      document.createTextNode(v.sessions === 1 ? ' sesión' : ' sesiones'));
  }

  function loadToday() {
    fetch(FIXTURE ? 'fixture-history.json' : '/api/history')
      .then(function (r) { if (!r.ok) throw new Error('http ' + r.status); return r.json(); })
      .then(function (d) { historyToday = todayView(d); renderToday(); })
      .catch(function () { /* sin historial: el resumen queda oculto */ });
  }

  // ---- Detalle ----

  function selectedSession() {
    return resolveSession(sessions, selectedKey);
  }

  // La línea de tiempo se pide al seleccionar y cuando cambia lastActivityAt (no en cada snapshot).
  function ensureTimeline(s) {
    var key = rowKey(s);
    var at = s.lastActivityAt || 0;
    var t = timeline;
    if (t.key === key && t.at === at) return;
    var fresh = t.key !== key;
    t.key = key;
    t.at = at;
    if (fresh) { t.status = 'loading'; t.events = []; }
    t.version++;
    var req = ++t.req;
    var url = FIXTURE ? 'fixture-timeline.json' : '/api/sessions/' + encodeURIComponent(s.sessionId) + '/timeline';
    fetch(url)
      .then(function (r) { if (!r.ok) throw new Error('http ' + r.status); return r.json(); })
      .then(function (d) {
        if (req !== t.req) return;
        t.events = d && Array.isArray(d.events) ? d.events : [];
        t.status = 'ok';
        t.version++;
        render();
      })
      .catch(function () {
        if (req !== t.req) return;
        if (fresh) t.status = 'err';
        t.version++;
        render();
      });
  }

  function timelineNode() {
    var t = timeline;
    if (t.status === 'loading') return el('p', 'empty', 'Cargando…');
    if (t.status === 'err') return el('p', 'empty', 'No se pudo cargar la línea de tiempo.');
    if (!t.events.length) return el('p', 'empty', 'Sin eventos.');
    var ol = el('ol', 'events');
    t.events.forEach(function (ev) {
      var kind = ev && (ev.kind === 'prompt' || ev.kind === 'tool') ? ev.kind : 'text';
      var li = el('li', 'ev ev-' + kind);
      li.appendChild(el('time', 'ev-at', ev && ev.at ? new Date(ev.at).toLocaleTimeString() : '—'));
      li.appendChild(el('span', 'ev-kind', kind === 'prompt' ? 'prompt' : kind === 'tool' ? 'herramienta' : 'texto'));
      var evText = el('div', 'ev-text', ev && ev.text != null ? String(ev.text) : '');
      evText.title = evText.textContent; // los comandos se recortan a 2 líneas; el completo en el tooltip
      li.appendChild(evText);
      ol.appendChild(li);
    });
    return ol;
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

  function qrBox(s) {
    var box = el('div', 'qr-box');
    var rows;
    try { rows = qrMatrix(s.remoteUrl); } catch (e) { rows = []; }
    if (rows.length) {
      var canvas = el('canvas', 'qr');
      canvas.setAttribute('role', 'img');
      canvas.setAttribute('aria-label', 'Código QR con el link de la sesión');
      drawQr(canvas, rows);
      box.appendChild(canvas);
    }
    var side = el('div', 'qr-side');
    side.appendChild(el('p', 'help', rows.length ? 'Escaneá el QR con la cámara del teléfono, o abrí el link.' : 'No se pudo generar el QR; usá el link.'));
    var a = el('a', 'phone-link', s.remoteUrl);
    a.href = s.remoteUrl;
    a.target = '_blank';
    a.rel = 'noopener';
    side.appendChild(a);
    var cp = btn('copiar link');
    cp.addEventListener('click', function () { copyText(s.remoteUrl, cp); });
    side.appendChild(cp);
    box.appendChild(side);
    return box;
  }

  function stat(label, value, warn) {
    var b = el('div', 'stat' + (warn ? ' warn' : ''));
    b.appendChild(el('span', 'stat-k', label));
    b.appendChild(el('span', 'stat-v num', value));
    return b;
  }

  // Las URLs ya pasaron por validPrUrl/validArtifactUrl: nunca llega un javascript:.
  function extLink(text, url) {
    var a = el('a', null, text);
    a.href = url;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    a.title = url;
    return a;
  }

  function buildDetail(d, s) {
    var st = statusOf(s);
    var key = rowKey(s);
    var nodes = [];

    var back = btn('← volver a la lista', 'back');
    back.addEventListener('click', function () { phoneDetail = false; render(); });
    nodes.push(back);

    var head = el('div', 'detail-head');
    head.appendChild(el('h3', null, titleOf(s)));
    head.appendChild(el('span', 'pill s-' + st, STATUS_ICON[st] + ' ' + STATUS_LABEL[st]));
    nodes.push(head);

    var meta = el('div', 'meta');
    meta.appendChild(el('span', null, s.project || '—'));
    if (s.name && s.name !== s.title) meta.appendChild(el('span', null, s.name));
    meta.appendChild(el('span', null, ago(s.lastActivityAt || s.startedAt)));
    var perm = permissionChip(s);
    if (perm) meta.appendChild(el('span', null, 'permisos: ' + perm));
    if (s.remote) {
      var rm = el('span', null, '📡 remoto');
      rm.title = 'Sesión con bridge remoto';
      meta.appendChild(rm);
    }
    nodes.push(meta);

    if (typeof s.branch === 'string' && s.branch) {
      var br = el('div', 'branch', '⎇ ' + s.branch);
      br.title = s.branch;
      nodes.push(br);
    }

    var aw = awayLine(s);
    if (aw) {
      var awBox = el('div', 'away');
      awBox.appendChild(el('span', 'lbl', 'Mientras no estabas'));
      awBox.appendChild(document.createTextNode(aw));
      nodes.push(awBox);
    }

    var chips = chipsFor(s, true);
    if (chips) nodes.push(chips);

    var av = askView(s);
    if (av) {
      var ask = el('div', 'ask ask-' + av.mode);
      ask.appendChild(el('span', 'ask-label', av.label));
      ask.appendChild(el('span', 'ask-text', plainText(av.text)));
      if (av.more) ask.appendChild(el('span', 'ask-more', av.more));
      nodes.push(ask);
    }

    if (s.orq) {
      var ol = orqLabel(s.orq);
      if (ol) {
        var ob = el('div', 'orq', ol);
        ob.title = 'Worker de orq' + (s.orq.ws ? ' · ws ' + s.orq.ws : '') + (s.orq.job ? ' · job ' + s.orq.job : '');
        nodes.push(ob);
      }
    }

    if (!av && (s.waitingFor || isWaiting(s.status))) {
      var wv = waitView(s);
      nodes.push(el('div', 'waiting-for', waitFull(wv)));
    }

    if (s.lastPrompt) {
      var p = el('p', 'prompt' + (expanded.has(s.id) ? ' open' : ''), plainText(s.lastPrompt));
      p.title = 'Click para expandir/colapsar';
      p.addEventListener('click', function () {
        if (expanded.has(s.id)) expanded.delete(s.id); else expanded.add(s.id);
        p.classList.toggle('open');
      });
      nodes.push(p);
    }

    if (s.lastAction && s.lastAction.text) {
      var prefix = s.lastAction.kind === 'tool' ? '▸ ' : '“ ';
      var ac = el('div', 'action', prefix + (s.lastAction.kind === 'text' ? plainText(s.lastAction.text) : oneLine(s.lastAction.text)));
      ac.title = String(s.lastAction.text);
      nodes.push(ac);
    }

    if (s.tokens) {
      var tk = s.tokens;
      var tt = el('div', 'tokens', '🔢 ' + formatTokens(totalTokens(tk)) + ' tokens' + (tk.model ? ' · ' + tk.model : ''));
      tt.title = 'input ' + (tk.input || 0) + ' · output ' + (tk.output || 0) +
        ' · cache leído ' + (tk.cacheRead || 0) + ' · cache creado ' + (tk.cacheCreation || 0);
      nodes.push(tt);
    }

    var ctxText = formatContext(s.context);
    if (ctxText) {
      var cx = el('div', 'context' + (s.context.warn ? ' warn' : ''), ctxText);
      cx.title = contextTitle(s.context);
      nodes.push(cx);
    }

    var stats = el('div', 'stats');
    var tmin = turnMinutes(s, nowMs());
    if (tmin !== null) stats.appendChild(stat('turno actual', tmin + ' min', tmin >= TURN_WARN_MIN));
    if (hasLines(s)) stats.appendChild(stat('líneas', '+' + s.lines.added + ' −' + s.lines.removed, false));
    if (stats.childNodes.length) nodes.push(stats);

    var prs = validPrs(s);
    var arts = validArtifacts(s);
    if (prs.length || arts.length) {
      var links = el('div', 'links');
      links.appendChild(el('span', 'section-lbl', 'Links'));
      prs.forEach(function (p) { links.appendChild(extLink('PR #' + p.number + (p.repo ? ' · ' + p.repo : ''), p.url)); });
      arts.forEach(function (a) { links.appendChild(extLink('Artifact: ' + (a.title || a.url), a.url)); });
      nodes.push(links);
    }

    // DD-40: copias de la misma conversación colapsadas en una fila.
    var entry = collapsedBy[key];
    if (entry && entry.copies.length > 1) {
      var cps = el('div', 'copies');
      cps.appendChild(el('span', 'section-lbl', 'Copias (' + entry.copies.length + ')'));
      entry.copies.forEach(function (c) {
        var ck = rowKey(c);
        var cur = ck === key;
        var b = btn('', 'copy-item' + (cur ? ' current' : ''));
        b.dataset.copy = ck;
        b.setAttribute('aria-pressed', cur ? 'true' : 'false');
        var cst = statusOf(c);
        b.appendChild(el('span', 'copy-st s-' + cst, STATUS_LABEL[cst]));
        b.appendChild(el('span', 'copy-when', ago(c.lastActivityAt || c.startedAt)));
        b.appendChild(el('span', 'copy-pid num', c.pid > 0 ? 'PID ' + c.pid : 'sin PID'));
        if (cur) b.appendChild(el('span', 'copy-now', 'actual'));
        b.addEventListener('click', function () { select(ck, false); });
        cps.appendChild(b);
      });
      nodes.push(cps);
    }

    var tl = el('div', 'tl-wrap');
    tl.appendChild(el('span', 'section-lbl', 'Línea de tiempo'));
    tl.appendChild(timelineNode());
    nodes.push(tl);

    var mode = phoneMode(s);
    var actions = el('div', 'actions');
    if (mode === 'qr') {
      var ph = btn('📱 abrir en el teléfono');
      ph.dataset.act = 'qr';
      ph.setAttribute('aria-expanded', qrOpenFor === key ? 'true' : 'false');
      ph.addEventListener('click', function () { qrOpenFor = qrOpenFor === key ? '' : key; render(); });
      actions.appendChild(ph);
    }
    var cp = btn('copiar comando');
    cp.dataset.act = 'copy';
    cp.addEventListener('click', function () { copyText(copyCommand(s), cp); });
    actions.appendChild(cp);
    nodes.push(actions);

    if (mode === 'qr' && qrOpenFor === key) nodes.push(qrBox(s));
    if (mode === 'enable') {
      nodes.push(el('div', 'help', '📱 Para abrirla en el teléfono, activalo escribiendo /remote-control (o /rc) en esa sesión; el QR aparece solo en ~3 s.'));
    }
    var warn = resumeWarning(s);
    if (warn) nodes.push(el('div', 'help', warn));

    d.replaceChildren.apply(d, nodes);
  }

  function renderDetail() {
    var d = $('detail');
    var s = selectedSession();
    if (s && (!isPhone() || phoneDetail)) ensureTimeline(s);
    var minute = s ? Math.floor((nowMs() - (s.lastActivityAt || s.startedAt || 0)) / 60000) : 0;
    var ent = s ? collapsedBy[rowKey(s)] : null;
    var sig = JSON.stringify([s, minute, s ? turnMinutes(s, nowMs()) : null, qrOpenFor === (s && rowKey(s)), s ? expanded.has(s.id) : false,
      timeline.key, timeline.version, timeline.status, ent ? ent.copies : null]);
    if (sig === detailSig) return;
    detailSig = sig;
    var scroll = d.scrollTop;
    var act = d.contains(document.activeElement) && document.activeElement.dataset ? document.activeElement.dataset.act : '';
    if (!s) {
      d.replaceChildren(el('p', 'empty', 'Elegí una sesión de la lista.'));
      return;
    }
    buildDetail(d, s);
    d.scrollTop = scroll;
    if (act) {
      var again = d.querySelector('[data-act="' + act + '"]');
      if (again) again.focus({ preventScroll: true });
    }
  }

  // Filas de la lista y tarjetas de la bandeja.
  function rowByKey(k) {
    var rows = document.querySelectorAll('#rows .row, #inbox-items .ask-card');
    for (var i = 0; i < rows.length; i++) if (rows[i].dataset.key === k) return rows[i];
    return null;
  }

  // Un solo render para todo; conserva foco de la fila, scroll de la página, selección y detalle abierto.
  function render(opts) {
    opts = opts || {};
    var now = nowMs();
    var c = counts(sessions, now);
    $('n-waiting').textContent = c.waiting;
    $('n-busy').textContent = c.busy;
    $('n-idle').textContent = c.idle;
    STATUS_FILTERS.forEach(function (f) {
      $('c-' + f).setAttribute('aria-pressed', filters.status === f ? 'true' : 'false');
    });
    updateFavicon(c.waiting);
    document.title = c.waiting > 0 ? '(' + c.waiting + ') esperándote — ' + BASE_TITLE : BASE_TITLE;

    // Selección vieja sin '#pid' (localStorage): se migra a la clave exacta de la sesión resuelta.
    var cur = selectedSession();
    if (cur && rowKey(cur) !== selectedKey) { selectedKey = rowKey(cur); saveSelected(selectedKey); }

    var ae = document.activeElement;
    var focusKey = opts.focusKey || (ae && ae.classList && (ae.classList.contains('row') || ae.classList.contains('ask-card')) ? ae.dataset.key : '');
    var y = window.scrollY;

    renderInbox(now);
    renderList(now);
    if (sessions.length && !selectedSession()) {
      // selección inexistente (o nunca elegida): la primera fila, sin guardarla
      selectedKey = navKeys[0] || '';
      var rs = document.querySelectorAll('#rows .row, #inbox-items .ask-card');
      for (var i = 0; i < rs.length; i++) rs[i].setAttribute('aria-selected', rs[i].dataset.key === selectedKey ? 'true' : 'false');
    }
    renderDetail();
    document.body.classList.toggle('phone-detail', phoneDetail && isPhone());

    if (focusKey) {
      var r = rowByKey(focusKey);
      if (r) {
        r.focus({ preventScroll: true });
        if (opts.focusKey) r.scrollIntoView({ block: 'nearest' });
      }
    }
    if (!opts.focusKey && Math.abs(window.scrollY - y) > 1) window.scrollTo(0, y);
  }

  function select(key, open) {
    selectedKey = key;
    saveSelected(key);
    if (open && isPhone()) phoneDetail = true;
    render({ focusKey: visibleKeyOf(key) });
    if (open && isPhone()) window.scrollTo(0, 0);
  }

  function onKey(ev, search) {
    if (ev.ctrlKey || ev.metaKey || ev.altKey) return;
    var ae = document.activeElement;
    var tag = ae && ae.tagName;
    var typing = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
    if (ev.key === 'Escape') {
      if (!$('history').hidden) { closeHistory(); return; }
      if (search.value || ae === search) {
        if (search.value) { search.value = ''; query = ''; render(); }
        search.blur();
        return;
      }
      if (typing) return;
      if (qrOpenFor) { qrOpenFor = ''; render(); return; }
      if (phoneDetail) { phoneDetail = false; render(); window.scrollTo(0, 0); }
      return;
    }
    if (typing || !$('history').hidden) return;
    var k = ev.key;
    if (k === 'j' || k === 'ArrowDown' || k === 'k' || k === 'ArrowUp') {
      var next = stepSelection(navKeys, visibleKeyOf(selectedKey), k === 'j' || k === 'ArrowDown' ? 1 : -1);
      if (next === null) return;
      ev.preventDefault();
      if (next !== selectedKey) { selectedKey = next; saveSelected(next); }
      render({ focusKey: next });
    } else if (k === 'Enter') {
      if (tag === 'BUTTON' || tag === 'A' || !selectedKey) return;
      if (isPhone()) { phoneDetail = true; render(); window.scrollTo(0, 0); }
    } else if (k === '/') {
      ev.preventDefault();
      search.focus();
    } else if (k === '1' || k === '2' || k === '3') {
      filters.status = toggleStatusFilter(filters.status, STATUS_FILTERS[+k - 1]);
      saveFilters();
      render();
    }
  }

  function init() {
    chkDone.checked = filters.hideDone;
    chkIdle.checked = filters.hideIdle;
    selProject.addEventListener('change', function () { filters.project = selProject.value; saveFilters(); render(); });
    chkDone.addEventListener('change', function () { filters.hideDone = chkDone.checked; saveFilters(); render(); });
    chkIdle.addEventListener('change', function () { filters.hideIdle = chkIdle.checked; saveFilters(); render(); });

    STATUS_FILTERS.forEach(function (f) {
      $('c-' + f).addEventListener('click', function () {
        filters.status = toggleStatusFilter(filters.status, f);
        saveFilters();
        render();
      });
    });

    // DD-38: botones del encabezado; despliegan su grupo y lo traen a la vista (no son filtros).
    function revealGroup(id, isOpen, toggle) {
      if (!isOpen()) toggle();
      var g = document.getElementById(id);
      if (g && g.scrollIntoView) g.scrollIntoView({ block: 'nearest' });
    }
    $('btn-done').addEventListener('click', function () {
      revealGroup('grp-done', function () { return showOldOn; }, toggleDone);
    });
    $('btn-abandoned').addEventListener('click', function () {
      revealGroup('grp-abandoned', function () { return showAbandonedOn; }, toggleAbandoned);
    });

    var chkGroup = $('f-group');
    chkGroup.checked = groupByPlanOn;
    chkGroup.addEventListener('change', function () { groupByPlanOn = chkGroup.checked; writeFlag(GROUP_KEY, groupByPlanOn); render(); });
    $('btn-notify').addEventListener('click', toggleNotify);
    updateNotifyBtn();
    $('history-close').addEventListener('click', closeHistory);
    $('history').addEventListener('click', function (ev) { if (ev.target === $('history')) closeHistory(); });
    $('btn-history').addEventListener('click', function (ev) { openHistory(ev.currentTarget); });
    var search = $('f-search');
    search.addEventListener('input', function () { query = search.value; render(); });
    document.addEventListener('keydown', function (ev) { onKey(ev, search); });
    window.addEventListener('resize', function () { render(); });

    // refresca los "hace X min" sin esperar un snapshot
    setInterval(render, 30000);
    loadToday();
    setInterval(loadToday, 60000);

    if (FIXTURE) {
      fetch(FIXTURE_FILE)
        .then(function (r) { return r.json(); })
        .then(function (d) { setSnapshot(d); setConn(true); })
        .catch(function () { setConn(false); $('empty').textContent = 'No se pudo cargar ' + FIXTURE_FILE + ' (¿servido por HTTP?)'; $('empty').hidden = false; });
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
