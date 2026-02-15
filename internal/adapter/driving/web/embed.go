package web

import "embed"

// StaticFS holds the embedded static assets (vendor JS, CSS, animation JS).
//
//go:embed static/*
var StaticFS embed.FS
