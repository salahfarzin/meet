package swagger

import (
	"embed"
)

// UI is the filesystem for the Swagger UI assets.
//
//go:embed all:assets
var UI embed.FS

// Gen is the filesystem for the generated Swagger definitions.
//
//go:embed all:gen
var Gen embed.FS
