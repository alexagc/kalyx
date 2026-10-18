import {
  EventDefinition,
  EventVersion,
  FieldSpec,
  Proposal,
  PATToken,
  TeamMapping,
} from './types';

export const MOCK_EVENTS: EventDefinition[] = [
  {
    id: 'orders.order.completed',
    name: 'OrderCompleted',
    type: 'orders.order.completed.v1',
    domain: 'Orders',
    owner_team: 'Checkout Core',
    created_at: '2024-11-04T10:00:00Z',
    updated_at: '2025-05-18T14:30:00Z',
    active_version: 'v1.2.0',
    status: 'approved',
    pii_fields_count: 1,
    has_pii: true,
    synced_tables: ['Iceberg', 'Delta Lake'],
    description: 'Emitted when a customer order completes payment authorization and inventory capture.',
    fingerprint: 'sha256:7f49a2c',
  },
  {
    id: 'payments.payment.processed',
    name: 'PaymentProcessed',
    type: 'payments.payment.processed.v1',
    domain: 'Payments',
    owner_team: 'Billing Team',
    created_at: '2024-09-12T08:00:00Z',
    updated_at: '2025-04-10T11:20:00Z',
    active_version: 'v2.0.0',
    status: 'approved',
    pii_fields_count: 2,
    has_pii: true,
    synced_tables: ['Iceberg & Delta v3'],
    description: 'Emitted when Stripe/Adyen webhook acknowledges settled credit capture.',
    fingerprint: 'sha256:3b91fa8',
  },
  {
    id: 'orders.cart.abandoned',
    name: 'CartAbandoned',
    type: 'orders.cart.abandoned.v1',
    domain: 'Orders',
    owner_team: 'Growth & Funnel',
    created_at: '2025-06-01T15:00:00Z',
    updated_at: '2025-06-15T09:45:00Z',
    active_version: 'v0.9.0',
    status: 'in_review',
    pii_fields_count: 1,
    has_pii: true,
    synced_tables: ['Iceberg'],
    description: 'Triggered when a basket remains inactive for 45 minutes.',
    fingerprint: 'sha256:e82110c',
  },
  {
    id: 'users.user.registered',
    name: 'UserRegistered',
    type: 'users.user.registered.v1',
    domain: 'Users',
    owner_team: 'Identity IAM',
    created_at: '2024-08-01T12:00:00Z',
    updated_at: '2025-03-22T17:10:00Z',
    active_version: 'v1.4.0',
    status: 'approved',
    pii_fields_count: 3,
    has_pii: true,
    synced_tables: ['Iceberg', 'Delta Lake'],
    description: 'Emitted upon successful OIDC onboarding or passwordless verification.',
    fingerprint: 'sha256:1a84c9e',
  },
  {
    id: 'inventory.item.reserved',
    name: 'InventoryReserved',
    type: 'inventory.item.reserved.v1',
    domain: 'Inventory',
    owner_team: 'Supply Chain',
    created_at: '2024-10-15T09:00:00Z',
    updated_at: '2025-02-18T13:40:00Z',
    active_version: 'v1.0.0',
    status: 'approved',
    pii_fields_count: 0,
    has_pii: false,
    synced_tables: ['Iceberg'],
    description: 'Reserves SKU stock in local warehouse buffer prior to pack-and-ship.',
    fingerprint: 'sha256:9f40ab1',
  },
  {
    id: 'orders.shipment.dispatched',
    name: 'ShipmentDispatched',
    type: 'orders.shipment.dispatched.v1',
    domain: 'Orders',
    owner_team: 'Logistics Core',
    created_at: '2025-01-10T14:00:00Z',
    updated_at: '2025-05-02T16:25:00Z',
    active_version: 'v1.1.0',
    status: 'approved',
    pii_fields_count: 1,
    has_pii: true,
    synced_tables: ['Iceberg', 'Delta Lake'],
    description: 'Carrier barcode scanned on dock loading door; tracking number assigned.',
    fingerprint: 'sha256:4d701ee',
  },
];

export const MOCK_ORDER_COMPLETED_SCHEMA_V120 = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/ev_orders_completed/v1.2.0.json",
  "title": "OrderCompleted",
  "type": "object",
  "properties": {
    "order_id": {
      "type": "string",
      "format": "uuid",
      "description": "Primary transaction reference. Partitioned across Kafka & Iceberg lake."
    },
    "customer_id": {
      "type": "string",
      "format": "uuid",
      "description": "Foreign identifier linking to User Identity store."
    },
    "customer_email": {
      "type": "string",
      "format": "email",
      "description": "Customer notification email (Confidential PII, AES-256 masked in Data Lake)."
    },
    "amount_cents": {
      "type": "integer",
      "minimum": 0,
      "description": "Gross completed transaction amount in minor currency unit."
    },
    "currency": {
      "type": "string",
      "enum": ["USD", "EUR", "GBP"],
      "description": "ISO-4217 standard currency code."
    },
    "completed_at": {
      "type": "string",
      "format": "date-time",
      "description": "ISO-8601 UTC timestamp of payment settlement."
    }
  },
  "required": [
    "order_id",
    "customer_id",
    "customer_email",
    "amount_cents",
    "currency",
    "completed_at"
  ],
  "additionalProperties": false
}`;

export const MOCK_ORDER_COMPLETED_PROPOSED_SCHEMA = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/ev_orders_completed/v1.3.0.json",
  "title": "OrderCompleted",
  "type": "object",
  "properties": {
    "order_id": {
      "type": "string",
      "format": "uuid",
      "description": "Primary transaction reference. Partitioned across Kafka & Iceberg lake."
    },
    "customer_id": {
      "type": "string",
      "format": "uuid",
      "description": "Foreign identifier linking to User Identity store."
    },
    "customer_email": {
      "type": "string",
      "format": "email",
      "description": "Customer notification email (Confidential PII, AES-256 masked in Data Lake)."
    },
    "loyalty_tier": {
      "type": "string",
      "enum": ["bronze", "silver", "gold", "platinum"],
      "description": "Customer tier at completion time"
    },
    "amount_cents": {
      "type": "integer",
      "minimum": 0,
      "description": "Gross completed transaction amount in minor currency unit."
    },
    "notes": {
      "type": "string",
      "maxLength": 500,
      "description": "Optional customer order fulfillment notes"
    },
    "currency": {
      "type": "string",
      "enum": ["USD", "EUR", "GBP"],
      "description": "ISO-4217 standard currency code."
    },
    "completed_at": {
      "type": "string",
      "format": "date-time",
      "description": "ISO-8601 UTC timestamp of payment settlement."
    }
  },
  "required": [
    "order_id",
    "customer_id",
    "customer_email",
    "amount_cents",
    "currency",
    "completed_at"
  ],
  "additionalProperties": false
}`;

export const MOCK_ORDER_COMPLETED_BREAKING_SCHEMA = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/ev_orders_completed/v2.0.0.json",
  "title": "OrderCompleted",
  "type": "object",
  "properties": {
    "order_id": {
      "type": "string",
      "format": "uuid"
    },
    "amount_cents": {
      "type": "integer",
      "minimum": 0
    },
    "currency": {
      "type": "string",
      "enum": ["USD", "EUR"]
    },
    "completed_at": {
      "type": "string",
      "format": "date-time"
    }
  },
  "required": [
    "order_id",
    "amount_cents",
    "completed_at"
  ],
  "additionalProperties": false
}`;

export const MOCK_ORDER_VERSIONS: EventVersion[] = [
  {
    id: 'v-120',
    event_id: 'orders.order.completed',
    semver: '1.2.0',
    status: 'approved',
    json_schema: MOCK_ORDER_COMPLETED_SCHEMA_V120,
    data_lake_metadata: {
      target_table: 'lake_orders_completed',
      partition_by: ['order_id'],
      primary_key: ['order_id'],
      format: 'iceberg',
    },
    created_at: '2025-05-18T10:00:00Z',
    approved_at: '2025-05-18T14:30:00Z',
    author: 'Alex Rivera (Checkout Core)',
    approver: 'Sarah Chen (Lead Platform Architect)',
    changelog: 'Added strict currency enum and completed_at date-time format validator.',
  },
  {
    id: 'v-110',
    event_id: 'orders.order.completed',
    semver: '1.1.0',
    status: 'approved',
    json_schema: MOCK_ORDER_COMPLETED_SCHEMA_V120,
    data_lake_metadata: {
      target_table: 'lake_orders_completed',
      partition_by: ['order_id'],
      primary_key: ['order_id'],
      format: 'iceberg',
    },
    created_at: '2025-02-10T09:15:00Z',
    approved_at: '2025-02-11T16:00:00Z',
    author: 'Alex Rivera (Checkout Core)',
    approver: 'Marcus Vance (Data Steward)',
    changelog: 'Added customer_email field with PII classification tag.',
  },
  {
    id: 'v-100',
    event_id: 'orders.order.completed',
    semver: '1.0.0',
    status: 'deprecated',
    json_schema: MOCK_ORDER_COMPLETED_SCHEMA_V120,
    data_lake_metadata: {
      target_table: 'lake_orders_completed_legacy',
      partition_by: ['order_id'],
      primary_key: ['order_id'],
      format: 'iceberg',
    },
    created_at: '2024-11-04T08:00:00Z',
    approved_at: '2024-11-05T12:00:00Z',
    author: 'Alex Rivera (Checkout Core)',
    approver: 'Sarah Chen (Lead Platform Architect)',
    changelog: 'Initial contract specification RFC 001. Sunsetting 2026-12-31.',
  },
];

export const MOCK_FIELD_SPECS: FieldSpec[] = [
  {
    path: 'id',
    name: 'id',
    type: 'string<uuid>',
    required: true,
    isPii: false,
    piiLevel: 'None / System',
    lakeColumn: 'event_id STRING NOT NULL',
    partitionKey: 'Dedup Key',
    description: 'Canonical CloudEvent UUID identifier.',
  },
  {
    path: 'data.order_id',
    name: 'order_id',
    type: 'string<uuid>',
    required: true,
    isPii: false,
    piiLevel: 'None / Public',
    lakeColumn: 'order_id STRING NOT NULL',
    partitionKey: 'bucket(16, order_id)',
    description: 'Primary transaction reference. Partitioned across Kafka & Iceberg lake.',
    constraints: '^ord_[0-9a-z]+$',
  },
  {
    path: 'data.customer_id',
    name: 'customer_id',
    type: 'string<uuid>',
    required: true,
    isPii: false,
    piiLevel: 'Pseudonymous',
    lakeColumn: 'customer_id STRING',
    partitionKey: '—',
    description: 'Foreign identifier linking to User Identity store.',
  },
  {
    path: 'data.customer_email',
    name: 'customer_email',
    type: 'string<email>',
    required: true,
    isPii: true,
    piiLevel: 'Confidential PII (AES-256)',
    lakeColumn: 'customer_email STRING MASKED',
    partitionKey: '—',
    description: 'Direct communication email. Masked in BI and analytics queries.',
  },
  {
    path: 'data.amount_cents',
    name: 'amount_cents',
    type: 'integer',
    required: true,
    isPii: false,
    piiLevel: 'Financial / Internal',
    lakeColumn: 'amount_cents BIGINT NOT NULL',
    partitionKey: '—',
    description: 'Gross completed transaction amount in minor currency unit (e.g. cents).',
    constraints: 'minimum: 0',
  },
  {
    path: 'data.currency',
    name: 'currency',
    type: 'string<enum>',
    required: true,
    isPii: false,
    piiLevel: 'Public',
    lakeColumn: 'currency VARCHAR(3) NOT NULL',
    partitionKey: '—',
    description: 'ISO-4217 standard currency code (USD, EUR, GBP).',
    constraints: 'enum[USD, EUR, GBP]',
  },
  {
    path: 'data.completed_at',
    name: 'completed_at',
    type: 'string<date-time>',
    required: true,
    isPii: false,
    piiLevel: 'Public',
    lakeColumn: 'completed_at TIMESTAMP NOT NULL',
    partitionKey: 'days(completed_at)',
    description: 'ISO-8601 UTC timestamp of payment settlement.',
  },
];

export const MOCK_ICEBERG_DDL = `-- Apache Iceberg DDL Auto-Generated by Kalyx Registry Engine
-- Target Table: lake_orders_completed (v1.2.0)
-- Compatibility: Iceberg Table Spec v2 / Spark 3.5+

CREATE TABLE IF NOT EXISTS prod_lake.orders.lake_orders_completed (
    event_id STRING NOT NULL COMMENT 'CloudEvent canonical UUID',
    order_id STRING NOT NULL COMMENT 'Primary transaction identifier',
    customer_id STRING NOT NULL COMMENT 'Foreign key to identity graph',
    customer_email STRING COMMENT 'PII Confidentially Encrypted Field',
    amount_cents BIGINT NOT NULL COMMENT 'Completed transaction minor units',
    currency VARCHAR(3) NOT NULL,
    completed_at TIMESTAMP NOT NULL,
    _kalyx_ingested_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP(),
    _kalyx_version VARCHAR(16) NOT NULL DEFAULT '1.2.0'
)
USING iceberg
PARTITIONED BY (days(completed_at), bucket(16, order_id))
TBLPROPERTIES (
    'format-version' = '2',
    'write.upsert.enabled' = 'true',
    'history.expire.max-snapshot-age-ms' = '604800000',
    'kalyx.contract.urn' = 'urn:kalyx:orders:OrderCompleted:1.2.0'
);`;

export const MOCK_DELTA_DDL = `-- Delta Lake 3.0 DDL Auto-Generated by Kalyx Registry Engine
-- Target Table: delta_orders_completed (v1.2.0)
-- Engine: Databricks / Delta Lake Universal Format (UniForm)

CREATE TABLE IF NOT EXISTS delta_lake.orders.delta_orders_completed (
    event_id STRING NOT NULL,
    order_id STRING NOT NULL,
    customer_id STRING NOT NULL,
    customer_email STRING,
    amount_cents LONG NOT NULL,
    currency STRING NOT NULL,
    completed_at TIMESTAMP NOT NULL,
    _kalyx_ingested_at TIMESTAMP NOT NULL,
    _kalyx_version STRING NOT NULL
)
USING DELTA
PARTITIONED BY (DATE(completed_at))
TBLPROPERTIES (
    'delta.enableChangeDataFeed' = 'true',
    'delta.universalFormat.enabledFormats' = 'iceberg',
    'delta.minReaderVersion' = '2',
    'delta.minWriterVersion' = '5'
);`;

export const MOCK_PROPOSALS: Proposal[] = [
  {
    id: 'PR-104',
    eventId: 'orders.order.completed',
    eventName: 'OrderCompleted',
    branch: 'proposal/add-notes-field',
    baseVersion: 'v1.2.0',
    proposedVersion: 'v1.3.0',
    author: 'Alex Rivera',
    team: 'Checkout Core',
    status: 'pending_team',
    isBreaking: false,
    breakingReasons: [],
    justification: 'Adding optional `loyalty_tier` and fulfillment `notes` to support the Q3 Global Loyalty Program rollout in the checkout-worker microservice.',
    createdAt: '2025-06-18T09:30:00Z',
    reviewers: [
      { name: 'Sarah Chen', role: 'Lead Platform Architect', status: 'Awaiting', avatarInitials: 'SA' },
      { name: 'Marcus Vance', role: 'Analytics Engineering Lead', status: 'Awaiting', avatarInitials: 'MD', isDataSteward: true },
    ],
    diffSummary: {
      linesAdded: 10,
      linesDeleted: 0,
      fieldsAdded: 2,
      fieldsModified: 0,
    },
  },
  {
    id: 'PR-102',
    eventId: 'payments.payment.processed',
    eventName: 'PaymentProcessed',
    branch: 'proposal/remove-legacy-card-hash',
    baseVersion: 'v2.0.0',
    proposedVersion: 'v3.0.0',
    author: 'Elena Rostova',
    team: 'Billing Team',
    status: 'pending_steward',
    isBreaking: true,
    breakingReasons: [
      "Property 'card_hash' was removed from required properties.",
      "Downstream Delta Lake column 'card_hash' deletion requires table migration.",
    ],
    justification: 'PCI-DSS v4.0 compliance migration: transitioning from legacy truncated card hashes to zero-knowledge vault tokens.',
    createdAt: '2025-06-16T14:10:00Z',
    reviewers: [
      { name: 'David Kim', role: 'Billing Lead', status: 'Approved', avatarInitials: 'DK' },
      { name: 'Marcus Vance', role: 'Data Steward', status: 'Awaiting', avatarInitials: 'MV', isDataSteward: true },
    ],
    diffSummary: {
      linesAdded: 4,
      linesDeleted: 12,
      fieldsAdded: 1,
      fieldsModified: 2,
    },
  },
];

export const MOCK_PAT_TOKENS: PATToken[] = [
  {
    id: 'pat-001',
    name: 'GitHub Actions CI - Checkout Service',
    token_preview: 'kalyx_sec_7a9f...e4b1',
    created_at: '2025-01-15T11:00:00Z',
    last_used_at: '2025-06-18T12:04:12Z',
    scopes: ['events:read', 'events:validate', 'proposals:write'],
  },
  {
    id: 'pat-002',
    name: 'ArgoCD Sync - Lakehouse CDC Ingestion',
    token_preview: 'kalyx_sec_10cd...8910',
    created_at: '2025-02-01T08:30:00Z',
    last_used_at: '2025-06-18T13:00:00Z',
    scopes: ['events:read', 'ddl:export'],
  },
  {
    id: 'pat-003',
    name: 'Local Developer CLI (Alex Rivera)',
    token_preview: 'kalyx_sec_ff41...98aa',
    created_at: '2025-04-10T14:20:00Z',
    last_used_at: '2025-06-17T18:45:00Z',
    scopes: ['*'],
  },
];

export const MOCK_TEAM_MAPPINGS: TeamMapping[] = [
  {
    idp_group: 'idp-grp-checkout-engineers',
    kalyx_team: 'Checkout Core',
    role: 'Producer & Consumer',
    members_count: 14,
  },
  {
    idp_group: 'idp-grp-data-stewards',
    kalyx_team: 'Data Governance Stewards',
    role: 'Global Data Steward (Breaking Change Gatekeeper)',
    members_count: 4,
  },
  {
    idp_group: 'idp-grp-billing-platform',
    kalyx_team: 'Billing Team',
    role: 'Producer & Consumer',
    members_count: 9,
  },
  {
    idp_group: 'idp-grp-analytics-bi',
    kalyx_team: 'Analytics & Lakehouse',
    role: 'Data Consumer & Lake Sync Admin',
    members_count: 22,
  },
];
