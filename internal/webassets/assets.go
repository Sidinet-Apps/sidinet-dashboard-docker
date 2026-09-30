package webassets

import "embed"

//go:embed web/index.html web/static/css/* web/static/js/*
var FS embed.FS
