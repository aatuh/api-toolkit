package migrator

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Runner executes SQL migrations transactionally and records state.
type Runner struct {
	DB         *sql.DB
	Opts       Options
	migrations []*Migration
	loaded     bool
}

// Options configures the runner.
type Options struct {
	// MigrationsDir is kept for backward compatibility. If non-empty,
	// it will be appended to MigrationsDirs.
	MigrationsDir string
	// MigrationsDirs allows loading migrations from multiple directories.
	// Directories are processed in order; later files with the same name
	// override earlier ones before parsing.
	MigrationsDirs []string
	// EmbeddedFSs allows loading migrations from multiple embedded
	// filesystems. Each is expected to contain a "migrations" directory.
	// Embedded filesystems are appended after explicit directories.
	EmbeddedFSs []fs.FS
	TableName   string
	LockKey     int64
	// AllowDangerousDown enables Down(); keep disabled in API binaries.
	AllowDangerousDown bool
	// Logger outputs formatted status messages.
	Logger func(format string, args ...any)
}

// Migration is a versioned SQL change.
type Migration struct {
	Version  int64
	Name     string
	Dir      string // "up" or "down"
	File     string
	SQL      string
	Checksum string
}

var (
	// Accept 8-digit (YYYYMMDD) or 14-digit (YYYYMMDDHHMMSS) version
	// prefixes for migration filenames.
	fileRe = regexp.MustCompile(
		`^(\d{8,14})_([a-zA-Z0-9_\-]+)\.(up|down)\.sql$`,
	)
	defaultTable = "schema_migrations"
	defaultLock  = int64(913551337114213777)
)

func New(db *sql.DB, opts Options) *Runner {
	if opts.TableName == "" {
		opts.TableName = defaultTable
	}
	if opts.LockKey == 0 {
		opts.LockKey = defaultLock
	}
	return &Runner{DB: db, Opts: opts}
}

// Up applies all pending "up" migrations.
func (r *Runner) Up(ctx context.Context) error {
	return r.withLock(ctx, func(ctx context.Context) error {
		if err := r.ensureTable(ctx); err != nil {
			return err
		}
		if err := r.loadMigrations(); err != nil {
			return err
		}
		applied, err := r.loadApplied(ctx)
		if err != nil {
			return err
		}
		pending := r.pendingUp(applied)
		if len(pending) == 0 {
			r.log("migrations up-to-date")
			return nil
		}
		for _, m := range pending {
			if err := r.applyOne(ctx, m); err != nil {
				return err
			}
		}
		return nil
	})
}

// Down reverts the latest successfully applied version by 1 step.
func (r *Runner) Down(ctx context.Context) error {
	if !r.Opts.AllowDangerousDown {
		return errors.New(
			"down is disabled; set AllowDangerousDown=true to enable",
		)
	}
	return r.withLock(ctx, func(ctx context.Context) error {
		if err := r.ensureTable(ctx); err != nil {
			return err
		}
		if err := r.loadMigrations(); err != nil {
			return err
		}
		appliedOK, err := r.loadAppliedSuccess(ctx)
		if err != nil {
			return err
		}
		if len(appliedOK) == 0 {
			r.log("no successful migrations to revert")
			return nil
		}
		latest := appliedOK[len(appliedOK)-1]
		down := r.find(latest.Version, "down")
		if down == nil {
			return fmt.Errorf(
				"no down migration for version %d (%s)",
				latest.Version, latest.Name,
			)
		}
		return r.revertOne(ctx, down)
	})
}

// Status reports current and pending state.
func (r *Runner) Status(ctx context.Context) (string, error) {
	if err := r.ensureTable(ctx); err != nil {
		return "", err
	}
	if err := r.loadMigrations(); err != nil {
		return "", err
	}
	applied, err := r.loadApplied(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("table: %s\n", r.Opts.TableName))
	if len(applied) == 0 {
		b.WriteString("applied: none\n")
	} else {
		b.WriteString("applied:\n")
		for _, a := range applied {
			b.WriteString(fmt.Sprintf(
				"  %d %s at %s ok=%t\n",
				a.Version, a.Name, a.AppliedAt.Format(time.RFC3339),
				a.Success,
			))
		}
	}
	var up []*Migration
	for _, m := range r.migrations {
		if m.Dir == "up" {
			up = append(up, m)
		}
	}
	next := r.pendingUp(applied)
	b.WriteString("available up:\n")
	for _, m := range up {
		flag := " "
		for _, n := range next {
			if n.Version == m.Version {
				flag = "*"
				break
			}
		}
		b.WriteString(fmt.Sprintf("  %s %d %s\n", flag, m.Version, m.Name))
	}
	return b.String(), nil
}

// --- internals -------------------------------------------------------------

func (r *Runner) ensureTable(ctx context.Context) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
  version BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL,
  exec_ms INTEGER NOT NULL,
  success BOOLEAN NOT NULL
);`, pq(r.Opts.TableName))
	_, err := r.DB.ExecContext(ctx, q)
	return err
}

type appliedRow struct {
	Version   int64
	Name      string
	Checksum  string
	AppliedAt time.Time
	ExecMS    int
	Success   bool
}

func (r *Runner) loadApplied(ctx context.Context) ([]appliedRow, error) {
	q := fmt.Sprintf(`
SELECT version, name, checksum, applied_at, exec_ms, success
FROM %s
ORDER BY applied_at ASC, version ASC;`, pq(r.Opts.TableName))
	rows, err := r.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []appliedRow
	for rows.Next() {
		var a appliedRow
		if err := rows.Scan(
			&a.Version, &a.Name, &a.Checksum, &a.AppliedAt,
			&a.ExecMS, &a.Success,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// loadAppliedSuccess returns only successful rows.
func (r *Runner) loadAppliedSuccess(
	ctx context.Context,
) ([]appliedRow, error) {
	all, err := r.loadApplied(ctx)
	if err != nil {
		return nil, err
	}
	var ok []appliedRow
	for _, a := range all {
		if a.Success {
			ok = append(ok, a)
		}
	}
	return ok, nil
}

func (r *Runner) pendingUp(applied []appliedRow) []*Migration {
	appliedSet := map[int64]appliedRow{}
	for _, a := range applied {
		if a.Success {
			appliedSet[a.Version] = a
		}
	}
	var pending []*Migration
	for _, m := range r.migrations {
		if m.Dir != "up" {
			continue
		}
		if a, ok := appliedSet[m.Version]; ok {
			if a.Checksum != m.Checksum {
				panic(fmt.Sprintf(
					"checksum mismatch at %d: have %s want %s",
					m.Version, a.Checksum, m.Checksum,
				))
			}
			continue
		}
		pending = append(pending, m)
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})
	return pending
}

func (r *Runner) applyOne(ctx context.Context, m *Migration) error {
	r.log("applying %d %s", m.Version, m.Name)
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	start := time.Now()
	_, err = tx.ExecContext(ctx, m.SQL)
	execMS := int(time.Since(start).Milliseconds())
	if err != nil {
		_ = tx.Rollback()
		_ = r.record(ctx, m, execMS, false)
		return fmt.Errorf("migration %d failed: %w", m.Version, err)
	}
	if err := tx.Commit(); err != nil {
		_ = r.record(ctx, m, execMS, false)
		return err
	}
	return r.record(ctx, m, execMS, true)
}

func (r *Runner) revertOne(ctx context.Context, m *Migration) error {
	r.log("reverting %d %s", m.Version, m.Name)
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	// Run the down SQL first.
	if strings.TrimSpace(m.SQL) != "" {
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("revert %d failed: %w", m.Version, err)
		}
	}
	// Then delete the "up" record in the same transaction.
	q := fmt.Sprintf(`DELETE FROM %s WHERE version = $1;`,
		pq(r.Opts.TableName))
	res, err := tx.ExecContext(ctx, q, m.Version)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	aff, _ := res.RowsAffected()
	if aff != 1 {
		_ = tx.Rollback()
		return fmt.Errorf(
			"revert %d succeeded but no row deleted from %s",
			m.Version, r.Opts.TableName,
		)
	}
	return tx.Commit()
}

func (r *Runner) record(
	ctx context.Context, m *Migration, execMS int, ok bool,
) error {
	q := fmt.Sprintf(`
INSERT INTO %s (version, name, checksum, applied_at, exec_ms, success)
VALUES ($1, $2, $3, NOW(), $4, $5)
ON CONFLICT (version) DO UPDATE SET
  name = EXCLUDED.name,
  checksum = EXCLUDED.checksum,
  applied_at = EXCLUDED.applied_at,
  exec_ms = EXCLUDED.exec_ms,
  success = EXCLUDED.success;`, pq(r.Opts.TableName))
	_, err := r.DB.ExecContext(
		ctx, q, m.Version, m.Name, m.Checksum, execMS, ok,
	)
	return err
}

func (r *Runner) withLock(
	ctx context.Context, fn func(context.Context) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if _, err := r.DB.ExecContext(
		ctx, `SELECT pg_advisory_lock($1);`, r.Opts.LockKey,
	); err != nil {
		return err
	}
	defer func() {
		_, _ = r.DB.ExecContext(
			context.Background(),
			`SELECT pg_advisory_unlock($1);`, r.Opts.LockKey,
		)
	}()
	return fn(ctx)
}

func (r *Runner) loadMigrations() error {
	if r.loaded {
		return nil
	}
	// Build list of roots to read from: explicit dirs, then embedded FS.
	var roots []fs.FS
	var dirs []string
	if r.Opts.MigrationsDir != "" {
		dirs = append(dirs, r.Opts.MigrationsDir)
	}
	if len(r.Opts.MigrationsDirs) > 0 {
		dirs = append(dirs, r.Opts.MigrationsDirs...)
	}
	// Deduplicate dirs while keeping order.
	seen := map[string]struct{}{}
	var uniqDirs []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		uniqDirs = append(uniqDirs, d)
	}
	for _, d := range uniqDirs {
		roots = append(roots, os.DirFS(d))
	}
	// Always include embedded FS (if provided) after explicit dirs.
	// Use "." so callers can embed files at the package root.
	// If a project prefers a subdir, they can still embed with
	// that structure; "." works for both cases when combined
	// with fs.Sub.
	base := "."
	for _, efs := range r.Opts.EmbeddedFSs {
		sub, err := fs.Sub(efs, base)
		if err != nil {
			return fmt.Errorf("embed sub: %w", err)
		}
		roots = append(roots, sub)
	}
	if len(roots) == 0 {
		return errors.New("no migrations source provided")
	}

	// Aggregate files from all roots.
	type fileRef struct {
		root fs.FS
		name string
	}
	var all []fileRef
	for _, root := range roots {
		ents, err := fs.ReadDir(root, ".")
		if err != nil {
			return fmt.Errorf("readdir: %w", err)
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			all = append(all, fileRef{root: root, name: e.Name()})
		}
	}
	// Parse and collect. Detect duplicate (version, dir) pairs.
	seenMig := map[string]struct{}{}
	for _, fr := range all {
		name := fr.name
		m, ok := parseFileName(name)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d:%s", m.Version, m.Dir)
		if _, exists := seenMig[key]; exists {
			return fmt.Errorf("duplicate migration for version %d dir %s",
				m.Version, m.Dir)
		}
		b, err := fs.ReadFile(fr.root, name)
		if err != nil {
			return err
		}
		sql := strings.TrimSpace(string(b))
		sum := checksum(sql)
		m.SQL = sql
		m.Checksum = sum
		r.migrations = append(r.migrations, &m)
		seenMig[key] = struct{}{}
	}
	sort.Slice(r.migrations, func(i, j int) bool {
		if r.migrations[i].Version == r.migrations[j].Version {
			return r.migrations[i].Dir < r.migrations[j].Dir
		}
		return r.migrations[i].Version < r.migrations[j].Version
	})
	r.loaded = true
	return nil
}

func parseFileName(name string) (Migration, bool) {
	m := fileRe.FindStringSubmatch(filepath.Base(name))
	if m == nil {
		return Migration{}, false
	}
	ver, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return Migration{}, false
	}
	return Migration{
		Version: ver,
		Name:    m[2],
		Dir:     m[3],
		File:    name,
	}, true
}

func (r *Runner) find(version int64, dir string) *Migration {
	for _, m := range r.migrations {
		if m.Version == version && m.Dir == dir {
			return m
		}
	}
	return nil
}

func (r *Runner) log(format string, args ...any) {
	if r.Opts.Logger != nil {
		r.Opts.Logger(format, args...)
	}
}

func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:])
}

func pq(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}
