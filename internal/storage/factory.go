package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// DriverFunc is a constructor function for a storage driver.
type DriverFunc func(ctx context.Context, dsn string) (Storage, error)

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]DriverFunc)
)

// RegisterDriver registers a storage driver constructor.
func RegisterDriver(name string, fn DriverFunc) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = fn
}

// NewFromEnv initializes a Storage implementation based on environment variables:
// - If DATABASE_URL is "memory", it returns a thread-safe MemoryStorage instance.
// - If DATABASE_URL starts with "postgres://" or "postgresql://", it initializes the PostgreSQL adapter.
// - By default, it initializes the pure-Go CGO-free SQLite adapter (Zero-Dependency).
func NewFromEnv(ctx context.Context) (Storage, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "memory" {
		return NewMemoryStorage(), nil
	}

	driversMu.RLock()
	defer driversMu.RUnlock()

	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		fn, ok := drivers["postgres"]
		if !ok {
			return nil, fmt.Errorf("postgres driver not registered")
		}
		return fn(ctx, dbURL)
	}

	sqlitePath := os.Getenv("KALYX_SQLITE_PATH")
	if sqlitePath == "" {
		if dbURL != "" && !strings.Contains(dbURL, "://") {
			sqlitePath = dbURL
		} else {
			sqlitePath = "kalyx.db"
		}
	}

	fn, ok := drivers["sqlite"]
	if !ok {
		return nil, fmt.Errorf("sqlite driver not registered")
	}
	return fn(ctx, sqlitePath)
}
