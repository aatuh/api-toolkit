package migrate

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit-contrib/adapters/logzap"
	"github.com/aatuh/api-toolkit-contrib/bootstrap"
)

// Config customizes how the migration runner behaves.
type Config struct {
	Embedded    fs.FS   // deprecated: use EmbeddedFSs
	EmbeddedFSs []fs.FS // multiple embedded sources
	EnvVar      string
	Timeout     time.Duration
}

// Run executes the migrate CLI workflow using the provided configuration.
func Run(cfg Config) {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	dir := flags.String("dir", "", "migrations dir override")
	table := flags.String("table", "schema_migrations", "schema_migrations table")
	lock := flags.Int64("lock", 0, "advisory lock key")
	allowD := flags.Bool("allow-down", false, "enable down")

	if err := flags.Parse(os.Args[1:]); err != nil {
		log.Fatalf("parse flags: %v", err)
	}
	if flags.NArg() == 0 {
		log.Fatal("command required: up | down | status")
	}
	cmd := strings.ToLower(flags.Arg(0))

	envVar := cfg.EnvVar
	if envVar == "" {
		envVar = "DATABASE_URL"
	}
	dsn := os.Getenv(envVar)
	if dsn == "" {
		log.Fatalf("%s env is required", envVar)
	}

	ctx := context.Background()
	pool, err := bootstrap.OpenAndPingDB(ctx, dsn, 3*time.Second)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer pool.Close()

	var dirs []string
	var embeddedFS []fs.FS
	if *dir != "" {
		dirs = []string{*dir}
	} else {
		embeddedFS = cfg.EmbeddedFSs
		if len(embeddedFS) == 0 && cfg.Embedded != nil {
			embeddedFS = []fs.FS{cfg.Embedded}
		}
		if len(embeddedFS) == 0 {
			log.Fatal("embedded migrations FS is required when no directory override is provided")
		}
	}
	m, err := bootstrap.NewMigrator(dsn, *table, *lock, *allowD, logzap.NewProduction(), dirs, embeddedFS)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.Printf("close migrator: %v", err)
		}
	}()

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	migrationDir := "."
	if *dir != "" {
		migrationDir = *dir
	}

	switch cmd {
	case "up":
		if err := bootstrap.RunUp(ctx, m, migrationDir); err != nil {
			log.Fatal(err)
		}
		log.Println("migrations applied successfully")
	case "down":
		if err := bootstrap.RunDown(ctx, m, migrationDir); err != nil {
			log.Fatal(err)
		}
		log.Println("migrations rolled back successfully")
	case "status":
		status, err := bootstrap.Status(ctx, m, migrationDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(status)
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}
