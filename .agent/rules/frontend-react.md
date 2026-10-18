# Frontend & Stitch Rules for Kalyx 🌸

## 1. 'Stitch-First Gate' 4-Phase Protocol
No developer or AI agent is authorized to write frontend code in the repository without strictly adhering to this protocol:

1. **Phase 1: Functional Specification:**
   * The view must be specified in detail in `.notes/STITCH_UI_SPEC.md` (fields, actions, error states, and navigation).
2. **Phase 2: Prototyping & Validation in Stitch:**
   * The screen must be designed and visually approved in Stitch until it reaches the `Approved` state.
3. **Phase 3: Technical Handoff (`.notes/stitch/`):**
   * Freeze the design reference into a handoff document at `.notes/stitch/<screen_name>.md` detailing:
     * Component hierarchy (e.g., `CatalogHeader` ➔ `FilterBar` ➔ `EventTable` ➔ `EventRow`).
     * Tailwind CSS design token mapping.
     * Interactive states (hover, active, loading, empty state, RFC 7807 error banner).
4. **Phase 4: Implementation in `web/`:**
   * Only with the technical handoff completed, implement the React + TypeScript code in `web/`, ensuring 1:1 visual fidelity against the approved Stitch screen.

---

## 2. Technology Stack
* **Framework:** React 18+ with TypeScript and Vite.
* **Styling:** Tailwind CSS with Shadcn UI (Radix UI) accessible primitives.
* **Schema Editor & Diff:** `@monaco-editor/react` for JSON Schema Draft 2020-12 viewing, editing, and side-by-side visual diffing with syntax highlighting.
* **Icons:** Lucide React (`lucide-react`).

---

## 3. Packaging & Go Binary Embedding Protocol
* Production build output must be placed in `internal/webserver/dist/`.
* Go embeds static assets at compile time using:
  ```go
  //go:embed dist/*
  var embeddedUI embed.FS
  ```
* The Go HTTP server must fallback to `index.html` for all client routes (SPA HTML5 History API).
