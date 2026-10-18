import React, { useState, useEffect, useMemo } from 'react';
import { EventDefinition, ContractStatus } from '../types';
import { api } from '../api';

interface CatalogPageProps {
  onNavigate: (path: string) => void;
  onShowToast: (message: string, type?: 'success' | 'info' | 'error') => void;
}

export const CatalogPage: React.FC<CatalogPageProps> = ({ onNavigate, onShowToast }) => {
  const [events, setEvents] = useState<EventDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedDomain, setSelectedDomain] = useState<string>('ALL');
  const [selectedStatus, setSelectedStatus] = useState<string>('ALL');
  const [piiOnly, setPiiOnly] = useState<boolean>(false);
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [viewMode, setViewMode] = useState<'table' | 'cards'>('table');

  useEffect(() => {
    let mounted = true;
    api.getEvents().then((data) => {
      if (mounted) {
        setEvents(data);
        setLoading(false);
      }
    });
    return () => {
      mounted = false;
    };
  }, []);

  const domains = useMemo(() => {
    const set = new Set<string>();
    events.forEach((e) => {
      if (e.domain) set.add(e.domain);
    });
    return Array.from(set);
  }, [events]);

  const domainCounts = useMemo(() => {
    const counts: Record<string, number> = { ALL: events.length };
    events.forEach((e) => {
      counts[e.domain] = (counts[e.domain] || 0) + 1;
    });
    return counts;
  }, [events]);

  const filteredEvents = useMemo(() => {
    return events.filter((ev) => {
      if (selectedDomain !== 'ALL' && ev.domain !== selectedDomain) return false;
      if (selectedStatus !== 'ALL') {
        if (selectedStatus === 'approved' && ev.status !== 'approved') return false;
        if (selectedStatus === 'in_review' && ev.status !== 'in_review') return false;
        if (selectedStatus === 'deprecated' && ev.status !== 'deprecated') return false;
      }
      if (piiOnly && !ev.has_pii) return false;
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const matchesName = ev.name.toLowerCase().includes(q);
        const matchesType = ev.type.toLowerCase().includes(q);
        const matchesDomain = ev.domain.toLowerCase().includes(q);
        const matchesTeam = ev.owner_team.toLowerCase().includes(q);
        if (!matchesName && !matchesType && !matchesDomain && !matchesTeam) return false;
      }
      return true;
    });
  }, [events, selectedDomain, selectedStatus, piiOnly, searchQuery]);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard?.writeText(text);
    onShowToast(`Copied ${label} to clipboard`, 'success');
  };

  return (
    <div className="flex flex-col w-full">
      <div className="px-space-lg py-space-md flex flex-col gap-space-lg max-w-[1720px] mx-auto w-full">
        {/* SUB-NAV / VIEW TITLE & STREAM HEALTH */}
        <div className="flex flex-wrap items-center justify-between gap-space-md">
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2">
              <span className="font-label-sm text-label-sm text-primary uppercase tracking-wider">
                Registry Engine
              </span>
              <span className="text-outline-variant font-label-sm text-label-sm">/</span>
              <span className="font-label-sm text-label-sm text-on-surface-variant">
                Production Namespace: prod-east-aws
              </span>
            </div>
            <h1 className="font-headline-lg text-headline-lg text-on-surface font-semibold tracking-tight">
              Event Contracts Catalog
            </h1>
          </div>

          <div className="flex items-center gap-space-sm flex-wrap">
            <div className="inline-flex items-center gap-1.5 px-space-sm py-1 rounded-DEFAULT bg-surface-container-low border border-outline-variant/40">
              <span className="material-symbols-outlined text-tertiary text-[15px]">
                check_circle
              </span>
              <span className="font-label-sm text-label-sm text-tertiary">
                Schema Lint: 0 Errors
              </span>
            </div>

            <div className="inline-flex items-center gap-1.5 px-space-sm py-1 rounded-DEFAULT bg-surface-container-high border border-outline-variant/60">
              <span className="font-label-sm text-label-sm text-outline">Fingerprint:</span>
              <span className="font-label-sm text-label-sm text-primary font-mono">
                sha256:7f49a2c
              </span>
            </div>

            <button
              onClick={() => onNavigate('/reviews')}
              className="h-8 px-space-md rounded-DEFAULT bg-surface-container-high hover:bg-surface-bright text-on-surface font-body-sm text-body-sm inline-flex items-center gap-1.5 transition-colors border border-outline-variant/60"
            >
              <span className="material-symbols-outlined text-[15px]">history</span>
              <span>Schema Changelog</span>
            </button>
          </div>
        </div>

        {/* 1. TOP METRICS MINI-RIBBON (3 Compact KPI Metric Cards) */}
        <section className="grid grid-cols-1 md:grid-cols-3 gap-space-md">
          {/* Card 1: Active Contracts */}
          <div className="p-space-md rounded-DEFAULT bg-surface-container-low hover:bg-surface-container transition-all flex flex-col justify-between border border-outline-variant/40 relative overflow-hidden group">
            <div className="flex items-start justify-between">
              <div className="flex flex-col">
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Active Contracts
                </span>
                <div className="flex items-baseline gap-2 mt-1">
                  <span className="font-headline-xl text-headline-xl text-on-surface font-bold leading-none">
                    48
                  </span>
                  <span className="inline-flex items-center px-1.5 py-0.5 rounded-DEFAULT bg-tertiary/10 text-tertiary font-label-sm text-label-sm font-semibold">
                    +3 this wk
                  </span>
                </div>
              </div>
              <div className="w-8 h-8 rounded-DEFAULT bg-surface-container-high flex items-center justify-center text-primary">
                <span className="material-symbols-outlined text-[18px]">data_object</span>
              </div>
            </div>

            <div className="mt-space-md pt-space-xs flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-tertiary"></span>
                <span className="font-label-sm text-label-sm text-on-surface-variant">
                  100% backward compatible
                </span>
              </div>
              {/* Sparkline representation */}
              <svg
                className="w-16 h-4 text-tertiary/80 stroke-current fill-none stroke-[1.5]"
                viewBox="0 0 64 16"
              >
                <polyline points="0,12 12,10 24,14 36,7 48,8 64,2"></polyline>
              </svg>
            </div>
          </div>

          {/* Card 2: Pending Approvals */}
          <div className="p-space-md rounded-DEFAULT bg-surface-container-low hover:bg-surface-container transition-all flex flex-col justify-between border border-outline-variant/40 relative overflow-hidden group">
            <div className="flex items-start justify-between">
              <div className="flex flex-col">
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Pending Approvals
                </span>
                <div className="flex items-baseline gap-2 mt-1">
                  <span className="font-headline-xl text-headline-xl text-secondary font-bold leading-none">
                    2
                  </span>
                  <span className="font-label-sm text-label-sm text-secondary font-medium">
                    Proposals
                  </span>
                </div>
              </div>
              <div className="w-8 h-8 rounded-DEFAULT bg-secondary-container/20 flex items-center justify-center text-secondary">
                <span className="material-symbols-outlined text-[18px]">pending_actions</span>
              </div>
            </div>

            <div className="mt-space-md pt-space-xs flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse"></span>
                <span className="font-label-sm text-label-sm text-on-surface-variant">
                  Awaiting architectural review
                </span>
              </div>
              <button
                onClick={() => onNavigate('/reviews')}
                className="font-label-sm text-label-sm text-primary hover:text-primary-fixed inline-flex items-center gap-0.5 transition-colors"
              >
                Review Queue
                <span className="material-symbols-outlined text-[14px]">arrow_forward</span>
              </button>
            </div>
          </div>

          {/* Card 3: Data Lake Tables Synced */}
          <div className="p-space-md rounded-DEFAULT bg-surface-container-low hover:bg-surface-container transition-all flex flex-col justify-between border border-outline-variant/40 relative overflow-hidden group">
            <div className="flex items-start justify-between">
              <div className="flex flex-col">
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Data Lake Tables Synced
                </span>
                <div className="flex items-baseline gap-2 mt-1">
                  <span className="font-headline-xl text-headline-xl text-on-surface font-bold leading-none">
                    48
                  </span>
                  <span className="font-label-sm text-label-sm text-primary font-medium">
                    Iceberg & Delta
                  </span>
                </div>
              </div>
              <div className="w-8 h-8 rounded-DEFAULT bg-surface-container-high flex items-center justify-center text-primary-container">
                <span className="material-symbols-outlined text-[18px]">table_chart_view</span>
              </div>
            </div>

            <div className="mt-space-md pt-space-xs flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <span className="material-symbols-outlined text-[14px] text-primary">sync</span>
                <span className="font-label-sm text-label-sm text-on-surface-variant">
                  Sub-second CDC sync active
                </span>
              </div>
              <span className="font-label-sm text-label-sm text-tertiary font-mono">0 lag</span>
            </div>
          </div>
        </section>

        {/* 2. FILTER & GOVERNANCE BAR */}
        <section className="p-space-sm rounded-DEFAULT bg-surface-container-low border border-outline-variant/50 flex flex-col lg:flex-row items-stretch lg:items-center justify-between gap-space-md">
          {/* Domain segment filter pills */}
          <div className="flex items-center gap-1 overflow-x-auto select-none pb-1 lg:pb-0">
            <button
              onClick={() => setSelectedDomain('ALL')}
              className={`px-space-sm py-1 rounded-DEFAULT font-label-sm text-label-sm font-semibold transition-colors flex items-center gap-1.5 ${
                selectedDomain === 'ALL'
                  ? 'bg-surface-container-high text-primary border border-outline-variant/80'
                  : 'hover:bg-surface-container text-on-surface-variant hover:text-on-surface'
              }`}
            >
              <span>All Domains</span>
              <span className="px-1 py-0.2 rounded-DEFAULT bg-surface-container-lowest text-[10px] text-primary-fixed">
                {domainCounts['ALL'] || 48}
              </span>
            </button>

            {domains.map((dom) => (
              <button
                key={dom}
                onClick={() => setSelectedDomain(dom)}
                className={`px-space-sm py-1 rounded-DEFAULT font-label-sm text-label-sm transition-colors flex items-center gap-1 ${
                  selectedDomain === dom
                    ? 'bg-surface-container-high text-primary border border-outline-variant/80'
                    : 'hover:bg-surface-container text-on-surface-variant hover:text-on-surface'
                }`}
              >
                <span>{dom}</span>
                <span className="text-outline text-[10px]">{domainCounts[dom] || 0}</span>
              </button>
            ))}
          </div>

          {/* Controls & Dropdowns */}
          <div className="flex items-center gap-space-sm flex-wrap">
            {/* Status Dropdown */}
            <div className="relative">
              <select
                value={selectedStatus}
                onChange={(e) => setSelectedStatus(e.target.value)}
                aria-label="Filter contracts by status"
                className="h-8 px-space-sm pr-6 rounded-DEFAULT bg-surface-container-lowest hover:bg-surface-container text-on-surface font-body-sm text-body-sm border border-outline-variant/60 focus:outline-none focus:border-primary transition-colors cursor-pointer appearance-none"
              >
                <option value="ALL">Status: All Statuses</option>
                <option value="approved">Status: Approved</option>
                <option value="in_review">Status: In Review</option>
                <option value="deprecated">Status: Deprecated</option>
              </select>
            </div>

            {/* PII Toggle Switch */}
            <label className="h-8 px-space-sm rounded-DEFAULT bg-surface-container-lowest border border-outline-variant/60 flex items-center gap-2 cursor-pointer select-none">
              <input
                className="sr-only peer"
                type="checkbox"
                checked={piiOnly}
                onChange={(e) => setPiiOnly(e.target.checked)}
              />
              <span className="material-symbols-outlined text-[14px] text-secondary">shield</span>
              <span className="font-label-sm text-label-sm text-on-surface-variant peer-checked:text-on-surface">
                PII Only
              </span>
              <div
                className={`w-7 h-4 rounded-full p-0.5 transition-colors relative ${
                  piiOnly ? 'bg-secondary' : 'bg-surface-container-highest'
                }`}
              >
                <div
                  className={`w-3 h-3 bg-on-surface rounded-full transition-transform ${
                    piiOnly ? 'translate-x-3' : 'translate-x-0'
                  }`}
                ></div>
              </div>
            </label>

            {/* View Mode Segment */}
            <div className="inline-flex rounded-DEFAULT bg-surface-container-lowest border border-outline-variant/60 p-0.5">
              <button
                onClick={() => setViewMode('table')}
                className={`p-1 rounded-DEFAULT ${
                  viewMode === 'table'
                    ? 'bg-surface-container-high text-primary'
                    : 'text-outline hover:text-on-surface'
                }`}
                title="Dense Table View"
              >
                <span className="material-symbols-outlined text-[16px]">view_list</span>
              </button>
              <button
                onClick={() => setViewMode('cards')}
                className={`p-1 rounded-DEFAULT ${
                  viewMode === 'cards'
                    ? 'bg-surface-container-high text-primary'
                    : 'text-outline hover:text-on-surface'
                }`}
                title="Compact Cards View"
              >
                <span className="material-symbols-outlined text-[16px]">grid_view</span>
              </button>
            </div>
          </div>
        </section>

        {/* 3. HIGH-DENSITY CONTRACTS DATA TABLE */}
        <section className="rounded-DEFAULT bg-surface-container-lowest border border-outline-variant/60 overflow-hidden flex flex-col shadow-sm">
          {loading ? (
            <div className="p-12 flex flex-col items-center justify-center gap-3">
              <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
              <span className="font-label-sm text-label-sm text-outline">
                Loading event contracts from registry...
              </span>
            </div>
          ) : filteredEvents.length === 0 ? (
            <div className="p-12 flex flex-col items-center justify-center gap-2 text-center">
              <span className="material-symbols-outlined text-outline text-[32px]">
                filter_list_off
              </span>
              <p className="font-headline-md text-on-surface font-medium">No event contracts match</p>
              <p className="font-body-sm text-outline max-w-md">
                No event contracts match current filters. Try resetting domain or status filters.
              </p>
              <button
                onClick={() => {
                  setSelectedDomain('ALL');
                  setSelectedStatus('ALL');
                  setPiiOnly(false);
                  setSearchQuery('');
                }}
                className="mt-2 h-7 px-3 rounded bg-surface-container-high text-primary hover:bg-surface-bright text-body-sm font-medium transition-colors"
              >
                Reset All Filters
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto w-full">
              <table className="w-full text-left border-collapse select-text">
                <thead>
                  <tr className="h-9 bg-surface-container-low text-outline font-label-sm text-label-sm uppercase tracking-wider border-b border-outline-variant/60">
                    <th className="px-space-md py-1 font-semibold w-52">Event Name</th>
                    <th className="px-space-md py-1 font-semibold">CloudEvent Type (Spec 1.0)</th>
                    <th className="px-space-sm py-1 font-semibold w-28">Domain</th>
                    <th className="px-space-md py-1 font-semibold w-40">Active SemVer</th>
                    <th className="px-space-md py-1 font-semibold">Data Lake Bridge</th>
                    <th className="px-space-md py-1 font-semibold w-36">PII Risk</th>
                    <th className="px-space-md py-1 font-semibold text-right w-44">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-outline-variant/30 font-body-sm text-body-sm text-on-surface">
                  {filteredEvents.map((contract) => {
                    const isApproved = contract.status === 'approved';
                    const isInReview = contract.status === 'in_review';
                    const isDeprecated = contract.status === 'deprecated';

                    return (
                      <tr
                        key={contract.id}
                        onClick={() => onNavigate(`/events/${contract.name.toLowerCase()}`)}
                        className="h-10 hover:bg-surface-container/60 transition-colors group cursor-pointer"
                      >
                        {/* Event Name */}
                        <td className="px-space-md py-1.5 font-medium">
                          <div className="flex items-center gap-2">
                            <div
                              className={`w-1.5 h-4 rounded-sm ${
                                isApproved
                                  ? 'bg-primary-container'
                                  : isInReview
                                  ? 'bg-secondary'
                                  : 'bg-outline'
                              }`}
                            ></div>
                            <span className="font-headline-md text-body-md text-on-surface font-semibold group-hover:text-primary transition-colors">
                              {contract.name}
                            </span>
                          </div>
                        </td>

                        {/* CloudEvent Type */}
                        <td className="px-space-md py-1.5">
                          <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-surface-container-low font-label-sm text-label-sm text-primary border border-outline-variant/40">
                            <span className="font-mono">{contract.type}</span>
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                copyToClipboard(contract.type, 'CloudEvent Type');
                              }}
                              className="text-outline hover:text-on-surface transition-colors"
                              title="Copy CloudEvent Type"
                            >
                              <span className="material-symbols-outlined text-[13px]">
                                content_copy
                              </span>
                            </button>
                          </div>
                        </td>

                        {/* Domain */}
                        <td className="px-space-sm py-1.5">
                          <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container-high text-on-surface-variant font-label-sm text-label-sm">
                            {contract.domain}
                          </span>
                        </td>

                        {/* Active SemVer */}
                        <td className="px-space-md py-1.5">
                          {isApproved && (
                            <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-tertiary/10 text-tertiary font-label-sm text-label-sm font-semibold border border-tertiary/20">
                              <span className="w-1.5 h-1.5 rounded-full bg-tertiary"></span>
                              <span>{contract.active_version} Approved</span>
                            </div>
                          )}
                          {isInReview && (
                            <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-secondary-container/20 text-secondary font-label-sm text-label-sm font-semibold border border-secondary/30">
                              <span className="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse"></span>
                              <span>{contract.active_version} In Review</span>
                            </div>
                          )}
                          {isDeprecated && (
                            <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-error-container/20 text-error font-label-sm text-label-sm font-semibold border border-error/30">
                              <span className="w-1.5 h-1.5 rounded-full bg-error"></span>
                              <span>{contract.active_version} Deprecated</span>
                            </div>
                          )}
                        </td>

                        {/* Data Lake Bridge */}
                        <td className="px-space-md py-1.5">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container-high text-primary-fixed-dim font-label-sm text-label-sm inline-flex items-center gap-1 border border-outline-variant/40">
                              <span className="material-symbols-outlined text-[12px]">check</span>
                              <span>Iceberg</span>
                            </span>
                            {contract.synced_tables?.some((t) => t.toLowerCase().includes('delta')) && (
                              <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container-high text-primary font-label-sm text-label-sm inline-flex items-center gap-1 border border-outline-variant/40">
                                <span className="material-symbols-outlined text-[12px]">check</span>
                                <span>Delta Lake</span>
                              </span>
                            )}
                          </div>
                        </td>

                        {/* PII Risk */}
                        <td className="px-space-md py-1.5">
                          {contract.has_pii ? (
                            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-DEFAULT bg-secondary-container/20 text-secondary font-label-sm text-label-sm border border-secondary/30">
                              <span className="material-symbols-outlined text-[12px]">shield</span>
                              <span>
                                {contract.pii_fields_count || 1}{' '}
                                {contract.pii_fields_count === 1 ? 'PII Field' : 'PII Fields'}
                              </span>
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-DEFAULT bg-surface-container text-outline font-label-sm text-label-sm">
                              None
                            </span>
                          )}
                        </td>

                        {/* Actions */}
                        <td className="px-space-md py-1.5 text-right">
                          <div
                            className="inline-flex items-center gap-1"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <button
                              onClick={() => onNavigate(`/events/${contract.name.toLowerCase()}`)}
                              className="h-6 px-2 rounded-DEFAULT bg-surface-container-high hover:bg-surface-bright text-on-surface font-label-sm text-label-sm inline-flex items-center gap-1 transition-colors border border-outline-variant/60"
                              title="Inspect Schema"
                            >
                              <span className="material-symbols-outlined text-[13px]">code</span>
                              <span>Inspect</span>
                            </button>
                            <button
                              onClick={() => onNavigate(`/export`)}
                              className="p-1 rounded-DEFAULT hover:bg-surface-container-high text-outline hover:text-on-surface transition-colors"
                              title="Export DDL"
                            >
                              <span className="material-symbols-outlined text-[16px]">
                                file_download
                              </span>
                            </button>
                            <button
                              onClick={() => onNavigate(`/events/${contract.name.toLowerCase()}/propose`)}
                              className="p-1 rounded-DEFAULT hover:bg-surface-container-high text-outline hover:text-on-surface transition-colors"
                              title="Propose Schema Change"
                            >
                              <span className="material-symbols-outlined text-[16px]">fork_right</span>
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>
    </div>
  );
};
