# Git & Collaboration Workflow Rules for Kalyx 🌸

## 1. Branching & Commit Discipline
* Branch naming:
  * `feat/<short-description>`: for new features.
  * `fix/<short-description>`: for bug fixes.
  * `chore/<short-description>`: for tooling, dependencies, or maintenance.
* Conventional Commits (Must be in English):
  * Format: `<type>(<scope>): <subject>`
  * Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`.
  * Example: `feat(storage): implement CGO-free sqlite adapter with embedded migrations`

## 2. Parallel Coordination
* Before starting any work, inspect `.notes/TASK_BOARD.md`.
* Move your assigned task to `In Progress` along with your identifier/branch.
* When finished and all tests pass (`make test`), mark it as `Done`.

## 3. Pull Requests & Merging
* All Pull Requests must target `main`.
* Ensure CI passes cleanly (backend tests, lint, vulncheck).
* Merging to `main` is strictly done via **Squash & Merge** to maintain a clean linear commit history for GoReleaser.
