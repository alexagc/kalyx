export type ContractStatus = 'approved' | 'in_review' | 'draft' | 'deprecated' | 'rejected';

export interface DataLakeMetadata {
  target_table: string;
  partition_by: string[];
  primary_key?: string[];
  format?: 'iceberg' | 'delta';
}

export interface EventDefinition {
  id: string;
  domain: string;
  name: string;
  type: string; // CloudEvent Type
  owner_team: string;
  created_at: string;
  updated_at: string;
  active_version?: string;
  status?: ContractStatus;
  pii_fields_count?: number;
  has_pii?: boolean;
  synced_tables?: string[];
  description?: string;
  fingerprint?: string;
}

export interface EventVersion {
  id: string;
  event_id: string;
  semver: string;
  status: ContractStatus;
  json_schema: string;
  data_lake_metadata: DataLakeMetadata;
  created_at: string;
  approved_at?: string | null;
  author?: string;
  approver?: string;
  changelog?: string;
}

export interface FieldSpec {
  path: string;
  name: string;
  type: string;
  required: boolean;
  isPii: boolean;
  piiLevel: string;
  lakeColumn: string;
  partitionKey: string;
  description: string;
  constraints?: string;
}

export interface Reviewer {
  name: string;
  role: string;
  status: 'Awaiting' | 'Approved' | 'Changes Requested';
  avatarInitials: string;
  isDataSteward?: boolean;
}

export interface Proposal {
  id: string;
  eventId: string;
  eventName: string;
  branch: string;
  baseVersion: string;
  proposedVersion: string;
  author: string;
  team: string;
  status: 'pending_team' | 'pending_steward' | 'approved' | 'rejected';
  isBreaking: boolean;
  breakingReasons: string[];
  justification: string;
  createdAt: string;
  reviewers: Reviewer[];
  diffSummary: {
    linesAdded: number;
    linesDeleted: number;
    fieldsAdded: number;
    fieldsModified: number;
  };
}

export interface PATToken {
  id: string;
  name: string;
  token_preview: string;
  created_at: string;
  last_used_at: string;
  scopes: string[];
}

export interface TeamMapping {
  idp_group: string;
  kalyx_team: string;
  role: string;
  members_count: number;
}

export interface ProblemDetails {
  type?: string;
  title: string;
  status: number;
  detail: string;
  instance?: string;
  invalid_params?: Array<{
    name: string;
    reason: string;
  }>;
}

export interface CompatibilityResult {
  is_compatible: boolean;
  compatibility_type: string;
  breaking_changes?: string[];
  recommended_bump: 'patch' | 'minor' | 'major';
  suggested_version: string;
}
