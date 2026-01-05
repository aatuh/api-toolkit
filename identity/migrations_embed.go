package identity

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrationsFS returns the embedded identity migrations filesystem.
func MigrationsFS() (fs.FS, error) {
	return fs.Sub(migrations, "migrations")
}

// MustMigrationsFS returns the embedded migrations filesystem or panics.
func MustMigrationsFS() fs.FS {
	sub, err := MigrationsFS()
	if err != nil {
		panic("identity migrations fs not available")
	}
	return sub
}
