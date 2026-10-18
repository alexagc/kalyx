# Kalyx: C4 Architectural Model 🏛️🌸

This document defines the technical architecture of **Kalyx** following the **C4 standard (Context, Containers, Components, Code)** using **Mermaid** diagrams.

---

## Level 1: System Context Diagram
Shows users and external systems interacting with Kalyx across the organization.

```mermaid
C4Context
    title Level 1: System Context for Kalyx

    Person(dev, "Microservice Developer", "Emits CloudEvents in backend services (Go, TS, Python)")
    Person(steward, "Data Steward / Tech Lead", "Governs data quality and approves breaking schema changes")
    
    System(kalyx, "Kalyx Platform", "Out-of-band CloudEvents Contract Hub & Governance Registry")
    
    System_Ext(idp, "Identity Provider (OIDC)", "GitHub, Okta, Google Workspace (authentication and claim groups)")
    System_Ext(lake, "Data Lake / Lakehouse", "Apache Iceberg / Delta Lake storing events in Parquet format")
    System_Ext(orchestrator, "Data Orchestrator", "Airflow, Dagster, Kafka Connect consuming schema webhooks")
    System_Ext(gateway, "API Gateway / Edge Ingress", "Kong, Envoy validating CloudEvents at runtime (/api/v1/validate)")

    Rel(dev, kalyx, "Queries contracts and generates typed SDKs", "kalyxctl CLI / Web UI")
    Rel(steward, kalyx, "Reviews and approves contracts and breaking changes", "Web UI (HTTPS)")
    Rel(gateway, kalyx, "Validates events at runtime", "POST /api/v1/validate")
    Rel(kalyx, idp, "Validates identity and user groups", "OIDC / OAuth2")
    Rel(kalyx, lake, "Generates DDL and table schema migrations", "SQL / DDL")
    Rel(kalyx, orchestrator, "Notifies approved contracts", "CloudEvents Webhooks")
```

---

## Level 2: Container Diagram
Shows high-level deployable units and physical architecture of Kalyx.

```mermaid
C4Container
    title Level 2: Containers for Kalyx

    Person(dev, "Developer", "CLI User")
    Person(steward, "Data Steward", "UI User")

    Container_Boundary(c1, "Kalyx Ecosystem") {
        Container(cli, "kalyxctl (CLI)", "Go Binary", "SDK code generator and local CI/CD contract validator")
        Container(server, "kalyx-server", "Go Binary", "HTTP REST API server, compatibility engine, runtime validator, and DDL generator")
        Container(ui, "Embedded Web Dashboard", "React + Vite + Tailwind (//go:embed)", "Visual catalog, Monaco editor, and approval workflows")
        ContainerDb(db_local, "Local Database", "SQLite CGO-free (modernc.org)", "Default embedded storage (Zero-Dependency)")
        ContainerDb(db_prod, "Production Database", "PostgreSQL (pgx)", "High-availability persistent storage (optional)")
    }

    Rel(dev, cli, "Runs kalyxctl generate / lint", "Terminal")
    Rel(cli, server, "Fetches contracts and validates in CI", "REST API (RFC 7807 / Bearer PAT)")
    Rel(steward, ui, "Browses and approves", "Browser (HTTPS)")
    Rel(ui, server, "Catalog and proposal operations", "REST API (/api/v1)")
    Rel(server, ui, "Serves embedded static web assets", "HTTP //go:embed")
    Rel(server, db_local, "Reads/Writes in local mode", "SQL Driver")
    Rel(server, db_prod, "Reads/Writes in production mode", "pgx Driver")
```

---

## Level 3: Component Diagram (Server Core)
Shows the internal decoupled package architecture inside the `kalyx-server` binary.

```mermaid
C4Component
    title Level 3: Internal Components of kalyx-server

    Container_Boundary(api_boundary, "kalyx-server Core") {
        Component(router, "HTTP Router & Handlers", "net/http ServeMux", "Routes REST endpoints, K8s probes (/livez, /readyz), and /metrics")
        Component(auth_mw, "Auth & RBAC Middleware", "internal/auth", "Validates OIDC JWTs, verifies PATs, and evaluates Claim Groups")
        Component(validator, "Runtime Validator", "internal/validator", "Validates incoming CloudEvents against JSON Schemas (POST /api/v1/validate)")
        Component(compat_engine, "Compatibility Engine", "internal/compatibility", "SemVer diagnosis (Major/Minor/Patch) between JSON Schemas")
        Component(datalake_gen, "Data Lake DDL Generator", "internal/datalake", "Translates JSON Schema into DDL for Apache Iceberg and Delta Lake")
        Component(webhook_disp, "Webhook Dispatcher", "internal/webhooks", "Dispatches CloudEvents org.kalyx.contract.approved.v1 with backoff")
        Component(storage_iface, "Storage Interface", "internal/storage", "Abstract persistence interface (Get, List, Create, Approve)")
        Component(sqlite_impl, "SQLite Adapter", "internal/storage/sqlite", "CGO-free implementation with embedded SQL migrations")
        Component(pg_impl, "PostgreSQL Adapter", "internal/storage/postgres", "Production implementation using pgx")
    }

    Rel(router, auth_mw, "Protects API endpoints")
    Rel(router, validator, "Validation requests /api/v1/validate")
    Rel(router, compat_engine, "Requests compatibility checks")
    Rel(router, datalake_gen, "Generates DDL for Data Lake tables")
    Rel(router, webhook_disp, "Triggers webhooks upon contract approval")
    Rel(router, storage_iface, "Invokes data operations")
    Rel(validator, storage_iface, "Retrieves active schema")
    Rel(storage_iface, sqlite_impl, "SQLite execution")
    Rel(storage_iface, pg_impl, "PostgreSQL execution")
```
