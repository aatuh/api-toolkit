package migrations

import "embed"

// Embed migrations so services can run without a filesystem dir.
//
//go:embed *.sql
var Migrations embed.FS
