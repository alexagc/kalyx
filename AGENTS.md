# Kalyx: AI Agent Guidelines & Architecture Rules 🤖🌸

> **Universal Agent Instructions:** This document serves as the single source of truth for all AI coding assistants (Google Antigravity, Cursor, Claude Code, GitHub Copilot) and human contributors working on **Kalyx**.

---

## 1. Core Mission & Philosophy
* **What is Kalyx:** An open-source **CloudEvents v1.0.2 Contract & Schema Registry** for microservices and Data Lakes (Apache Iceberg / Delta Lake).
* **Strict English Mandate:** All code, identifiers, comments, documentation, commit messages, issue templates, and internal notes **MUST be written in English**. No exceptions.
* **Architecture Standard:** We follow the **C4 Architectural Model** documented with Mermaid diagrams in [docs/architecture/c4-model.md](file:///docs/architecture/c4-model.md).
* **Zero-Dependency Mandate:** The Kalyx server compiles into a **single, standalone binary** with embedded CGO-free SQLite (`modernc.org/sqlite`) and an embedded web dashboard (`//go:embed`).
* **Never add external runtime dependencies** (e.g. CGO drivers, Node.js runtime in production, external message brokers) without explicit architectural review.

---

## 2. Multi-Agent & Parallel Development Protocol
To allow multiple human developers and autonomous AI agents to collaborate in parallel without merge conflicts or context loss:

1. **Check the Task Board First:**
   * Always inspect `.notes/TASK_BOARD.md` before starting work to see what tasks are *In Progress* or *Claimed*.
   * Mark your task as *In Progress* and update it upon completion.
2. **Hexagonal Architecture & Interfaces First:**
   * Every subsystem (`storage`, `auth`, `compatibility`, `datalake`, `validator`) MUST expose its capabilities through **Go interfaces** defined in `internal/`.
   * Never couple HTTP handlers directly to concrete database structs.
   * Provide pure in-memory fakes/mocks for every interface so other agents/developers can test against your package before the concrete implementation is merged.
3. **Branching & Git Isolation:**
   * Work in short-lived feature branches: `feat/<feature-name>`, `fix/<bug-name>`, `chore/<task>`.
   * Use **Conventional Commits** (`feat:`, `fix:`, `docs:`, `test:`, `chore:`) in English.
   * Merging to `main` is strictly done via **Squash & Merge**.
   * Never push broken code to `main`.
4. **Protect Private Context:**
   * Never commit files under `.notes/`, `.env`, or credential tokens (`kalyx_sec_...`) to git. These are local and gitignored.

---

## 3. Go Backend Standards
* **Toolchain:** Go 1.23+.
* **Logging:** Use native structured logging with `log/slog`. Do **NOT** introduce third-party loggers (like Logrus or Zap).
* **Error Handling:** Use error wrapping with `%w` (`fmt.Errorf("failed to load event: %w", err)`). All API errors must conform to **RFC 7807 (Problem Details for HTTP APIs)**.
* **Database Drivers:**
  * Local/Testing: Pure Go CGO-free SQLite (`modernc.org/sqlite`).
  * Production: Pure Go PostgreSQL (`github.com/jackc/pgx/v5` or `database/sql` driver).
* **Testing:**
  * Follow the [Testing Standards](file:///.agent/rules/testing-standards.md) across all 4 levels (Unit, Integration, Benchmarks, E2E).
  * Run `make test` before completing your turn. All tests must pass with race detector enabled (`-race`).
  * Run `make lint` (`golangci-lint run ./...`).

---

## 4. Frontend & Stitch Design System
* **Strict 'Stitch-First Gate' Rule:** Do **NOT** write arbitrary frontend code in the repository until screen wireframes and flows are defined and approved in **Stitch** following the 4-phase protocol in `.agent/rules/frontend-react.md` and `.notes/STITCH_UI_SPEC.md`.
* **Technology Stack (When Ready):** React + TypeScript + Vite + Tailwind CSS / Shadcn UI + Monaco Editor.
* **Asset Embedding:** The compiled frontend bundle (`dist/`) will be embedded into the Go binary with `//go:embed`.

---

## 5. Detailed Domain Rules
Refer to specialized rules in `.agent/rules/`:
* [Backend Go Rules](file:///.agent/rules/backend-go.md)
* [Frontend & Stitch Rules](file:///.agent/rules/frontend-react.md)
* [Git & Workflow Rules](file:///.agent/rules/git-workflow.md)
* [Testing Standards & Pyramid](file:///.agent/rules/testing-standards.md)
