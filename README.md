# Kalyx 🌸

[![CI](https://github.com/alexagc/kalyx/actions/workflows/ci.yml/badge.svg)](https://github.com/alexagc/kalyx/actions/workflows/ci.yml)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![CloudEvents](https://img.shields.io/badge/Standard-CloudEvents_v1.0.2-blueviolet.svg)](https://cloudevents.io/)

> **Open-Source CloudEvents Contract & Schema Registry for Microservices and Data Lakes.**

Kalyx protects your data pipelines by shifting data contracts left. It enables software teams to collaboratively design, review, version, and generate typed SDKs for CloudEvents, while automatically keeping downstream Data Lake tables (Apache Iceberg, Delta Lake) in sync.

---

## ✨ Key Features

- **⚡ 100% Self-Contained & Zero External Dependencies:**
  - Single standalone binary (Windows, Linux, macOS).
  - Embedded pure-Go SQLite database (`modernc.org/sqlite`, CGO-free).
  - Embedded web UI inside the executable (`//go:embed`). No PostgreSQL, Node.js, or Docker required to run!
- **🌐 CloudEvents Native (CNCF):**
  - First-class support for the CloudEvents v1.0.2 specification (`id`, `source`, `type`, `time`, `datacontenttype`, `dataschema`).
- **🛡️ Data Contracts & Schema Evolution:**
  - JSON Schema Draft 2020-12 with Data Lake extensions (`x-datalake.partition-by`, `x-pii`).
  - Automated SemVer compatibility checker (detects breaking changes before publishing).
- **💻 Developer-First CLI (`kalyxctl`):**
  - Generate strongly-typed models (TypeScript, Python Pydantic, Go structs) with built-in CloudEvent builders.
  - Zero network overhead during event emission (compile-time validation).
- **🧊 Data Lake Ingestion Autopilot:**
  - Auto-generate DDL migration scripts for **Apache Iceberg**, **Delta Lake**, and relational SQL tables.
  - Webhook dispatching to notify data orchestrators (Airflow, Dagster, Kafka Connect).
- **👥 Collaborative Governance UI:**
  - Visual catalog, Monaco editor for JSON Schema, side-by-side diff viewer, and approval workflows (*Draft* ➔ *In Review* ➔ *Approved*).

---

## 🏗️ Architecture

```
                      ┌────────────────────────────────────────────────────────┐
                      │              Web Portal (React / Next.js)              │
                      │  • Catalog & domain search                             │
                      │  • Monaco Editor (JSON Schema 2020-12)                 │
                      │  • Side-by-side Diff Viewer & SemVer breaking checks   │
                      │  • Review & Approval Workflow (Draft ➔ Approved)       │
                      └───────────────────────────┬────────────────────────────┘
                                                  │ REST / gRPC
                                                  ▼
                      ┌────────────────────────────────────────────────────────┐
                      │             Kalyx Core Server (Go Binary)              │
                      │  • Compatibility Engine (SemVer & Breaking Changes)    │
                      │  • Data Lake DDL Generator (Iceberg, Delta Lake)       │
                      │  • Webhooks dispatcher                                 │
                      │  • Storage: Embedded SQLite (or external PostgreSQL)   │
                      └─────────────┬────────────────────────────┬─────────────┘
                                    │                            │
                     CLI / CodeGen  │                            │ Webhooks / DDL
                                    ▼                            ▼
   ┌──────────────────────────────────────────────────┐   ┌────────────────────────────────┐
   │         `kalyxctl` (Developer CLI)               │   │           Data Lake            │
   │  • `kalyxctl gen --event order.created --lang ts`│   │  • Auto-evolve Iceberg/Delta   │
   │  • Strongly-typed classes & CloudEvent builders  │   │    table schemas via DDL       │
   │  • Local validation (zero runtime latency)       │   │  • Webhook triggers pipelines  │
   └──────────────────────────────────────────────────┘   └────────────────────────────────┘
```

---

## 🚀 Quickstart

### 1. Download & Run (Standalone Binary)
```bash
# Run Kalyx (starts web UI + API + embedded database on port 8080)
./kalyx
```
Visit **`http://localhost:8080`** in your browser.

### 2. Generate Client Code in Microservices
```bash
# Generate TypeScript types & CloudEvent builder
kalyxctl generate --event com.company.orders.completed --version 1.0.0 --lang typescript
```

---

## 📂 Repository Structure

```
kalyx/
├── cmd/
│   ├── kalyx/            # Core Server binary (API + Embedded Web UI)
│   └── kalyxctl/         # Developer CLI binary (CodeGen & Validation)
├── internal/
│   ├── core/             # Domain models (Events, Versions, Contracts)
│   ├── compatibility/    # SemVer compatibility engine
│   ├── datalake/         # Analytical DDL generators (Iceberg, Delta Lake)
│   ├── storage/          # Pure-Go SQLite embedded storage
│   └── webserver/        # HTTP server with embedded frontend assets
├── web/                  # Next.js web application
└── .devcontainer/        # 1-click cloud development via GitHub Codespaces
```

---

## 📄 License

Licensed under the [Apache License, Version 2.0](LICENSE).
