package web

import "embed"

// Content is the local management UI (index.html etc).
//
//go:embed index.html
var Content embed.FS
