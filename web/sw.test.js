'use strict';
const test = require('node:test');
const assert = require('node:assert');
const { shouldHandle, safeUrl, parsePayload, SHELL } = require('./sw.js');

test('shouldHandle: API y fixtures nunca se interceptan', () => {
  assert.strictEqual(shouldHandle('/api/sessions'), false);
  assert.strictEqual(shouldHandle('/api/events'), false);
  assert.strictEqual(shouldHandle('/api/sessions/x/timeline'), false);
  assert.strictEqual(shouldHandle('/fixture.json'), false);
  assert.strictEqual(shouldHandle('/fixture-timeline.json'), false);
  assert.strictEqual(shouldHandle('fixture.json'), false);
});

test('shouldHandle: shell sí; no-GET y otros orígenes no', () => {
  assert.strictEqual(shouldHandle('/'), true);
  assert.strictEqual(shouldHandle('/app.js'), true);
  assert.strictEqual(shouldHandle('/app.js', 'POST'), false);
  assert.strictEqual(shouldHandle('https://x.example/app.js', 'GET', 'https://me.example'), false);
  assert.strictEqual(shouldHandle('https://me.example/app.js', 'GET', 'https://me.example'), true);
  assert.strictEqual(shouldHandle('https://me.example/api/sessions', 'GET', 'https://me.example'), false);
});

test('parsePayload: inválido -> aviso genérico', () => {
  for (const bad of ['', 'no es json', 'null', '[1]', '"x"', undefined]) {
    const p = parsePayload(bad);
    assert.strictEqual(p.title, 'claude-dashboard');
    assert.strictEqual(p.url, '/');
  }
});

test('parsePayload: válido se respeta', () => {
  const p = parsePayload(JSON.stringify({ v: 1, title: 'T', body: 'B', tag: 'cd-1', url: '/?s=abc' }));
  assert.deepStrictEqual(p, { title: 'T', body: 'B', tag: 'cd-1', url: '/?s=abc' });
});

test('url: solo rutas relativas al origen', () => {
  assert.strictEqual(safeUrl('//evil.example'), '/');
  assert.strictEqual(safeUrl('https://evil.example'), '/');
  assert.strictEqual(safeUrl('/\\evil.example'), '/');
  assert.strictEqual(safeUrl('javascript:alert(1)'), '/');
  assert.strictEqual(safeUrl(42), '/');
  assert.strictEqual(safeUrl('/?s=abc'), '/?s=abc');
  assert.strictEqual(parsePayload('{"title":"x","url":"//evil.example"}').url, '/');
});

test('precache: sin rutas de API ni fixtures', () => {
  for (const p of SHELL) assert.ok(!/api|fixture/.test(p), p);
});
