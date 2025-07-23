// Package psql provides an embedded filesystem containing all the psql migrations
package psql

import (
	"embed"
)

// Migrations contain an embedded filesystem with all the sql migration files
//
//go:embed migrations/*.sql
var Migrations embed.FS
