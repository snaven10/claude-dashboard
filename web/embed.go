// Package web embebe el frontend estático (sin build, sin CDN).
package web

import "embed"

// FS contiene index.html, app.js, app.css, los fixtures (fixture.json, fixture-lista.json,
// fixture-timeline.json, fixture-history.json), vendor/qrcode.js, la PWA (manifest.webmanifest,
// sw.js, icons/*.png). Los tests (app.test.js, sw.test.js) no.
//
//go:embed index.html app.js app.css fixture.json fixture-lista.json fixture-timeline.json fixture-history.json vendor/qrcode.js manifest.webmanifest sw.js icons/*.png
var FS embed.FS
