// Package web embebe el frontend estático (sin build, sin CDN).
package web

import "embed"

// FS contiene index.html, app.js, app.css y los fixtures en la raíz (app.test.js no).
//
//go:embed index.html app.js app.css fixture.json fixture-timeline.json
var FS embed.FS
