package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed sqlite/*.sql
var SQLiteMigrationsFS embed.FS

//go:embed postgres/*.sql
var PostgresMigrationsFS embed.FS

// Migration represents a parsed SQL migration file.
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Filename string
}

// RunSQLiteMigrations executes pending migrations embedded in SQLiteMigrationsFS.
func RunSQLiteMigrations(ctx context.Context, db *sql.DB) error {
	return runMigrations(ctx, db, SQLiteMigrationsFS, "sqlite", "sqlite")
}

// RunPostgresMigrations executes pending migrations embedded in PostgresMigrationsFS.
func RunPostgresMigrations(ctx context.Context, db *sql.DB) error {
	return runMigrations(ctx, db, PostgresMigrationsFS, "postgres", "postgres")
}

func runMigrations(ctx context.Context, db *sql.DB, fsys embed.FS, dir, dialect string) error {
	// 1. Ensure schema_migrations table exists
	ensureTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL
	);`
	if _, err := db.ExecContext(ctx, ensureTableSQL); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table: %w", err)
	}

	// 2. Query applied migrations
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var ver int
		if err := rows.Scan(&ver); err != nil {
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[ver] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error reading migration rows: %w", err)
	}

	// 3. Read embedded files
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migration directory %s: %w", dir, err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		filename := entry.Name()
		parts := strings.SplitN(filename, "_", 2)
		if len(parts) < 2 {
			continue
		}
		ver, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		content, err := fsys.ReadFile(path.Join(dir, filename))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		migrations = append(migrations, Migration{
			Version:  ver,
			Name:     filename,
			SQL:      string(content),
			Filename: filename,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// 4. Apply pending migrations sequentially inside a transaction
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		slog.Info("applying database migration", "version", m.Version, "name", m.Filename, "dialect", dialect)

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin migration transaction for %s: %w", m.Filename, err)
		}

		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", m.Filename, err)
		}

		insertSQL := "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)"
		if dialect == "postgres" {
			insertSQL = "INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)"
		}
		if _, err := tx.ExecContext(ctx, insertSQL, m.Version, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", m.Filename, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", m.Filename, err)
		}

		slog.Info("✔ migration applied successfully", "version", m.Version)
	}

	return nil
}
