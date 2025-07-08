// Package crdb provides an embedded filesystem containing all the crdb migrations
package crdb

import (
	"embed"
)

// Migrations contain an embedded filesystem with all the sql migration files
//
//go:embed migrations/*.sql
var Migrations embed.FS
