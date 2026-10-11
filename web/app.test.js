'use strict';
const test = require('node:test');
const assert = require('node:assert');
const { notifyTargets, formatTokens, totalTokens, groupByPlan, orqLabel } = require('./app.js');

const s = (id, status, extra) => Object.assign({ id, sessionId: id, status }, extra);

test('primer snapshot (prev null) no notifica', () => {
  assert.deepStrictEqual(notifyTargets(null, [s('a', 'waiting')]), []);
  assert.deepStrictEqual(notifyTargets(undefined, [s('a', 'blocked')]), []);
});

test('notifica solo en la transición a waiting/blocked', () => {
  const prev = [s('a', 'busy'), s('b', 'waiting'), s('c', 'idle'), s('d', 'busy')];
  const next = [s('a', 'waiting'), s('b', 'waiting'), s('c', 'blocked'), s('d', 'idle')];
  assert.deepStrictEqual(notifyTargets(prev, next).map((x) => x.id), ['a', 'c']);
});

test('waiting -> blocked no vuelve a notificar; snapshot igual tampoco', () => {
  const prev = [s('a', 'waiting')];
  assert.deepStrictEqual(notifyTargets(prev, [s('a', 'blocked')]), []);
  assert.deepStrictEqual(notifyTargets(prev, prev), []);
});

test('sesión nueva que ya aparece waiting sí notifica; las que desaparecen no rompen', () => {
  assert.deepStrictEqual(notifyTargets([s('x', 'busy')], [s('n', 'waiting')]).map((x) => x.id), ['n']);
});

test('formatTokens', () => {
  assert.strictEqual(formatTokens(0), '0');
  assert.strictEqual(formatTokens(999), '999');
  assert.strictEqual(formatTokens(1000), '1k');
  assert.strictEqual(formatTokens(1234), '1.2k');
  assert.strictEqual(formatTokens(1200000), '1.2M');
  assert.strictEqual(formatTokens(999999), '1M');
  assert.strictEqual(formatTokens(2500000000), '2.5G');
  assert.strictEqual(formatTokens(null), '0');
  assert.strictEqual(formatTokens(-5), '0');
});

test('totalTokens suma los cuatro campos y tolera null', () => {
  assert.strictEqual(totalTokens({ input: 1, output: 2, cacheRead: 3, cacheCreation: 4 }), 10);
  assert.strictEqual(totalTokens(null), 0);
});

test('groupByPlan ordena por plan numérico y deja "sin plan" al final', () => {
  const list = [
    s('a', 'busy'),
    s('b', 'waiting', { orq: { plan: '10' } }),
    s('c', 'busy', { orq: { plan: '2' } }),
    s('d', 'blocked', { orq: { plan: '2' } }),
    s('e', 'idle', { orq: { plan: null, task: null } }),
  ];
  const g = groupByPlan(list);
  assert.deepStrictEqual(g.map((x) => x.plan), ['2', '10', null]);
  assert.deepStrictEqual(g.map((x) => x.sessions.length), [2, 1, 2]);
  assert.deepStrictEqual(g.map((x) => x.waiting), [1, 1, 0]);
});

test('orqLabel', () => {
  assert.strictEqual(orqLabel({ plan: '1', task: '005', specialist: 'w' }), 'PLAN-1 · TASK-005 · w');
  assert.strictEqual(orqLabel({ plan: null, task: null, specialist: 'w' }), 'w');
  assert.strictEqual(orqLabel(null), '');
});

const A = require('./app.js');

test('formatContext: normal, con compactaciones, warn y null', () => {
  assert.strictEqual(A.formatContext({ current: 412000, compactions: 2, warn: false }), 'contexto 412k · compactó 2×');
  assert.strictEqual(A.formatContext({ current: 90000, compactions: 0, warn: false }), 'contexto 90k');
  assert.strictEqual(A.formatContext({ current: 850000, compactions: 1, warn: true }), '⚠ contexto 850k, cerca de compactar');
  assert.strictEqual(A.formatContext(null), '');
  assert.strictEqual(A.formatContext(undefined), '');
});

test('contextTitle y rateTitle', () => {
  assert.match(A.contextTitle({ compactions: 2, lastPreTokens: 967001 }), /compactó en 967k/);
  assert.doesNotMatch(A.contextTitle({ compactions: 0, lastPreTokens: null }), /compactó en/);
  assert.strictEqual(A.rateTitle({ windowMin: 5, output: 48000, calls: 31 }), 'En 5 min: 48k tokens de output y 31 llamadas');
  assert.strictEqual(A.rateTitle(null), '');
});

test('planTotals suma tokens y tolera tokens null', () => {
  const list = [
    { tokens: { input: 1, output: 2, cacheRead: 3, cacheCreation: 4 } },
    { tokens: null },
    { tokens: { input: 10, output: 20, cacheRead: 0, cacheCreation: 0 } },
  ];
  assert.deepStrictEqual(A.planTotals(list), { total: 40, output: 22 });
  assert.deepStrictEqual(A.planTotals([]), { total: 0, output: 0 });
});

test('isOldDone: corte de 24 h, usa startedAt sin lastActivityAt, solo done', () => {
  const now = 1000 * 86400000;
  const H = 3600000;
  assert.strictEqual(A.isOldDone({ status: 'done', lastActivityAt: now - 25 * H }, now), true);
  assert.strictEqual(A.isOldDone({ status: 'done', lastActivityAt: now - 23 * H }, now), false);
  assert.strictEqual(A.isOldDone({ status: 'done', lastActivityAt: now - 24 * H }, now), false);
  assert.strictEqual(A.isOldDone({ status: 'done', lastActivityAt: null, startedAt: now - 48 * H }, now), true);
  assert.strictEqual(A.isOldDone({ status: 'done' }, now), false);
  assert.strictEqual(A.isOldDone({ status: 'idle', lastActivityAt: now - 99 * H }, now), false);
});

test('splitOldDone separa sin perder ni reordenar', () => {
  const now = 1000 * 86400000;
  const list = [
    { id: 'a', status: 'busy', lastActivityAt: 1 },
    { id: 'b', status: 'done', lastActivityAt: now - 3600000 },
    { id: 'c', status: 'done', lastActivityAt: now - 3 * 86400000 },
  ];
  const r = A.splitOldDone(list, now);
  assert.deepStrictEqual(r.recent.map((x) => x.id), ['a', 'b']);
  assert.deepStrictEqual(r.old.map((x) => x.id), ['c']);
});

test('normalizeText quita tildes y mayúsculas', () => {
  assert.strictEqual(A.normalizeText('Migración ÁÉÍÓÚ ñ'), 'migracion aeiou n');
  assert.strictEqual(A.normalizeText(null), '');
});

test('matchesQuery: campos, tildes, mayúsculas, orq y vacío', () => {
  const sess = { name: 'bg-lint', title: 'Migración de pagos', lastPrompt: 'Corré el LINTER', project: 'demo-web', orq: { plan: '3', task: '009' } };
  assert.strictEqual(A.matchesQuery(sess, ''), true);
  assert.strictEqual(A.matchesQuery(sess, '   '), true);
  assert.strictEqual(A.matchesQuery(sess, 'MIGRACION'), true);
  assert.strictEqual(A.matchesQuery(sess, 'corre el linter'), true);
  assert.strictEqual(A.matchesQuery(sess, 'demo-web'), true);
  assert.strictEqual(A.matchesQuery(sess, 'plan-3'), true);
  assert.strictEqual(A.matchesQuery(sess, '009'), true);
  assert.strictEqual(A.matchesQuery(sess, 'inexistente'), false);
  assert.strictEqual(A.matchesQuery({ name: 'x' }, 'y'), false);
});

const fs = require('node:fs');
const qrcode = require('./vendor/qrcode.js');
const BASE = 'https://claude.ai/' + 'code/';
const ID = 'session_01FakeFixtureIdAbCdEfGhI';
const RURL = BASE + ID;

test('phoneMode: qué ve cada tipo de sesión (DD-20)', () => {
  assert.strictEqual(A.phoneMode({ kind: 'interactive', remoteUrl: RURL }), 'qr');
  assert.strictEqual(A.phoneMode({ kind: 'background', remoteUrl: RURL }), 'qr');
  assert.strictEqual(A.phoneMode({ kind: 'interactive', remoteUrl: null }), 'enable');
  assert.strictEqual(A.phoneMode({ kind: 'interactive' }), 'enable');
  assert.strictEqual(A.phoneMode({ kind: 'background', remoteUrl: null }), 'none');
  assert.strictEqual(A.phoneMode(null), 'none');
});

test('validRemoteUrl rechaza todo lo que no sea el link esperado', () => {
  assert.strictEqual(A.validRemoteUrl(RURL), true);
  assert.strictEqual(A.validRemoteUrl(RURL + '?x=1'), false);
  assert.strictEqual(A.validRemoteUrl(RURL + '/'), false);
  assert.strictEqual(A.validRemoteUrl('javascript:alert(1)'), false);
  assert.strictEqual(A.validRemoteUrl(BASE.replace('https', 'http') + ID), false);
  assert.strictEqual(A.validRemoteUrl(BASE.replace('claude.ai', 'evil.example') + ID), false);
  assert.strictEqual(A.validRemoteUrl(RURL + '\n'), false);
  assert.strictEqual(A.validRemoteUrl(null), false);
});

test('resumeWarning: solo interactivas no terminadas; sugiere el QR si hay link', () => {
  assert.match(A.resumeWarning({ kind: 'interactive', status: 'busy' }), /segunda copia/);
  assert.doesNotMatch(A.resumeWarning({ kind: 'interactive', status: 'busy' }), /QR/);
  assert.match(A.resumeWarning({ kind: 'interactive', status: 'idle', remoteUrl: RURL }), /QR/);
  assert.strictEqual(A.resumeWarning({ kind: 'interactive', status: 'done' }), '');
  assert.strictEqual(A.resumeWarning({ kind: 'background', status: 'busy' }), '');
});

test('qrMatrix tiene el tamaño de getModuleCount() y es cuadrada', () => {
  const qr = qrcode(0, 'M');
  qr.addData(RURL);
  qr.make();
  const n = qr.getModuleCount();
  const m = A.qrMatrix(RURL, qrcode);
  assert.strictEqual(m.length, n);
  assert.ok(m.every((row) => row.length === n));
  assert.ok(m.some((row) => row.some(Boolean)));
  // esquina superior izquierda: patrón de posición (oscuro)
  assert.strictEqual(m[0][0], true);
});

test('fixture: una con link, una interactiva sin él, una background sin él', () => {
  const d = JSON.parse(fs.readFileSync(__dirname + '/fixture.json', 'utf8'));
  const modes = d.sessions.map((x) => A.phoneMode(x));
  assert.ok(modes.includes('qr'));
  assert.ok(modes.includes('enable'));
  assert.ok(modes.includes('none'));
  d.sessions.forEach((x) => assert.ok('remoteUrl' in x));
});

test('app.js no usa los generadores de HTML de la librería ni innerHTML', () => {
  const src = fs.readFileSync(__dirname + '/app.js', 'utf8');
  assert.doesNotMatch(src, /createSvgTag|createImgTag|createTableTag|innerHTML/);
});

// ---- PLAN-005 ----

test('askView: según estado y kind (DD-25)', () => {
  const q = { kind: 'question', text: '¿Mergeo?', more: 2 };
  const w = A.askView({ status: 'waiting', ask: q });
  assert.deepStrictEqual(w, { mode: 'big', label: 'pregunta:', text: '¿Mergeo?', more: '+2 preguntas' });
  assert.strictEqual(A.askView({ status: 'blocked', ask: { kind: 'question', text: 'x', more: 1 } }).more, '+1 pregunta');
  assert.strictEqual(A.askView({ status: 'waiting', ask: { kind: 'question', text: 'x', more: 0 } }).more, '');
  const t = A.askView({ status: 'waiting', ask: { kind: 'tool', text: 'Bash: git push' } });
  assert.strictEqual(t.label, 'pide permiso:');
  assert.strictEqual(t.text, 'Bash: git push');
  assert.strictEqual(A.askView({ status: 'waiting', ask: { kind: 'text', text: 'hola' } }).label, 'último mensaje:');
  const i = A.askView({ status: 'idle', ask: { kind: 'text', text: 'hola' } });
  assert.strictEqual(i.mode, 'small');
  assert.strictEqual(i.label, 'último mensaje:');
  assert.strictEqual(A.askView({ status: 'busy', ask: q }), null);
  assert.strictEqual(A.askView({ status: 'done', ask: q }), null);
  assert.strictEqual(A.askView({ status: 'waiting', ask: null }), null);
  assert.strictEqual(A.askView({ status: 'waiting' }), null);
  assert.strictEqual(A.askView({ status: 'waiting', ask: { kind: 'text', text: '' } }), null);
});

test('errorsView: insignia solo con high; sin high solo tooltip', () => {
  assert.strictEqual(A.errorsView(null), null);
  assert.strictEqual(A.errorsView({ count: 0, high: false }), null);
  const hi = A.errorsView({ windowMin: 10, count: 6, high: true });
  assert.strictEqual(hi.badge, '⚠ 6 errores en 10 min');
  assert.match(hi.title, /6 errores/);
  const lo = A.errorsView({ windowMin: 10, count: 2, high: false });
  assert.strictEqual(lo.badge, '');
  assert.match(lo.title, /2 errores/);
});

test('groupByPlan: clave plan.path con fallback a orq.plan', () => {
  const info = { number: 42, name: 'PLAN-042-x', path: '/p/PLAN-042-x', total: 28, done: 14, inProgress: 2, blocked: 4 };
  const list = [
    s('a', 'busy', { plan: info, orq: { plan: '42' } }),
    s('b', 'waiting', { plan: info }),
    s('c', 'busy', { orq: { plan: '2' } }),
    s('d', 'idle', { plan: { number: 7, path: '/p/PLAN-007-y', total: 1, done: 1 } }),
    s('e', 'idle'),
  ];
  const g = groupByPlan(list);
  assert.deepStrictEqual(g.map((x) => x.plan), ['2', '7', '42', null]);
  assert.deepStrictEqual(g.map((x) => x.sessions.length), [1, 1, 2, 1]);
  assert.strictEqual(g[2].info, info);
  assert.strictEqual(g[0].info, null);
  assert.strictEqual(g[2].waiting, 1);
  // mismo plan.path con distinto orq.plan: un solo grupo
  assert.strictEqual(groupByPlan([s('x', 'busy', { plan: info, orq: { plan: '1' } }), s('y', 'busy', { plan: info })]).length, 1);
});

test('planHeading y planProgress', () => {
  const g = { plan: '42', info: { total: 28, done: 14, blocked: 4, inProgress: 2 } };
  assert.strictEqual(A.planHeading(g), 'PLAN-42 · 14/28 done · 4 blocked · 2 in_progress');
  assert.strictEqual(A.planHeading({ plan: '7', info: { total: 5, done: 5, blocked: 0, inProgress: 0 } }), 'PLAN-7 · 5/5 done');
  assert.strictEqual(A.planHeading({ plan: '2', info: null }), 'PLAN-2');
  assert.strictEqual(A.planHeading({ plan: null, info: null }), 'Sin plan');
  assert.strictEqual(A.planProgress(g.info), 0.5);
  assert.strictEqual(A.planProgress({ total: 0, done: 0 }), 0);
  assert.strictEqual(A.planProgress({ total: 2, done: 9 }), 1);
  assert.strictEqual(A.planProgress(null), 0);
});

test('faviconLabel: 0 original, 3, 9, 10+ = 9+', () => {
  assert.strictEqual(A.faviconLabel(0), '');
  assert.strictEqual(A.faviconLabel(3), '3');
  assert.strictEqual(A.faviconLabel(9), '9');
  assert.strictEqual(A.faviconLabel(10), '9+');
  assert.strictEqual(A.faviconLabel(57), '9+');
  assert.strictEqual(A.faviconLabel(null), '');
  assert.strictEqual(A.faviconLabel(-1), '');
});

test('filtro por estado: toggle y coincidencia (blocked cuenta como waiting)', () => {
  assert.strictEqual(A.toggleStatusFilter('', 'waiting'), 'waiting');
  assert.strictEqual(A.toggleStatusFilter('waiting', 'waiting'), '');
  assert.strictEqual(A.toggleStatusFilter('waiting', 'busy'), 'busy');
  assert.strictEqual(A.toggleStatusFilter('busy', 'done'), 'busy');
  assert.strictEqual(A.matchesStatus({ status: 'blocked' }, 'waiting'), true);
  assert.strictEqual(A.matchesStatus({ status: 'busy' }, 'waiting'), false);
  assert.strictEqual(A.matchesStatus({ status: 'idle' }, 'idle'), true);
  assert.strictEqual(A.matchesStatus({ status: 'done' }, ''), true);
});

test('matchesQuery encuentra por rama y por nombre del plan', () => {
  const x = { name: 'n', branch: 'feat-plan-042-latido-front', plan: { name: 'PLAN-042-calidad-ejemplo' } };
  assert.strictEqual(A.matchesQuery(x, 'latido'), true);
  assert.strictEqual(A.matchesQuery(x, 'FEAT-PLAN'), true);
  assert.strictEqual(A.matchesQuery(x, 'calidad'), true);
  assert.strictEqual(A.matchesQuery({ name: 'n', branch: null }, 'latido'), false);
});

test('fixture PLAN-005: cubre cada caso del contrato', () => {
  const d = JSON.parse(fs.readFileSync(__dirname + '/fixture.json', 'utf8'));
  const l = d.sessions;
  assert.ok(l.some((x) => x.ask && x.ask.kind === 'question' && x.ask.more > 0 && x.status === 'waiting'));
  assert.ok(l.some((x) => x.ask && x.ask.kind === 'tool' && x.status === 'waiting'));
  assert.ok(l.some((x) => x.ask && x.ask.kind === 'text' && x.status === 'idle'));
  assert.ok(l.some((x) => x.errors && x.errors.high));
  assert.ok(l.some((x) => x.errors && x.errors.count > 0 && !x.errors.high));
  assert.ok(l.some((x) => x.plan && !x.orq));
  assert.ok(l.filter((x) => x.plan && x.plan.number === 42).length >= 2);
  assert.ok(l.some((x) => x.branch && x.branch.length > 80));
  assert.ok(l.some((x) => x.branch && x.branch.startsWith('<')));
});

test('planHeading con y sin unmerged', () => {
  assert.strictEqual(A.planHeading({ plan: '5', info: { total: 3, done: 3, unmerged: 2 } }), 'PLAN-5 · 3/3 done (2 sin mergear)');
  assert.strictEqual(A.planHeading({ plan: '5', info: { total: 3, done: 3, unmerged: 0 } }), 'PLAN-5 · 3/3 done');
  assert.strictEqual(A.planHeading({ plan: '5', info: { total: 3, done: 1, blocked: 1, unmerged: 1 } }), 'PLAN-5 · 1/3 done (1 sin mergear) · 1 blocked');
});

test('historyScale: máximo = 100 %, ceros sin barra pero con lugar, todo en 0 sin NaN', () => {
  const sc = A.historyScale([{ date: 'a', output: 50 }, { date: 'b', output: 0 }, { date: 'c', output: 200 }]);
  assert.deepStrictEqual(sc.map((x) => x.pct), [25, 0, 100]);
  assert.strictEqual(sc.length, 3);
  const z = A.historyScale([{ date: 'a', output: 0 }, { date: 'b' }]);
  assert.deepStrictEqual(z.map((x) => x.pct), [0, 0]);
  assert.deepStrictEqual(A.historyScale(null), []);
});

test('selectDay: por fecha, por defecto el último, null sin días', () => {
  const days = [{ date: '2026-10-06' }, { date: '2026-10-07' }, { date: '2026-10-08' }];
  assert.strictEqual(A.selectDay(days, '2026-10-07'), days[1]);
  assert.strictEqual(A.selectDay(days, null), days[2]);
  assert.strictEqual(A.selectDay(days, 'nope'), days[2]);
  assert.strictEqual(A.selectDay([], 'x'), null);
});

test('historyRows: share del total y total 0 sin dividir por cero', () => {
  const r = A.historyRows([{ name: 'a', output: 1, total: 75 }, { name: 'b', output: 2, total: 25 }]);
  assert.deepStrictEqual(r.map((x) => x.share), [0.75, 0.25]);
  assert.deepStrictEqual(A.historyRows([{ name: 'z', total: 0 }])[0].share, 0);
  assert.deepStrictEqual(A.historyRows(undefined), []);
});

// ---- PLAN-007: bandeja ----
const bandeja = require('./app.js');

const NOW = 1791476712278;
const H = 3600000;
const D = 24 * H;

test('isAbandoned: umbral exacto de 24 h (más de, no desde)', () => {
  const w = (t) => s('w', 'waiting', { lastActivityAt: t });
  assert.strictEqual(bandeja.isAbandoned(w(NOW - D), NOW), false);
  assert.strictEqual(bandeja.isAbandoned(w(NOW - D - 1), NOW), true);
  assert.strictEqual(bandeja.isAbandoned(s('b', 'blocked', { lastActivityAt: NOW - D - 1 }), NOW), true);
});

test('isAbandoned: startedAt de respaldo, sin fechas no es abandonada, done/busy/idle nunca', () => {
  assert.strictEqual(bandeja.isAbandoned(s('a', 'blocked', { lastActivityAt: null, startedAt: NOW - 90 * D }), NOW), true);
  assert.strictEqual(bandeja.isAbandoned(s('a', 'waiting', { lastActivityAt: null, startedAt: NOW - H }), NOW), false);
  assert.strictEqual(bandeja.isAbandoned(s('a', 'waiting', {}), NOW), false);
  ['done', 'busy', 'idle'].forEach((st) => {
    assert.strictEqual(bandeja.isAbandoned(s('x', st, { lastActivityAt: NOW - 90 * D }), NOW), false);
  });
  assert.strictEqual(bandeja.isAbandoned(null, NOW), false);
});

test('counts: las abandonadas no cuentan como esperándote; done no cuenta; desconocido = idle', () => {
  const list = [
    s('a', 'waiting', { lastActivityAt: NOW - H }),
    s('b', 'blocked', { lastActivityAt: NOW - 2 * H }),
    s('c', 'blocked', { startedAt: NOW - 90 * D }),
    s('d', 'waiting', { lastActivityAt: NOW - 3 * D }),
    s('e', 'busy', { lastActivityAt: NOW }),
    s('f', 'idle', { lastActivityAt: NOW - 90 * D }),
    s('g', 'done', {}),
    s('h', 'raro', {}),
  ];
  assert.deepStrictEqual(bandeja.counts(list, NOW), { waiting: 2, busy: 1, idle: 2 });
  assert.deepStrictEqual(bandeja.counts(null, NOW), { waiting: 0, busy: 0, idle: 0 });
  // el favicon sale de este contador
  assert.strictEqual(bandeja.faviconLabel(bandeja.counts(list, NOW).waiting), '2');
});

test('inboxItems: solo waiting/blocked vivas, la más reciente primero', () => {
  const list = [
    s('old', 'waiting', { lastActivityAt: NOW - 5 * H }),
    s('ghost', 'blocked', { startedAt: NOW - 90 * D }),
    s('new', 'blocked', { lastActivityAt: NOW - H }),
    s('busy', 'busy', { lastActivityAt: NOW }),
  ];
  assert.deepStrictEqual(bandeja.inboxItems(list, NOW).map((x) => x.id), ['new', 'old']);
  assert.deepStrictEqual(bandeja.inboxItems([], NOW), []);
});

test('splitAbandoned separa sin perder ninguna', () => {
  const list = [s('a', 'waiting', { lastActivityAt: NOW }), s('b', 'waiting', { lastActivityAt: NOW - 2 * D })];
  const r = bandeja.splitAbandoned(list, NOW);
  assert.deepStrictEqual(r.live.map((x) => x.id), ['a']);
  assert.deepStrictEqual(r.abandoned.map((x) => x.id), ['b']);
});

test('stepSelection: j/k en los bordes no dan la vuelta', () => {
  const keys = ['a', 'b', 'c'];
  assert.strictEqual(bandeja.stepSelection(keys, 'a', 1), 'b');
  assert.strictEqual(bandeja.stepSelection(keys, 'c', 1), 'c');
  assert.strictEqual(bandeja.stepSelection(keys, 'a', -1), 'a');
  assert.strictEqual(bandeja.stepSelection(keys, 'c', -1), 'b');
  assert.strictEqual(bandeja.stepSelection(keys, 'zzz', 1), 'a');
  assert.strictEqual(bandeja.stepSelection(keys, '', -1), 'a');
  assert.strictEqual(bandeja.stepSelection([], 'a', 1), null);
});

test('plainText: negritas, código, encabezado, link y espacios', () => {
  assert.strictEqual(bandeja.plainText('**Listo**: __todo__ `ok`'), 'Listo: todo ok');
  assert.strictEqual(bandeja.plainText('## Resumen\ntexto   con\n\nsaltos'), 'Resumen texto con saltos');
  assert.strictEqual(bandeja.plainText('mirá [el PR](https://example.com/x) ya'), 'mirá el PR ya');
  assert.strictEqual(bandeja.plainText('a # no es encabezado'), 'a # no es encabezado');
  assert.strictEqual(bandeja.plainText(null), '');
  assert.strictEqual(bandeja.plainText(undefined), '');
  assert.strictEqual(bandeja.plainText(42), '42');
});

test('plainText: el texto hostil queda intacto como texto (no se interpreta ni se escapa)', () => {
  const evil = '<img src=x onerror=alert(1)> <script>alert(2)</script>';
  assert.strictEqual(bandeja.plainText(evil), evil);
  assert.strictEqual(bandeja.plainText('**' + evil + '**'), evil);
});

test('oneLine colapsa comandos de varias líneas', () => {
  assert.strictEqual(bandeja.oneLine('Bash: a \\\n  b\n\tc'), 'Bash: a \\ b c');
  assert.strictEqual(bandeja.oneLine(null), '');
});

test('todayView: último día; oculto si no está listo o no hay días', () => {
  const d = { ready: true, days: [{ output: 5, sessions: 1 }, { output: 2600000, sessions: 35 }] };
  assert.deepStrictEqual(bandeja.todayView(d), { output: '2.6M', sessions: 35 });
  assert.strictEqual(bandeja.todayView({ ready: false, days: d.days }), null);
  assert.strictEqual(bandeja.todayView({ ready: true, days: [] }), null);
  assert.strictEqual(bandeja.todayView(null), null);
});

test('franja y fixture: las dos abandonadas del fixture no cuentan ni entran a la franja', () => {
  const fx = require('./fixture.json');
  const now = fx.generatedAt;
  const ab = fx.sessions.filter((x) => bandeja.isAbandoned(x, now));
  assert.strictEqual(ab.length, 2);
  assert.ok(ab.some((x) => x.status === 'blocked' && !x.lastActivityAt));
  assert.ok(ab.some((x) => x.status === 'waiting' && x.kind === 'interactive'));
  const inbox = bandeja.inboxItems(fx.sessions, now);
  assert.ok(inbox.length > 0);
  assert.ok(inbox.every((x) => !bandeja.isAbandoned(x, now)));
  assert.ok(fx.sessions.length - ab.length >= 15, 'el fixture tiene al menos 15 filas vivas');
});

test('plainText: encabezados que quedan en medio al juntar líneas', () => {
  assert.strictEqual(bandeja.plainText('Keycloak cerrado.\n## Lo de Keycloak: listo'), 'Keycloak cerrado. Lo de Keycloak: listo');
  assert.strictEqual(bandeja.plainText('PR #87 y C# quedan'), 'PR #87 y C# quedan');
});

// ---- PLAN-008 / TASK-019: datos de sesión ----

const T0 = 1791476712278;
const MIN = 60000;
const chipTexts = (x) => bandeja.sessionChips(x, T0).map((c) => c.text);

test('chips: subagentes solo con activos > 0', () => {
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { subagents: { active: 0, total: 5 } })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { subagents: null })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { subagents: { active: 1, total: 5 } })), ['1 subagente']);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { subagents: { active: 2, total: 7 } })), ['2 subagentes']);
});

test('chips: turno solo busy y >= 15 min', () => {
  assert.deepStrictEqual(chipTexts(s('a', 'busy', { turnStartedAt: T0 - 14 * MIN })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'busy', { turnStartedAt: T0 - 15 * MIN })), ['turno de 15 min']);
  assert.deepStrictEqual(chipTexts(s('a', 'busy', { turnStartedAt: T0 - 42 * MIN })), ['turno de 42 min']);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { turnStartedAt: T0 - 42 * MIN })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'busy', { turnStartedAt: null })), []);
  assert.strictEqual(bandeja.turnMinutes(s('a', 'waiting', { turnStartedAt: T0 - 20 * MIN }), T0), null);
  assert.strictEqual(bandeja.turnMinutes(s('a', 'busy', { turnStartedAt: T0 + 5 * MIN }), T0), 0);
});

test('chips: permisos ocultos en default y con valores raros', () => {
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { permissionMode: 'default' })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { permissionMode: null })), []);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { permissionMode: 'auto' })), ['auto']);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { permissionMode: '<img src=x>' })), []);
});

test('chips: PR (el último válido), líneas y orden', () => {
  const x = s('a', 'busy', {
    subagents: { active: 2, total: 7 }, turnStartedAt: T0 - 20 * MIN, permissionMode: 'auto',
    lines: { added: 412, removed: 96 },
    prs: [{ number: 85, url: 'https://gl.test/mr/85', repo: 'g/r' }, { number: 87, url: 'https://gl.test/mr/87', repo: 'g/r' }],
  });
  assert.deepStrictEqual(chipTexts(x), ['2 subagentes', 'PR #87', '+412 −96', 'turno de 20 min', 'auto']);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { lines: { added: 0, removed: 0 } })), []);
});

test('URL hostil descartada: PR y artifacts', () => {
  assert.strictEqual(bandeja.validPrUrl('https://gitlab.ejemplo/g/r/-/merge_requests/1'), true);
  ['javascript:alert(1)', 'http://x.test/1', 'https://x.test/a b', 'https://x.test/"onmouseover=1', "https://x.test/'", 'data:text/html,x', '', null, 5]
    .forEach((u) => assert.strictEqual(bandeja.validPrUrl(u), false, String(u)));
  assert.strictEqual(bandeja.validArtifactUrl('https://claude.ai/artifact/abc'), true);
  ['https://evil.example/claude.ai/', 'https://claude.ai.evil.test/x', 'http://claude.ai/x', 'javascript:alert(1)', 'https://claude.ai/a b', null]
    .forEach((u) => assert.strictEqual(bandeja.validArtifactUrl(u), false, String(u)));
  const x = s('a', 'idle', {
    prs: [{ number: 66, url: 'javascript:alert(1)', repo: 'r' }, { number: 0, url: 'https://x.test/0' }, { number: 3, url: 'https://x.test/3' }],
    artifacts: [{ title: 't', url: 'javascript:alert(2)' }, { title: 'ok', url: 'https://claude.ai/artifact/1' }],
  });
  assert.deepStrictEqual(bandeja.validPrs(x).map((p) => p.number), [3]);
  assert.deepStrictEqual(bandeja.validArtifacts(x).map((a) => a.title), ['ok']);
  assert.deepStrictEqual(chipTexts(s('a', 'idle', { prs: [{ number: 66, url: 'javascript:alert(1)' }] })), []);
  assert.deepStrictEqual(bandeja.validPrs({}), []);
});

test('away: línea secundaria solo fuera de busy', () => {
  const away = { text: 'Terminé la **migración**', at: T0 };
  assert.strictEqual(bandeja.awayLine(s('a', 'waiting', { away })), 'Terminé la migración');
  assert.strictEqual(bandeja.awayLine(s('a', 'idle', { away })), 'Terminé la migración');
  assert.strictEqual(bandeja.awayLine(s('a', 'done', { away })), 'Terminé la migración');
  assert.strictEqual(bandeja.awayLine(s('a', 'busy', { away })), '');
  assert.strictEqual(bandeja.awayLine(s('a', 'idle', { away: null })), '');
  assert.strictEqual(bandeja.awayLine(s('a', 'idle', {})), '');
  // el HTML hostil queda como texto literal (quien lo pinta usa textContent)
  assert.strictEqual(bandeja.awayLine(s('a', 'idle', { away: { text: '<img src=x onerror=alert(1)>' } })), '<img src=x onerror=alert(1)>');
});

test('búsqueda por PR y repo; el PR de URL hostil no se encuentra', () => {
  const x = s('a', 'idle', { title: 'algo', prs: [{ number: 87, url: 'https://gl.test/mr/87', repo: 'grupo/demo-api' }] });
  assert.ok(bandeja.matchesQuery(x, 'PR #87'));
  assert.ok(bandeja.matchesQuery(x, 'pr 87'));
  assert.ok(bandeja.matchesQuery(x, '#87'));
  assert.ok(bandeja.matchesQuery(x, 'grupo/demo-api'));
  assert.ok(!bandeja.matchesQuery(x, 'PR #88'));
  const h = s('b', 'idle', { title: 'algo', prs: [{ number: 66, url: 'javascript:alert(1)', repo: 'zzz/repo' }] });
  assert.ok(!bandeja.matchesQuery(h, 'zzz/repo'));
  assert.ok(bandeja.matchesQuery(s('c', 'idle', { title: 'x' }), ''));
});

test('fixture: los 7 campos están en varias sesiones y lo hostil es inerte', () => {
  const fx = require('./fixture.json');
  const withKey = (k) => fx.sessions.filter((x) => (Array.isArray(x[k]) ? x[k].length : x[k] != null));
  ['away', 'prs', 'artifacts', 'lines', 'permissionMode', 'subagents', 'turnStartedAt'].forEach((k) => {
    assert.ok(withKey(k).length >= 2, k);
  });
  const hostil = fx.sessions.find((x) => x.name === 'hostil');
  assert.ok(hostil.prs.some((p) => p.url.startsWith('javascript:')));
  assert.deepStrictEqual(bandeja.validPrs(hostil), []);
  assert.deepStrictEqual(bandeja.validArtifacts(hostil), []);
  assert.deepStrictEqual(chipTexts(hostil), ['3 subagentes']);
  fx.sessions.forEach((x) => {
    bandeja.validPrs(x).forEach((p) => assert.ok(p.url.startsWith('https://')));
    const c = bandeja.sessionChips(x, fx.generatedAt);
    if (x.status !== 'busy') assert.ok(!c.some((y) => /^turno/.test(y.text)));
  });
  const busyAway = fx.sessions.find((x) => x.status === 'busy' && x.away);
  assert.strictEqual(bandeja.awayLine(busyAway), '');
});

// ---- PLAN-010 / TASK-024: rowView (fila de una línea) ----

const RNOW = 1_800_000_000_000;
const RMIN = 60000;
const RVIEW = require('./app.js');

// sesión con TODAS las señales posibles, para probar orden y tope
const cargada = () => s('c', 'busy', {
  title: 'Refactor del cliente',
  project: 'api-backend',
  branch: 'feature/x',
  lastActivityAt: RNOW - 5 * RMIN,
  errors: { count: 4, high: true, windowMin: 10 },
  context: { current: 900000, warn: true },
  rate: { high: true },
  stale: true,
  turnStartedAt: RNOW - 20 * RMIN,
  prs: [{ number: 12, url: 'https://git.test/pr/12', repo: 'org/api-backend' }],
  subagents: { active: 2, total: 3 },
  lines: { added: 10, removed: 2 },
  permissionMode: 'acceptEdits'
});

test('rowView: prioridad de chips y tope de 2 con "+N" y extraTitle', () => {
  const v = RVIEW.rowView(cargada(), { now: RNOW, projectFilter: '' });
  assert.strictEqual(v.chips.length, 2);
  assert.ok(/errores/.test(v.chips[0].text));
  assert.ok(/contexto/.test(v.chips[1].text));
  // quedan: ritmo, stale, turno, PR, subagentes, líneas, permisos = 7
  assert.strictEqual(v.extra, 7);
  assert.ok(v.extraTitle.indexOf('ritmo') !== -1 && v.extraTitle.indexOf('PR #12') !== -1);
  assert.ok(v.extraTitle.indexOf('errores') === -1);
});

test('rowView: orden completo errores > contexto > ritmo > stale > turno > PR > subagentes > líneas > permisos', () => {
  const v = RVIEW.rowView(cargada(), { now: RNOW, projectFilter: '', maxChips: 99 });
  const kinds = v.chips.map((c) => {
    if (/errores/.test(c.text)) return 'err';
    if (/contexto/.test(c.text)) return 'ctx';
    if (/ritmo/.test(c.text)) return 'rate';
    if (/sin actividad/.test(c.text)) return 'stale';
    if (/^turno/.test(c.text)) return 'turn';
    if (/^PR/.test(c.text)) return 'pr';
    if (/subagente/.test(c.text)) return 'sub';
    if (c.lines) return 'lines';
    return 'perm';
  });
  assert.deepStrictEqual(kinds, ['err', 'ctx', 'rate', 'stale', 'turn', 'pr', 'sub', 'lines', 'perm']);
  assert.strictEqual(v.extra, 0);
});

test('rowView: turno corto no es chip; maxChips 1 (teléfono) manda el resto a "+N"', () => {
  const corto = s('t', 'busy', { title: 't', turnStartedAt: RNOW - 3 * RMIN, subagents: { active: 1, total: 1 }, lines: { added: 1, removed: 0 } });
  const v = RVIEW.rowView(corto, { now: RNOW, projectFilter: '', maxChips: 1 });
  assert.strictEqual(v.chips.length, 1);
  assert.ok(/subagente/.test(v.chips[0].text));
  assert.strictEqual(v.extra, 1);
  const sin = RVIEW.rowView(s('z', 'idle', { title: 'z' }), { now: RNOW, projectFilter: '' });
  assert.deepStrictEqual(sin.chips, []);
  assert.strictEqual(sin.extra, 0);
});

test('rowView: proyecto oculto con filtro de proyecto activo', () => {
  const x = s('p', 'idle', { title: 'algo', project: 'web-app' });
  assert.strictEqual(RVIEW.rowView(x, { now: RNOW, projectFilter: '' }).project, 'web-app');
  assert.strictEqual(RVIEW.rowView(x, { now: RNOW, projectFilter: 'web-app' }).project, null);
  assert.strictEqual(RVIEW.rowView(s('q', 'idle', { title: 'x' }), { now: RNOW, projectFilter: '' }).project, null);
});

test('rowView: tooltip = away, o ask de idle, o proyecto · rama; ariaLabel con estado en texto y hora', () => {
  const base = { title: 'Migración', project: 'web-app', branch: 'main', lastActivityAt: RNOW - 2 * RMIN };
  const conAway = RVIEW.rowView(s('a', 'idle', Object.assign({ away: { text: '**Terminó** la migración' } }, base)), { now: RNOW, projectFilter: '' });
  assert.strictEqual(conAway.tooltip, 'Terminó la migración');
  const conAsk = RVIEW.rowView(s('b', 'idle', Object.assign({ ask: { kind: 'text', text: 'Listo, ¿sigo?' } }, base)), { now: RNOW, projectFilter: '' });
  assert.strictEqual(conAsk.tooltip, 'Listo, ¿sigo?');
  const plano = RVIEW.rowView(s('c', 'busy', base), { now: RNOW, projectFilter: '' });
  assert.strictEqual(plano.tooltip, 'web-app · ⎇ main');
  const esperando = RVIEW.rowView(s('d', 'waiting', base), { now: RNOW, projectFilter: '' });
  assert.strictEqual(esperando.ariaLabel, 'Migración, esperándote, hace 2 min');
  assert.strictEqual(esperando.when, 'hace 2 min');
  assert.ok(/trabajando/.test(plano.ariaLabel));
});

test('rowView: texto hostil queda como texto (sin HTML interpretado, sin romper)', () => {
  const x = s('h', 'idle', {
    title: '<img src=x onerror=alert(1)> **negrita**',
    project: '<script>alert(1)</script>',
    away: { text: '<b onclick=x()>hola</b>' }
  });
  const v = RVIEW.rowView(x, { now: RNOW, projectFilter: '' });
  assert.strictEqual(v.title, '<img src=x onerror=alert(1)> negrita');
  assert.strictEqual(v.project, '<script>alert(1)</script>');
  assert.strictEqual(v.tooltip, '<b onclick=x()>hola</b>');
});

test('rowView: sin fecha y estado desconocido no revientan', () => {
  const v = RVIEW.rowView({ id: 'x', status: 'zzz' }, { now: RNOW, projectFilter: '' });
  assert.strictEqual(v.status, 'idle');
  assert.strictEqual(v.when, '—');
  assert.strictEqual(v.title, 'x');
});

test('rowView sobre el fixture de lista: ninguna fila revienta y el tope se respeta', () => {
  const fx = require('./fixture-lista.json');
  assert.ok(fx.sessions.length >= 40);
  fx.sessions.forEach((x) => {
    const v = RVIEW.rowView(x, { now: fx.generatedAt, projectFilter: '' });
    assert.ok(v.chips.length <= 2);
    assert.strictEqual(typeof v.title, 'string');
    assert.ok(v.ariaLabel.indexOf(v.when) !== -1);
  });
});

// ---- PLAN-010 / TASK-025: grupos por estado (DD-38, DD-39) ----

const GNOW = 1_800_000_000_000;
const GHOUR = 3600 * 1000;
const gat = (hoursAgo) => ({ lastActivityAt: GNOW - hoursAgo * GHOUR });

test('statusGroups: la waiting no abandonada no aparece en ningún grupo', () => {
  const list = [s('w', 'waiting', gat(1)), s('k', 'blocked', gat(2)), s('b', 'busy', gat(1)), s('i', 'idle', gat(1))];
  const groups = A.statusGroups(list, GNOW);
  assert.deepStrictEqual(groups.map((g) => g.id), ['busy', 'idle']);
  const ids = groups.flatMap((g) => g.sessions.map((x) => x.id));
  assert.deepStrictEqual(ids, ['b', 'i']);
});

test('statusGroups: una waiting de hace 3 días va a abandonadas', () => {
  const groups = A.statusGroups([s('old', 'waiting', gat(72)), s('blk', 'blocked', gat(30))], GNOW);
  assert.deepStrictEqual(groups.map((g) => g.id), ['abandoned']);
  assert.deepStrictEqual(groups[0].sessions.map((x) => x.id), ['old', 'blk']);
  assert.strictEqual(groups[0].label, 'Abandonadas');
});

test('statusGroups: terminadas recientes antes que las viejas, ambas en "Terminadas"', () => {
  const list = [s('viejo', 'done', gat(48)), s('nuevo', 'done', gat(2)), s('i', 'idle', gat(1))];
  const groups = A.statusGroups(list, GNOW);
  const done = groups.find((g) => g.id === 'done');
  assert.strictEqual(done.label, 'Terminadas');
  assert.deepStrictEqual(done.sessions.map((x) => x.id), ['nuevo', 'viejo']);
});

test('statusGroups: orden fijo, sin grupos vacíos, tolera null', () => {
  const list = [s('d', 'done', gat(1)), s('i', 'idle', gat(1)), s('b', 'busy', gat(1)), s('a', 'waiting', gat(100))];
  assert.deepStrictEqual(A.statusGroups(list, GNOW).map((g) => g.id), ['busy', 'idle', 'done', 'abandoned']);
  assert.deepStrictEqual(A.statusGroups([s('d', 'done', gat(1))], GNOW).map((g) => g.id), ['done']);
  assert.deepStrictEqual(A.statusGroups([], GNOW), []);
  assert.deepStrictEqual(A.statusGroups(null, GNOW), []);
});

test('listTotals: cuenta terminadas (todas) y abandonadas; las waiting vivas no cuentan', () => {
  const list = [
    s('d1', 'done', gat(1)), s('d2', 'done', gat(50)), s('d3', 'done'),
    s('a1', 'waiting', gat(30)), s('a2', 'blocked', gat(90)),
    s('w', 'waiting', gat(1)), s('b', 'busy', gat(1)),
  ];
  assert.deepStrictEqual(A.listTotals(list, GNOW), { done: 3, abandoned: 2 });
  assert.deepStrictEqual(A.listTotals(null, GNOW), { done: 0, abandoned: 0 });
});

// ---- PLAN-010 / TASK-026: clave única y colapso "×N" (DD-40) ----

const cs = (id, status, extra) => Object.assign({ id, sessionId: id, status, cwd: '/home/user/api-backend' }, extra);

test('rowKey: gemelos con el mismo sessionId y distinto PID tienen claves distintas', () => {
  const a = { id: 'x1', sessionId: 'abc', pid: 100 };
  const b = { id: 'x2', sessionId: 'abc', pid: 200 };
  assert.strictEqual(A.rowKey(a), 'abc#100');
  assert.notStrictEqual(A.rowKey(a), A.rowKey(b));
});

test('rowKey: pid 0 o ausente → sin sufijo; sin sessionId cae al id', () => {
  assert.strictEqual(A.rowKey({ id: 'x', sessionId: 'abc', pid: 0 }), 'abc');
  assert.strictEqual(A.rowKey({ id: 'x', sessionId: 'abc' }), 'abc');
  assert.strictEqual(A.rowKey({ id: 'only', pid: 7 }), 'only#7');
});

test('resolveSession: clave exacta; la vieja sin # cae a la primera con ese sessionId', () => {
  const list = [{ id: 'a', sessionId: 'abc', pid: 1 }, { id: 'b', sessionId: 'abc', pid: 2 }];
  assert.strictEqual(A.resolveSession(list, 'abc#2').id, 'b');
  assert.strictEqual(A.resolveSession(list, 'abc').id, 'a');
  assert.strictEqual(A.resolveSession(list, 'abc#9'), null);
  assert.strictEqual(A.resolveSession(list, ''), null);
});

test('notifyTargets: gemelos de mismo sessionId no se confunden', () => {
  const prev = [{ id: 'a', sessionId: 'abc', pid: 1, status: 'busy' }, { id: 'b', sessionId: 'abc', pid: 2, status: 'waiting' }];
  const next = [{ id: 'a', sessionId: 'abc', pid: 1, status: 'waiting' }, { id: 'b', sessionId: 'abc', pid: 2, status: 'waiting' }];
  assert.deepStrictEqual(A.notifyTargets(prev, next).map((x) => x.id), ['a']);
});

test('collapseRepeats: gemelos de mismo sessionId → una fila ×2', () => {
  const r = A.collapseRepeats([cs('a', 'idle', { sessionId: 'abc', pid: 1, title: 'Fix login' }),
    cs('b', 'idle', { sessionId: 'abc', pid: 2, title: 'Fix login' })], 'idle');
  assert.strictEqual(r.length, 1);
  assert.strictEqual(r[0].copies.length, 2);
});

test('collapseRepeats: mismo cwd + título → ×2; mismo título en otro cwd → 2 filas', () => {
  const same = A.collapseRepeats([cs('a', 'idle', { title: 'Refactor' }), cs('b', 'idle', { title: 'Refactor' })], 'idle');
  assert.strictEqual(same.length, 1);
  const diff = A.collapseRepeats([cs('a', 'idle', { title: 'Refactor' }),
    cs('b', 'idle', { title: 'Refactor', cwd: '/home/user/web-app' })], 'idle');
  assert.strictEqual(diff.length, 2);
});

test('collapseRepeats: título vacío o ausente nunca colapsa', () => {
  const r = A.collapseRepeats([cs('a', 'done'), cs('b', 'done'), cs('c', 'done', { title: '  ' })], 'done');
  assert.strictEqual(r.length, 3);
});

test('collapseRepeats: busy y waiting nunca colapsan', () => {
  const l = [cs('a', 'busy', { title: 'Same' }), cs('b', 'busy', { title: 'Same' })];
  assert.strictEqual(A.collapseRepeats(l, 'busy').length, 2);
  assert.strictEqual(A.collapseRepeats(l, 'waiting').length, 2);
});

test('collapseRepeats: tres done iguales → ×3 con la más reciente como head', () => {
  const r = A.collapseRepeats([cs('a', 'done', { title: 'T', lastActivityAt: 100 }),
    cs('b', 'done', { title: 'T', lastActivityAt: 300 }), cs('c', 'done', { title: 'T', lastActivityAt: 200 })], 'done');
  assert.strictEqual(r.length, 1);
  assert.strictEqual(r[0].head.id, 'b');
  assert.deepStrictEqual(r[0].copies.map((x) => x.id), ['a', 'b', 'c']);
});

test('collapseRepeats: mayúsculas, tildes y markdown no impiden el colapso; conserva el orden', () => {
  const r = A.collapseRepeats([cs('x', 'idle', { title: 'Otra cosa' }),
    cs('a', 'idle', { title: 'Revisión **Rápida**' }), cs('b', 'idle', { title: 'revision rapida' })], 'idle');
  assert.deepStrictEqual(r.map((e) => e.head.id), ['x', 'a']);
  assert.strictEqual(r[1].copies.length, 2);
});

test('collapseRepeats: tolera null y sesiones sin cwd', () => {
  assert.deepStrictEqual(A.collapseRepeats(null, 'idle'), []);
  const r = A.collapseRepeats([{ id: 'a', status: 'idle', title: 'T' }, { id: 'b', status: 'idle', title: 'T' }], 'idle');
  assert.strictEqual(r.length, 2);
});

test('collapseMixed: colapsa por estado sin mezclar idle con done', () => {
  const r = A.collapseMixed([cs('a', 'idle', { title: 'T' }), cs('b', 'done', { title: 'T' }),
    cs('c', 'idle', { title: 'T' }), cs('d', 'busy', { title: 'T' })], GNOW);
  assert.deepStrictEqual(r.map((e) => e.copies.length), [2, 1, 1]);
});

test('rowView: el chip ×N va primero y cuenta para el tope', () => {
  const v = A.rowView(cs('a', 'idle', { title: 'T', errors: { count: 2, last: 'x' }, stale: true }),
    { now: GNOW, maxChips: 2, copies: 3 });
  assert.strictEqual(v.chips[0].text, '×3');
  assert.strictEqual(v.chips.length, 2);
  assert.match(v.chips[0].title, /3 sesiones con el mismo título/);
  assert.strictEqual(A.rowView(cs('a', 'idle'), { now: GNOW, copies: 1 }).chips.every((c) => c.cls !== 'rep'), true);
});
