package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/logzap"
	"github.com/aatuh/api-toolkit/contrib/v3/bootstrap"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")), "migration directory")
	table := flags.String("table", "schema_migrations", "migration table")
	lock := flags.Int64("lock", 0, "advisory lock key")
	timeout := flags.Duration("timeout", 15*time.Minute, "migration timeout")
	allowDangerousDown := flags.Bool("allow-dangerous-down", false, "allow local/schema-teardown down migrations")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("command required: plan | up | status | check | verify | down")
	}
	if strings.TrimSpace(*dir) == "" {
		*dir = "migrations"
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	command := strings.ToLower(flags.Arg(0))
	downAllowed := command == "down" && *allowDangerousDown && strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_DANGEROUS_MIGRATION_DOWN")), "true")
	migrator, err := bootstrap.NewMigrator(databaseURL, *table, *lock, downAllowed, logzap.NewProduction(), []string{*dir}, nil)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer migrator.Close()

	switch command {
	case "plan":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		fmt.Print(status)
		return nil
	case "up":
		if err := bootstrap.RunUp(ctx, migrator, *dir); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "migrations applied")
		return nil
	case "status":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		fmt.Print(status)
		return nil
	case "check":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		if strings.Contains(status, "*") {
			fmt.Print(status)
			return errors.New("pending migrations")
		}
		fmt.Fprintln(os.Stdout, "migrations up-to-date")
		return nil
	case "verify":
		status, err := bootstrap.Status(ctx, migrator, *dir)
		if err != nil {
			return err
		}
		if strings.Contains(status, "checksum") || strings.Contains(status, "dirty") {
			fmt.Print(status)
			return errors.New("migration verification failed")
		}
		fmt.Fprintln(os.Stdout, "migration checksums verified")
		return nil
	case "down":
		if !downAllowed {
			return errors.New("down migrations require --allow-dangerous-down and ALLOW_DANGEROUS_MIGRATION_DOWN=true")
		}
		if err := bootstrap.RunDown(ctx, migrator, *dir); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "one migration reverted")
		return nil
	default:
		return errors.New("unknown command; expected plan, up, status, check, verify, or down")
	}
}
