# RFC-001: Kalyx Architecture & Design Specification 🌸

- **Status:** Draft / In Planning
- **Author:** Alejandro Gómez Canal & Antigravity
- **Date:** October 2026
- **Repository:** [https://github.com/alexagc/kalyx](https://github.com/alexagc/kalyx)

---

## 1. Contexto y Visión

Kalyx nace como una plataforma open-source de **Event Governance & Data Contracts** orientada a resolver la brecha histórica entre los equipos que producen eventos en microservicios y los equipos que los consumen (Data Lake, pipelines analíticos y otros servicios).

### El Problema
1. Los microservicios emiten eventos push hacia buses o lagos de datos sin validación estricta ni acuerdos formales.
2. Los cambios en los esquemas rompen silenciosamente los pipelines de analítica aguas abajo (*Data Lake degradation / Data Swamp*).
3. Las herramientas actuales son:
   - O catálogos pasivos de metadatos *post-mortem* (como DataHub), que no intervienen en el momento en que el desarrollador construye el evento.
   - O registries técnicos hiper-complejos acoplados a Kafka/JVM (como Confluent o Apicurio), que carecen de una interfaz colaborativa amigable y centrada en Data Contracts.

### La Propuesta de Kalyx
- **Estándar CNCF CloudEvents v1.0.2:** Formato neutral para la envoltura de eventos (`id`, `source`, `type`, `time`, `dataschema`, `datacontenttype`).
- **Contratos con JSON Schema 2020-12:** Enriquecidos con anotaciones de Data Lake (`partition-by`, `x-pii`).
- **Filosofía Zero-Dependency:**
  - Binario único compilado en Go.
  - Base de datos embebida (SQLite CGO-free, sin necesidad de Postgres o Docker para arrancar).
  - Frontend web embebido (`//go:embed`).
  - Entorno de desarrollo instantáneo en la nube mediante `.devcontainer` (GitHub Codespaces).
- **Shift-Left en el Microservicio:** CLI (`kalyxctl`) para generar tipos y clases fuertemente tipadas en TypeScript, Python, etc. con constructores CloudEvents y validación local sin latencia de red.
- **Data Lake Autopilot:** Generación de DDL analítico (Apache Iceberg / Delta Lake) y webhooks ante publicaciones.

---

## 2. Decisiones Arquitectónicas Fundamentales

| Decisión | Opción Seleccionada | Justificación |
| :--- | :--- | :--- |
| **Rol Operativo** | Hub de Contratos & CodeGen (Fuera de la ruta crítica de datos) | Evita cuellos de botella de red y caídas de servicio. La validación se hace en origen mediante el SDK/tipos compilados. |
| **Estándar de Esquemas** | JSON Schema (Draft 2020-12) enriquecido | Máxima adopción en microservicios y compatibilidad con CloudEvents JSON, extendido con metadatos para tablas analíticas. |
| **Consumo en Microservicios** | CLI / Generación de Código (`kalyxctl`) | Proporciona autocompletado en el IDE, validación en tiempo de compilación y cero llamadas HTTP síncronas en cada evento emitido. |
| **Flujo de Gobernanza** | Revisión nativa en UI (*Draft ➔ In Review ➔ Approved*) | Colaboración directa entre desarrolladores y Data Engineers con detección automática de SemVer / Breaking Changes. |
| **Data Lake Bridge** | Generación de DDL (Iceberg/Delta) + Webhooks | Los ingenieros de datos reciben la definición exacta de tabla sin tener que traducir JSON a SQL manualmente. |
| **Stack Tecnológico** | Go (Backend + CLI) + React/Next.js (Web) + SQLite | Binario único, ultra-rápido, bajo consumo de memoria y despliegue trivial. |

---

## 3. Próximos Pasos de Planificación Detallada

1. **Taxonomía y Especificación de Contratos:** Definir la estructura exacta de nombres de eventos, versiones y extensiones `x-datalake`.
2. **Wireframes y Diseño de Pantallas UI:** Especificar campos y vistas para Catálogo, Editor Monaco, Diff Viewer y Aprobaciones.
3. **Contrato de la API REST:** Especificación OpenAPI de los endpoints del Core Server.
4. **Modelo Relacional de Base de Datos:** Esquema de tablas SQLite (eventos, versiones, esquemas, propuestas, auditoría).
