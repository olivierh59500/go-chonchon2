// Package assets embeds original presentation resources.
package assets

import "embed"

//go:embed original/*
var Files embed.FS
