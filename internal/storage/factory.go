package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alexagc/kalyx/internal/storage/postgres"
	"github.com/alexagc/kalyx/internal/storage/sqlite"
)

// NewFromEnv initializes a Storage implementation based on environment variables:
// - If DATABASE_URL is "memory", it returns a thread-safe MemoryStorage instance.
// - If DATABASE_URL starts with "postgres://" or "postgresql://", it initializes the PostgreSQL adapter.
// - By default, it initializes the pure-Go CGO-free SQLite adapter (Zero-Dependency).
func NewFromEnv(ctx context.Context) (Storage, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "memory" {
		return NewMemoryStorage(), nil
	}

	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		return postgres.New(ctx, dbURL)
	}

	sqlitePath := os.Getenv("KALYX_SQLITE_PATH")
	if sqlitePath == "" {
		if dbURL != "" && !strings.Contains(dbURL, "://") {
			sqlitePath = dbURL
		} else {
			sqlitePath = "kalyx.db"
		}
	}

	return sqlite.New(ctx, sqlitePath)
}
