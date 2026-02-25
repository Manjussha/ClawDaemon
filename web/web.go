// Package web embeds the frontend static files into the binary.
package web

import "embed"

//go:embed *.html
var Files embed.FS
