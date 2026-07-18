// Package db embeds the SQL migration files so the migrate binary
// needs no filesystem access at deploy time.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
