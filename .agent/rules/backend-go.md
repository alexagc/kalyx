# Backend Go Development Rules for Kalyx 🌸

## 1. Zero-Dependency & Pure Go Mandate
* Kalyx must compile cleanly with `CGO_ENABLED=0`.
* SQLite database access must use `modernc.org/sqlite` (pure Go), never `mattn/go-sqlite3`.
* PostgreSQL driver must use pure Go (`github.com/jackc/pgx/v5` stdlib adapter).

## 2. Architecture & Interfaces
* Subsystems are decoupled using Go interfaces defined in `internal/`:
  * `storage.Storage`: persistence operations (get, list, create, approve).
  * `compatibility.Checker`: SemVer compatibility and breaking change detection.
  * `datalake.Generator`: Iceberg and Delta Lake DDL generation.
  * `auth.Authenticator`: OIDC and PAT validation.
  * `validator.Validator`: CloudEvents JSON Schema validation.
* Always provide an in-memory fake/mock implementation (`MemoryStorage`, etc.) alongside persistent drivers to enable fast parallel unit testing.

## 3. Structured Logging & Error Handling
* Use `log/slog` for all logs. Example:
  ```go
  slog.Info("contract approved", "event_id", id, "semver", ver, "approved_by", user)
  ```
* Format all API error responses following RFC 7807 (`application/problem+json`):
  * `type` (URI identifier)
  * `title` (short summary)
  * `status` (HTTP status code)
  * `detail` (human-readable explanation)
  * `invalid_params` (array of validation failures)

## 4. Testing & Verification
* Unit tests: `make test` (`go test -v -race ./internal/...`).
* Integration tests: `make test-integration` (`go test -v -race ./tests/integration/...`).
* Benchmarks: `make bench` (`go test -v -bench=. -benchmem -run=^# ./internal/...`).
* Linters: `make lint` (`golangci-lint run ./...`).
* Vulnerabilities: `make vulncheck` (`govulncheck ./...`).
