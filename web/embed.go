// Package web exposes the embedded Vite build output. The Go file lives next
// to dist/ because go:embed cannot traverse outside the package directory.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
