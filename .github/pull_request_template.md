## Description
Briefly describe the purpose of this PR and what changes or features it introduces to Kalyx.

## Type of Change
- [ ] Backend Go enhancement or fix (`internal/`, `cmd/`)
- [ ] CLI `kalyxctl` command or SDK codegen update
- [ ] Web Dashboard feature (`internal/webserver/dist/`, `web/`)
- [ ] Storage migration or schema evolution (`internal/storage/migrations/`)
- [ ] CI/CD pipeline or release automation improvement
- [ ] Documentation update

## Architectural & Quality Checklist
- [ ] **Strict English Mandate:** All code, comments, identifiers, and commit messages are in English.
- [ ] **Zero-Dependency Mandate:** No runtime CGO dependencies introduced; compiles cleanly with `CGO_ENABLED=0`.
- [ ] **Hexagonal Architecture:** Subsystems expose capabilities through Go interfaces defined in `internal/` with pure in-memory fakes.
- [ ] **Error Standards:** API errors conform to RFC 7807 (`application/problem+json`).
- [ ] **Testing Pyramid:**
  - [ ] Unit tests added/updated and passing cleanly (`go test -race ./internal/...`).
  - [ ] Integration tests pass (`go test ./tests/integration/...`).
  - [ ] CLI E2E tests pass (`go test ./tests/e2e/...`).
- [ ] **Linters & Security:** Code passes `golangci-lint run ./...` and `govulncheck ./...`.
