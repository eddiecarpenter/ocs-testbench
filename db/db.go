// Package db exposes the embedded SQL migration files.
package db

import "embed"

//go:embed migrations
var Migrations embed.FS
