package migrations

import "embed"

// Migrations embeds SQL migrations so services can run without a filesystem dir.
//
//go:embed *.sql
var Migrations embed.FS
