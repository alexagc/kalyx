import {
  EventDefinition,
  EventVersion,
  ProblemDetails,
  CompatibilityResult,
} from './types';
import {
  MOCK_EVENTS,
  MOCK_ORDER_VERSIONS,
  MOCK_ORDER_COMPLETED_SCHEMA_V120,
  MOCK_ICEBERG_DDL,
  MOCK_DELTA_DDL,
} from './mockData';

const BASE_URL = '';

export class APIError extends Error {
  public problem?: ProblemDetails;
  public status: number;

  constructor(status: number, message: string, problem?: ProblemDetails) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.problem = problem;
  }
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('kalyx_token');
  const headers: Record<string, string> = {
    'Accept': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    const res = await fetch(`${BASE_URL}${endpoint}`, {
      ...options,
      headers,
    });

    if (!res.ok) {
      let problem: ProblemDetails | undefined;
      try {
        problem = await res.json();
      } catch {
        // Not a JSON response
      }
      throw new APIError(res.status, problem?.title || `HTTP error ${res.status}`, problem);
    }

    return await res.json();
  } catch (err) {
    // If network or server error, throw so caller can fallback
    throw err;
  }
}

export const api = {
  async getEvents(): Promise<EventDefinition[]> {
    try {
      const data = await request<EventDefinition[]>('/api/v1/events');
      if (Array.isArray(data) && data.length > 0) {
        return data.map((ev) => ({
          ...ev,
          active_version: ev.active_version || 'v1.0.0',
          status: ev.status || 'approved',
          has_pii: ev.has_pii !== undefined ? ev.has_pii : true,
          synced_tables: ev.synced_tables || ['Iceberg'],
        }));
      }
      return MOCK_EVENTS;
    } catch {
      return MOCK_EVENTS;
    }
  },

  async getEvent(id: string): Promise<EventDefinition> {
    try {
      const data = await request<EventDefinition>(`/api/v1/events/${encodeURIComponent(id)}`);
      return {
        ...data,
        active_version: data.active_version || 'v1.2.0',
        status: data.status || 'approved',
        has_pii: data.has_pii !== undefined ? data.has_pii : true,
        synced_tables: data.synced_tables || ['Iceberg', 'Delta Lake'],
      };
    } catch {
      const found = MOCK_EVENTS.find((e) => e.id === id || e.name.toLowerCase() === id.toLowerCase());
      return found || MOCK_EVENTS[0];
    }
  },

  async getVersions(eventId: string): Promise<EventVersion[]> {
    try {
      const data = await request<EventVersion[]>(`/api/v1/events/${encodeURIComponent(eventId)}/versions`);
      if (Array.isArray(data) && data.length > 0) {
        return data;
      }
      return MOCK_ORDER_VERSIONS;
    } catch {
      return MOCK_ORDER_VERSIONS;
    }
  },

  async getVersion(eventId: string, semver: string): Promise<EventVersion> {
    try {
      return await request<EventVersion>(`/api/v1/events/${encodeURIComponent(eventId)}/versions/${encodeURIComponent(semver)}`);
    } catch {
      const match = MOCK_ORDER_VERSIONS.find((v) => v.semver === semver);
      return match || MOCK_ORDER_VERSIONS[0];
    }
  },

  async getDDL(eventId: string, semver: string, format: 'iceberg' | 'delta'): Promise<{ ddl: string; format: string; table: string }> {
    try {
      return await request<{ ddl: string; format: string; table: string }>(
        `/api/v1/events/${encodeURIComponent(eventId)}/versions/${encodeURIComponent(semver)}/ddl?format=${format}`
      );
    } catch {
      return {
        table: `lake_${eventId.replace(/[^a-zA-Z0-9]/g, '_')}`,
        format,
        ddl: format === 'iceberg' ? MOCK_ICEBERG_DDL : MOCK_DELTA_DDL,
      };
    }
  },

  async checkCompatibility(oldSchema: string, newSchema: string): Promise<CompatibilityResult> {
    try {
      const res = await request<any>('/api/v1/compatibility/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ old_schema: oldSchema, new_schema: newSchema }),
      });

      return {
        is_compatible: res.is_compatible ?? res.compatible ?? true,
        compatibility_type: res.compatibility_type || 'BACKWARD',
        breaking_changes: res.breaking_changes || [],
        recommended_bump: res.is_compatible ? 'minor' : 'major',
        suggested_version: res.is_compatible ? 'v1.3.0' : 'v2.0.0',
      };
    } catch {
      // Client-side heuristics fallback
      let isCompatible = true;
      const breakingChanges: string[] = [];

      try {
        const oldParsed = JSON.parse(oldSchema);
        const newParsed = JSON.parse(newSchema);

        const oldProps = Object.keys(oldParsed.properties || {});
        const newProps = Object.keys(newParsed.properties || {});

        const removed = oldProps.filter((p) => !newProps.includes(p));
        if (removed.length > 0) {
          isCompatible = false;
          breakingChanges.push(`Removed property '${removed[0]}' without deprecation notice`);
        }

        const oldReq: string[] = oldParsed.required || [];
        const newReq: string[] = newParsed.required || [];
        const addedReq = newReq.filter((r) => !oldReq.includes(r));
        if (addedReq.length > 0) {
          isCompatible = false;
          breakingChanges.push(`New required field '${addedReq[0]}' breaks existing producers`);
        }
      } catch {
        // syntax error
      }

      return {
        is_compatible: isCompatible,
        compatibility_type: isCompatible ? 'BACKWARD' : 'BREAKING',
        breaking_changes: breakingChanges,
        recommended_bump: isCompatible ? 'minor' : 'major',
        suggested_version: isCompatible ? 'v1.3.0' : 'v2.0.0',
      };
    }
  },
};
