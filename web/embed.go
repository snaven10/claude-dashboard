// Package web embebe el frontend estático (sin build, sin CDN).
package web

import "embed"

// FS contiene index.html, app.js, app.css, los fixtures y vendor/qrcode.js (app.test.js no).
//
//go:embed index.html app.js app.css fixture.json fixture-timeline.json fixture-history.json vendor/qrcode.js
var FS embed.FS
