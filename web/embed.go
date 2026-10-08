// Package web embebe el frontend estático (sin build, sin CDN).
package web

import "embed"

// FS contiene index.html, app.js, app.css y fixture.json en la raíz.
//
//go:embed index.html app.js app.css fixture.json
var FS embed.FS
