// Package changelog embeds this service's release notes, served at
// /release-notes (github.com/salahfarzin/utils/releasenotes).
package changelog

import "embed"

// Files are the changelog's Markdown files: unreleased.md and one per release.
//
//go:embed *.md
var Files embed.FS
