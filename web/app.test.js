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
const ID = 'session_01PheFY7bAAutvpmHjzYFAFy';
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
