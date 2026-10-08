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
