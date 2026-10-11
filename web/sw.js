'use strict';
// Service worker del panel (DD-54). Dos trabajos: recibir push con la app
// cerrada y dejar el shell disponible sin red. NUNCA cachea datos de sesiones:
// /api/* y los fixtures ni se interceptan.

const CACHE_PREFIX = 'cd-shell-';
const CACHE_NAME = CACHE_PREFIX + 'v1';
// Solo el shell estático. Ninguna ruta /api ni fixture va acá.
const SHELL = [
  '/',
  'app.js',
  'app.css',
  'vendor/qrcode.js',
  'manifest.webmanifest',
  'icons/icon-192.png',
  'icons/icon-512.png',
  'icons/icon-maskable-512.png',
  'icons/apple-touch-icon.png',
];

const GENERIC_TITLE = 'claude-dashboard';
const GENERIC_BODY = 'Una sesión te espera';

// shouldHandle decide si el SW intercepta la petición: solo GET del mismo
// origen que no sea API ni fixture ni el propio sw.js.
function shouldHandle(url, method, origin) {
  method = method || 'GET';
  if (method !== 'GET') return false;
  let u;
  try {
    u = new URL(url, origin || 'http://localhost');
  } catch (e) {
    return false;
  }
  if (origin && u.origin !== origin) return false;
  const p = u.pathname;
  if (p === '/api' || p.startsWith('/api/')) return false;
  if (/(^|\/)fixture[^/]*\.json$/.test(p)) return false;
  if (p === '/sw.js') return false;
  return true;
}

// safeUrl respeta solo rutas relativas al origen: empieza con "/" y no con "//"
// (ni "/\", que los navegadores tratan como "//").
function safeUrl(url) {
  if (typeof url !== 'string') return '/';
  if (!url.startsWith('/') || url.startsWith('//') || url.startsWith('/\\')) return '/';
  return url;
}

function genericPayload() {
  return { title: GENERIC_TITLE, body: GENERIC_BODY, tag: 'cd-generic', url: '/' };
}

// parsePayload nunca tira: si el JSON no parsea o no es objeto, aviso genérico
// (iOS revoca la suscripción si un push no muestra aviso).
function parsePayload(text) {
  let d;
  try {
    d = JSON.parse(text);
  } catch (e) {
    return genericPayload();
  }
  if (!d || typeof d !== 'object' || Array.isArray(d)) return genericPayload();
  const str = (v, def) => (typeof v === 'string' && v !== '' ? v : def);
  return {
    title: str(d.title, GENERIC_TITLE),
    body: typeof d.body === 'string' ? d.body : GENERIC_BODY,
    tag: str(d.tag, 'cd-generic'),
    url: safeUrl(d.url),
  };
}

if (typeof self !== 'undefined' && typeof self.addEventListener === 'function') {
  self.addEventListener('install', (event) => {
    event.waitUntil(
      caches
        .open(CACHE_NAME)
        .then((c) => c.addAll(SHELL))
        .then(() => self.skipWaiting())
        .catch(() => self.skipWaiting()),
    );
  });

  self.addEventListener('activate', (event) => {
    event.waitUntil(
      caches
        .keys()
        .then((keys) =>
          Promise.all(
            keys.filter((k) => k.startsWith(CACHE_PREFIX) && k !== CACHE_NAME).map((k) => caches.delete(k)),
          ),
        )
        .then(() => self.clients.claim()),
    );
  });

  self.addEventListener('fetch', (event) => {
    const req = event.request;
    if (!shouldHandle(req.url, req.method, self.location.origin)) return;
    event.respondWith(
      fetch(req)
        .then((res) => {
          if (res && res.ok) {
            const copy = res.clone();
            caches.open(CACHE_NAME).then((c) => c.put(req, copy)).catch(() => {});
          }
          return res;
        })
        .catch(() => caches.match(req).then((hit) => hit || Response.error())),
    );
  });

  self.addEventListener('push', (event) => {
    let p;
    try {
      p = parsePayload(event.data ? event.data.text() : '');
    } catch (e) {
      p = genericPayload();
    }
    event.waitUntil(
      self.registration.showNotification(p.title, {
        body: p.body,
        tag: p.tag,
        icon: 'icons/icon-192.png',
        data: { url: p.url },
      }),
    );
  });

  self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    const data = event.notification.data || {};
    const target = safeUrl(data.url);
    event.waitUntil(
      self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((wins) => {
        for (const w of wins) {
          if (typeof w.focus === 'function') {
            if (target !== '/' && typeof w.navigate === 'function') {
              return w.navigate(target).then((c) => (c || w).focus());
            }
            return w.focus();
          }
        }
        return self.clients.openWindow(target);
      }),
    );
  });
}

if (typeof module !== 'undefined') {
  module.exports = { shouldHandle, safeUrl, parsePayload, SHELL, CACHE_NAME };
}
