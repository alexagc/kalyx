# Kalyx: Testing Standards & Quality Pyramid 🧪🌸

This testing standard governs all human contributors and AI coding assistants writing code for **Kalyx**.

---

## 1. Testing Pyramid

```
                  / \
                 /   \
                / E2E \       CLI kalyxctl end-to-end against local server + Playwright UI
               /-------\
              / Bench   \     Latency < 5ms for compatibility and parsing (go test -bench)
             /-----------\
            / Integration \   HTTP API with httptest and real storage (SQLite / Postgres)
           /---------------\
          /   Unit Tests    \ Pure Go (*_test.go), in-memory mocks, race detector (-race), coverage >= 80%
         /-------------------\
```

---

## 2. Test Levels and Conventions

### 2.1. Unit Tests (`unit`)
* **Location:** Same package as code under test (`foo_test.go` adjacent to `foo.go`).
* **Rules:**
  * Must be ultra-fast and deterministic.
  * Use in-memory mocks/fakes from `internal/storage` or `internal/auth`.
  * Never depend on network or persistent filesystem where avoidable.
  * Must run cleanly with `-race`.
* **Command:**
  ```bash
  make test
  ```

### 2.2. Integration Tests (`integration`)
* **Location:** `tests/integration/` or package-level integration suites.
* **Rules:**
  * Use `net/http/httptest.NewServer` to test real HTTP requests against handlers.
  * Test real persistence using SQLite in-memory or temporary files.
  * Verify HTTP status codes and error payloads conform to **RFC 7807**.
* **Command:**
  ```bash
  make test-integration
  ```

### 2.3. Performance & Benchmark Tests (`bench`)
* **Location:** `Benchmark*` functions inside performance-critical packages (`compatibility`, `datalake`, `core`).
* **Latency Budgets:**
  * `CheckCompatibility`: **< 5ms** for schemas with up to 500 properties.
  * `GenerateIcebergDDL`: **< 2ms** per table.
  * JSON Schema compile & cache: **< 10ms**.
* **Command:**
  ```bash
  make bench
  ```

### 2.4. End-to-End Tests (`e2e`)
* **Location:** `tests/e2e/`.
* **Scope:**
  1. **CLI E2E:** Launch `kalyx-server`, authenticate `kalyxctl` via PAT, publish a contract, verify SemVer diagnostics, and generate TypeScript/Python SDKs.
  2. **UI E2E (Phase 2):** Playwright tests covering critical SPA journeys (Catalog ➔ Detail ➔ Diff ➔ Approval).
* **Command:**
  ```bash
  make test-e2e
  ```

---

## 3. Code Coverage
* Minimum coverage target for core business logic (`compatibility`, `core`, `datalake`) is **>= 80%**.
* Visual HTML report generation:
  ```bash
  make test-coverage
  ```
