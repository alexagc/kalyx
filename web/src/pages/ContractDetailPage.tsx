import React, { useState, useEffect, useMemo } from 'react';
import { EventDefinition, EventVersion, FieldSpec } from '../types';
import { api } from '../api';
import {
  MOCK_ORDER_VERSIONS,
  MOCK_FIELD_SPECS,
  MOCK_ICEBERG_DDL,
  MOCK_DELTA_DDL,
  MOCK_ORDER_COMPLETED_SCHEMA_V120,
} from '../mockData';

interface ContractDetailPageProps {
  eventId: string;
  onNavigate: (path: string) => void;
  onShowToast: (message: string, type?: 'success' | 'info' | 'error') => void;
}

export const ContractDetailPage: React.FC<ContractDetailPageProps> = ({
  eventId,
  onNavigate,
  onShowToast,
}) => {
  const [event, setEvent] = useState<EventDefinition | null>(null);
  const [versions, setVersions] = useState<EventVersion[]>([]);
  const [currentVersion, setCurrentVersion] = useState<EventVersion | null>(null);
  const [versionDropdownOpen, setVersionDropdownOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<'fields' | 'schema' | 'bridge' | 'evolution'>('fields');
  const [ddlFormat, setDdlFormat] = useState<'iceberg' | 'delta'>('iceberg');

  // Fields table filter states
  const [fieldSearch, setFieldSearch] = useState('');
  const [fieldTypeFilter, setFieldTypeFilter] = useState<'ALL' | 'primitive' | 'complex'>('ALL');
  const [piiOnly, setPiiOnly] = useState(false);

  useEffect(() => {
    let mounted = true;
    api.getEvent(eventId).then((ev) => {
      if (mounted) {
        setEvent(ev);
      }
    });

    api.getVersions(eventId).then((vers) => {
      if (mounted) {
        setVersions(vers);
        if (vers.length > 0) {
          setCurrentVersion(vers[0]);
        }
      }
    });

    return () => {
      mounted = false;
    };
  }, [eventId]);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard?.writeText(text);
    onShowToast(`Copied ${label} to clipboard`, 'success');
  };

  const filteredFields = useMemo(() => {
    return MOCK_FIELD_SPECS.filter((f) => {
      if (piiOnly && !f.isPii) return false;
      if (fieldTypeFilter === 'primitive' && (f.type.includes('object') || f.type.includes('array'))) {
        return false;
      }
      if (fieldTypeFilter === 'complex' && !f.type.includes('object') && !f.type.includes('array')) {
        return false;
      }
      if (fieldSearch.trim()) {
        const q = fieldSearch.toLowerCase();
        const matchesName = f.name.toLowerCase().includes(q);
        const matchesPath = f.path.toLowerCase().includes(q);
        const matchesCol = f.lakeColumn.toLowerCase().includes(q);
        if (!matchesName && !matchesPath && !matchesCol) return false;
      }
      return true;
    });
  }, [fieldSearch, fieldTypeFilter, piiOnly]);

  const isDeprecated = currentVersion?.status === 'deprecated' || currentVersion?.semver === '1.0.0';

  return (
    <div className="flex flex-col w-full">
      {/* 1. TOP WORKBENCH SUB-HEADER BAR (Sticky under app-shell) */}
      <section className="sticky top-14 z-30 w-full bg-surface-container-lowest/95 backdrop-blur-md px-margin py-3 border-b border-outline-variant/40 flex flex-col gap-3">
        {/* Breadcrumbs & Quick Metadata */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2 font-label-sm text-label-sm text-on-surface-variant flex-wrap">
            <button
              onClick={() => onNavigate('/events')}
              className="hover:text-primary transition-colors flex items-center gap-1"
            >
              <span className="material-symbols-outlined text-[15px] text-outline">account_tree</span>
              <span>Contracts</span>
            </button>
            <span className="text-outline-variant">/</span>
            <span className="px-2 py-0.5 rounded bg-surface-container-high text-on-surface font-body-sm font-medium">
              {event?.domain || 'Orders'}
            </span>
            <span className="text-outline-variant">/</span>
            <span className="text-primary font-semibold font-label-md text-label-md">
              {event?.name || 'OrderCompleted'}
            </span>
            <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container-high border border-outline-variant text-outline text-[10px] font-label-sm">
              Producer: {event?.owner_team || 'checkout-svc'}
            </span>
          </div>

          {/* Quick Action Badges & Hashes */}
          <div className="flex items-center gap-2">
            <div className="hidden md:flex items-center gap-1.5 px-2 py-1 rounded bg-surface-container border border-outline-variant/60 font-label-sm text-label-sm text-on-surface-variant">
              <span className="material-symbols-outlined text-[14px] text-outline">fingerprint</span>
              <span className="text-outline">sha256:</span>
              <span className="text-primary select-all">7f49a2c94bb80e14...8e</span>
              <button
                className="hover:text-on-surface ml-0.5 transition-colors"
                onClick={() =>
                  copyToClipboard(
                    '7f49a2c94bb80e14ec71de99fa68902d18471b78913b5e4ff58231c9448e',
                    'SHA-256 Hash'
                  )
                }
                title="Copy SHA256 Hash"
              >
                <span className="material-symbols-outlined text-[13px]">content_copy</span>
              </button>
            </div>
            <div className="flex items-center gap-1.5 px-2 py-1 rounded bg-tertiary/10 border border-tertiary/30 text-tertiary font-label-sm text-label-sm">
              <span className="relative flex h-1.5 w-1.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-tertiary opacity-75"></span>
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-tertiary-container"></span>
              </span>
              <span>CloudEvents 1.0 Spec</span>
            </div>
          </div>
        </div>

        {/* Contract Title Row + Version Switcher + Workbench CTAs */}
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex items-center gap-2">
              <h1 className="font-headline-lg text-headline-lg text-on-surface tracking-tight font-semibold flex items-center gap-2">
                {event?.name || 'OrderCompleted'}
                <span className="px-2 py-0.5 rounded bg-surface-container-high border border-outline-variant font-label-sm text-label-sm text-outline font-normal">
                  v{currentVersion?.semver || '1.2.0'}
                </span>
              </h1>
            </div>

            {/* CloudEvent Type Pill */}
            <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-DEFAULT bg-surface-container-low border border-outline-variant font-label-sm text-label-sm text-primary">
              <span className="material-symbols-outlined text-[14px] text-primary">tag</span>
              <span className="select-all tracking-tight font-mono">
                {event?.type || 'orders.order.completed.v1'}
              </span>
              <button
                className="hover:text-on-surface text-outline transition-colors p-0.5"
                onClick={() =>
                  copyToClipboard(event?.type || 'orders.order.completed.v1', 'CloudEvent Type')
                }
                title="Copy CE Type"
              >
                <span className="material-symbols-outlined text-[13px]">content_copy</span>
              </button>
            </div>

            {/* Version Selector Dropdown */}
            <div className="relative inline-block text-left">
              <button
                type="button"
                onClick={() => setVersionDropdownOpen(!versionDropdownOpen)}
                className="h-8 px-2.5 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface flex items-center gap-2 font-label-sm text-label-sm transition-colors"
              >
                <span
                  className={`w-2 h-2 rounded-full ${
                    isDeprecated
                      ? 'bg-error shadow-[0_0_8px_rgba(255,180,171,0.8)]'
                      : 'bg-tertiary shadow-[0_0_8px_rgba(86,229,169,0.8)]'
                  }`}
                ></span>
                <span className="font-medium text-on-surface">
                  v{currentVersion?.semver || '1.2.0'}{' '}
                  {currentVersion?.status === 'approved'
                    ? '(Approved - Current)'
                    : `(${currentVersion?.status})`}
                </span>
                <span className="material-symbols-outlined text-[15px] text-outline">
                  expand_more
                </span>
              </button>

              {versionDropdownOpen && (
                <div className="absolute left-0 mt-1.5 w-72 rounded-DEFAULT bg-surface-container-low border border-outline-variant shadow-xl z-50 p-1 font-label-sm text-label-sm">
                  <div className="px-2 py-1.5 text-[10px] uppercase font-semibold text-outline tracking-wider">
                    Semantic Versions
                  </div>
                  {versions.map((v) => (
                    <button
                      key={v.semver}
                      onClick={() => {
                        setCurrentVersion(v);
                        setVersionDropdownOpen(false);
                      }}
                      className={`w-full flex items-center justify-between px-2.5 py-2 rounded text-left transition-colors ${
                        currentVersion?.semver === v.semver
                          ? 'bg-surface-container-high text-primary border-l-2 border-primary-container'
                          : 'hover:bg-surface-container text-on-surface-variant'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <span
                          className={`w-1.5 h-1.5 rounded-full ${
                            v.status === 'approved'
                              ? 'bg-tertiary'
                              : v.status === 'deprecated'
                              ? 'bg-error'
                              : 'bg-outline'
                          }`}
                        ></span>
                        <span>v{v.semver}</span>
                        <span
                          className={`px-1 rounded text-[10px] ${
                            v.status === 'approved'
                              ? 'bg-tertiary/10 text-tertiary'
                              : v.status === 'deprecated'
                              ? 'bg-error-container/30 text-error'
                              : 'bg-surface-container-highest text-outline'
                          }`}
                        >
                          {v.status}
                        </span>
                      </div>
                      <span className="text-[10px] text-outline">
                        {v.created_at.slice(0, 10)}
                      </span>
                    </button>
                  ))}
                  <div className="border-t border-outline-variant/60 my-1 pt-1">
                    <button
                      onClick={() => {
                        setVersionDropdownOpen(false);
                        onNavigate(`/events/${event?.id || 'orders.order.completed'}/propose`);
                      }}
                      className="w-full flex items-center gap-1.5 px-2.5 py-1.5 text-outline hover:text-primary transition-colors text-[11px]"
                    >
                      <span className="material-symbols-outlined text-[13px]">history</span>
                      <span>Compare diff across versions...</span>
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Action CTAs */}
          <div className="flex items-center gap-2 flex-wrap">
            <button
              onClick={() => setActiveTab('bridge')}
              className="h-8 px-2.5 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-body-sm text-body-sm inline-flex items-center gap-1.5 transition-colors"
            >
              <span className="material-symbols-outlined text-[15px] text-outline">download</span>
              <span>Export DDL</span>
            </button>

            <button
              onClick={() => onShowToast('Subscribed to Kafka consumer group telemetry', 'info')}
              className="h-8 px-2.5 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-body-sm text-body-sm inline-flex items-center gap-1.5 transition-colors"
            >
              <span className="material-symbols-outlined text-[15px] text-outline">monitoring</span>
              <span>Inspect Kafka Topic</span>
            </button>

            <button
              onClick={() =>
                onNavigate(`/events/${event?.id || 'orders.order.completed'}/propose`)
              }
              className="h-8 px-3 rounded bg-primary-container text-on-primary font-body-sm text-body-sm font-semibold inline-flex items-center gap-1.5 hover:bg-primary transition-colors shadow-sm"
            >
              <span className="material-symbols-outlined text-[16px]">fork_right</span>
              <span>Propose Schema Change</span>
            </button>
          </div>
        </div>
      </section>

      {/* 2. RFC 8594 DEPRECATION NOTICE BANNER */}
      {isDeprecated ? (
        <div className="px-margin pt-3">
          <div className="rounded-DEFAULT bg-surface-container-low border border-amber-900/60 p-3 flex flex-col md:flex-row md:items-center justify-between gap-3 shadow-inner">
            <div className="flex items-start gap-2.5 min-w-0">
              <span className="material-symbols-outlined text-amber-400 text-[18px] shrink-0 mt-0.5">
                warning
              </span>
              <div className="text-body-sm font-body-sm text-on-surface-variant leading-relaxed">
                <span className="font-medium text-amber-300">RFC 8594 Deprecation Notice:</span>{' '}
                This contract version sunsets on{' '}
                <time className="font-label-sm text-label-sm text-amber-200" dateTime="2026-12-31">
                  2026-12-31
                </time>
                . Recommended successor:{' '}
                <button
                  onClick={() => {
                    if (versions[0]) setCurrentVersion(versions[0]);
                  }}
                  className="font-label-sm text-label-sm text-primary hover:underline font-semibold"
                >
                  v1.2.0 Approved
                </button>
                . Please migrate all producers and stream consumers before the sunset deadline.
              </div>
            </div>
            <div className="flex items-center gap-2 self-start md:self-auto shrink-0">
              <div className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-error-container/20 border border-error/30 text-error font-label-sm text-label-sm">
                <span>RFC 8594 Sunset Active</span>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="px-margin pt-3">
          <div className="rounded-DEFAULT bg-surface-container-low border border-outline-variant/40 p-3 flex flex-col md:flex-row md:items-center justify-between gap-3 shadow-inner">
            <div className="flex items-start gap-2.5 min-w-0">
              <span className="material-symbols-outlined text-tertiary text-[18px] shrink-0 mt-0.5">
                verified
              </span>
              <div className="text-body-sm font-body-sm text-on-surface-variant leading-relaxed">
                <span className="font-medium text-tertiary">Active Production Contract:</span>{' '}
                Version{' '}
                <span className="font-label-sm text-label-sm text-on-surface font-semibold">
                  v{currentVersion?.semver || '1.2.0'}
                </span>{' '}
                is active and enforced across API Gateways and Apache Iceberg CDC pipelines.
              </div>
            </div>
            <div className="flex items-center gap-2 self-start md:self-auto shrink-0">
              <div className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-tertiary-container/20 border border-tertiary/30 text-tertiary font-label-sm text-label-sm">
                <span className="material-symbols-outlined text-[13px]">swap_horiz</span>
                <span>BACKWARD_TRANSITIVE</span>
              </div>
              <div className="flex items-center gap-1 px-2 py-0.5 rounded bg-surface-container border border-outline-variant font-label-sm text-label-sm text-on-surface">
                <span className="material-symbols-outlined text-tertiary text-[14px]">
                  check_circle
                </span>
                <span>Linter: Passed (0 warnings)</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 3. PRIMARY WORKBENCH TAB WORKSPACE */}
      <div className="px-margin py-4 flex flex-col gap-4">
        {/* Tab Navigation Bar */}
        <div className="border-b border-outline-variant flex items-center justify-between gap-4 overflow-x-auto select-none">
          <div className="flex items-center gap-1">
            <button
              onClick={() => setActiveTab('fields')}
              className={`px-3.5 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'fields'
                  ? 'border-primary text-primary'
                  : 'border-transparent text-on-surface-variant hover:text-on-surface'
              }`}
            >
              <span className="material-symbols-outlined text-[17px]">table_chart</span>
              <span>Properties &amp; Fields Table</span>
              <span className="px-1.5 py-0.2 rounded-full bg-surface-container-high text-on-surface-variant font-label-sm text-[10px]">
                {filteredFields.length} fields
              </span>
            </button>

            <button
              onClick={() => setActiveTab('schema')}
              className={`px-3.5 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'schema'
                  ? 'border-primary text-primary'
                  : 'border-transparent text-on-surface-variant hover:text-on-surface'
              }`}
            >
              <span className="material-symbols-outlined text-[17px]">data_object</span>
              <span>Raw JSON Schema</span>
              <span className="px-1.5 py-0.2 rounded bg-surface-container text-outline font-label-sm text-[10px]">
                Draft 2020-12
              </span>
            </button>

            <button
              onClick={() => setActiveTab('bridge')}
              className={`px-3.5 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'bridge'
                  ? 'border-primary text-primary'
                  : 'border-transparent text-on-surface-variant hover:text-on-surface'
              }`}
            >
              <span className="material-symbols-outlined text-[17px]">hub</span>
              <span>Data Lake Bridge</span>
              <span className="px-1.5 py-0.2 rounded bg-secondary-container/20 border border-secondary/30 text-secondary font-label-sm text-[10px]">
                Iceberg / Delta
              </span>
            </button>

            <button
              onClick={() => setActiveTab('evolution')}
              className={`px-3.5 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'evolution'
                  ? 'border-primary text-primary'
                  : 'border-transparent text-on-surface-variant hover:text-on-surface'
              }`}
            >
              <span className="material-symbols-outlined text-[17px]">timeline</span>
              <span>SemVer Evolution &amp; Audit Trail</span>
              <span className="w-1.5 h-1.5 rounded-full bg-tertiary"></span>
            </button>
          </div>

          <div className="flex items-center gap-2 text-outline font-label-sm text-label-sm pb-1 shrink-0">
            <span className="material-symbols-outlined text-[14px]">lock</span>
            <span>Immutable (Production Locked)</span>
          </div>
        </div>

        {/* TAB 1: Fields & Properties Table */}
        {activeTab === 'fields' && (
          <section className="flex flex-col gap-3">
            {/* High Density Filter Toolbar */}
            <div className="p-2.5 rounded-DEFAULT bg-surface-container-low border border-outline-variant flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2.5 flex-1 min-w-[280px]">
                <div className="relative w-full max-w-sm">
                  <span className="material-symbols-outlined absolute left-2.5 top-1/2 -translate-y-1/2 text-outline text-[16px]">
                    filter_list
                  </span>
                  <input
                    className="w-full h-8 pl-8 pr-3 rounded bg-surface-container-lowest border border-outline-variant text-on-surface placeholder:text-outline font-body-sm text-body-sm focus:outline-none focus:border-primary transition-colors"
                    placeholder="Filter fields by name, path or lake mapping..."
                    type="text"
                    value={fieldSearch}
                    onChange={(e) => setFieldSearch(e.target.value)}
                  />
                </div>

                <div className="hidden sm:flex items-center gap-1 bg-surface-container-lowest p-0.5 rounded border border-outline-variant font-label-sm text-label-sm">
                  <button
                    onClick={() => setFieldTypeFilter('ALL')}
                    className={`px-2 py-1 rounded transition-colors ${
                      fieldTypeFilter === 'ALL'
                        ? 'bg-surface-container text-primary font-medium'
                        : 'text-outline hover:text-on-surface'
                    }`}
                  >
                    All ({MOCK_FIELD_SPECS.length})
                  </button>
                  <button
                    onClick={() => setFieldTypeFilter('primitive')}
                    className={`px-2 py-1 rounded transition-colors ${
                      fieldTypeFilter === 'primitive'
                        ? 'bg-surface-container text-primary font-medium'
                        : 'text-outline hover:text-on-surface'
                    }`}
                  >
                    Primitives
                  </button>
                  <button
                    onClick={() => setFieldTypeFilter('complex')}
                    className={`px-2 py-1 rounded transition-colors ${
                      fieldTypeFilter === 'complex'
                        ? 'bg-surface-container text-primary font-medium'
                        : 'text-outline hover:text-on-surface'
                    }`}
                  >
                    Objects / Arrays
                  </button>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <label className="inline-flex items-center gap-2 cursor-pointer text-on-surface-variant hover:text-on-surface transition-colors font-body-sm text-body-sm">
                  <input
                    className="rounded border-outline bg-surface-container text-primary focus:ring-0 cursor-pointer h-3.5 w-3.5"
                    type="checkbox"
                    checked={piiOnly}
                    onChange={(e) => setPiiOnly(e.target.checked)}
                  />
                  <span className="flex items-center gap-1">
                    <span className="w-2 h-2 rounded-full bg-secondary"></span>
                    <span>Only PII/Sensitive Fields</span>
                  </span>
                </label>
                <span className="text-outline-variant">|</span>
                <div className="flex items-center gap-1 text-outline font-label-sm text-label-sm">
                  <span>Showing:</span>
                  <span className="text-on-surface font-semibold">{filteredFields.length}</span>
                  <span>of {MOCK_FIELD_SPECS.length} columns</span>
                </div>
              </div>
            </div>

            {/* High Density Field Specification Table */}
            <div className="overflow-hidden rounded-DEFAULT border border-outline-variant bg-surface-container-low shadow-sm">
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="h-8 bg-surface-container border-b border-outline-variant font-label-sm text-label-sm text-outline uppercase tracking-wider select-none">
                      <th className="py-1 px-3 w-64">Field Path &amp; Name</th>
                      <th className="py-1 px-3 w-28">Type</th>
                      <th className="py-1 px-3 w-36">Constraints</th>
                      <th className="py-1 px-3 w-48">PII &amp; Security</th>
                      <th className="py-1 px-3 w-56">Data Lake Column (Iceberg)</th>
                      <th className="py-1 px-3 w-44">Partition / Key</th>
                      <th className="py-1 px-3">Description &amp; Documentation</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-outline-variant/40 font-body-sm text-body-sm">
                    {filteredFields.map((field) => (
                      <tr
                        key={field.path}
                        className={`h-10 transition-colors group ${
                          field.isPii
                            ? 'bg-secondary-container/10 hover:bg-secondary-container/20'
                            : 'hover:bg-surface-container'
                        }`}
                      >
                        <td className="py-1.5 px-3">
                          <div className="flex items-center gap-1.5">
                            {field.path.startsWith('data.') && (
                              <span className="text-outline-variant">↳</span>
                            )}
                            <span
                              className={`font-label-sm text-label-sm font-semibold ${
                                field.isPii
                                  ? 'text-secondary'
                                  : field.path === 'id'
                                  ? 'text-primary'
                                  : 'text-on-surface'
                              }`}
                            >
                              {field.path}
                            </span>
                            {field.path === 'id' && (
                              <span className="px-1 rounded bg-surface-container-high text-[10px] text-outline font-label-sm">
                                CE
                              </span>
                            )}
                            {field.isPii && (
                              <span
                                className="w-1.5 h-1.5 rounded-full bg-secondary animate-pulse"
                                title="PII Sensitive Field"
                              ></span>
                            )}
                          </div>
                        </td>

                        <td className="py-1.5 px-3">
                          <span className="px-1.5 py-0.5 rounded bg-surface-container border border-outline-variant/60 font-label-sm text-label-sm text-on-surface">
                            {field.type}
                          </span>
                        </td>

                        <td className="py-1.5 px-3">
                          <div className="flex items-center gap-1 flex-wrap">
                            {field.required && (
                              <span className="px-1.5 py-0.5 rounded bg-error-container/20 text-error border border-error/30 font-label-sm text-[10px] font-semibold uppercase">
                                Required
                              </span>
                            )}
                            {field.constraints && (
                              <span className="text-[10px] text-outline font-label-sm">
                                {field.constraints}
                              </span>
                            )}
                          </div>
                        </td>

                        <td className="py-1.5 px-3">
                          <span
                            className={`px-1.5 py-0.5 rounded font-label-sm text-[11px] ${
                              field.isPii
                                ? 'bg-secondary-container/30 text-secondary border border-secondary/40 font-medium'
                                : 'bg-surface-container text-outline'
                            }`}
                          >
                            {field.piiLevel}
                          </span>
                        </td>

                        <td className="py-1.5 px-3">
                          <code className="font-code-inline text-code-inline text-on-surface-variant bg-surface-container-lowest px-1 py-0.5 rounded">
                            {field.lakeColumn}
                          </code>
                        </td>

                        <td className="py-1.5 px-3">
                          {field.partitionKey !== '—' ? (
                            <span className="px-1.5 py-0.5 rounded bg-primary/10 border border-primary/30 text-primary font-label-sm text-[11px]">
                              {field.partitionKey}
                            </span>
                          ) : (
                            <span className="text-outline text-label-sm font-label-sm">—</span>
                          )}
                        </td>

                        <td className="py-1.5 px-3 text-on-surface-variant truncate max-w-xs">
                          {field.description}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </section>
        )}

        {/* TAB 2: Raw JSON Schema Viewer */}
        {activeTab === 'schema' && (
          <section className="flex flex-col gap-3">
            <div className="rounded-DEFAULT border border-outline-variant bg-[#0a0e16] overflow-hidden flex flex-col shadow-lg">
              <div className="px-4 py-2 bg-surface-container-low border-b border-outline-variant flex items-center justify-between text-label-sm">
                <div className="flex items-center gap-2">
                  <span className="text-on-surface font-medium">OrderCompleted.schema.json</span>
                  <span className="px-1.5 py-0.2 rounded bg-surface-container text-primary font-code-inline text-[10px]">
                    Draft 2020-12
                  </span>
                  <span className="text-outline text-[11px]">(Read Only)</span>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() =>
                      copyToClipboard(
                        currentVersion?.json_schema || MOCK_ORDER_COMPLETED_SCHEMA_V120,
                        'JSON Schema'
                      )
                    }
                    className="h-7 px-2.5 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-label-sm text-label-sm inline-flex items-center gap-1.5 transition-colors"
                  >
                    <span className="material-symbols-outlined text-[14px]">content_copy</span>
                    <span>Copy Schema</span>
                  </button>
                </div>
              </div>

              <pre className="p-4 font-mono text-[12px] leading-relaxed text-[#dfe2ee] overflow-x-auto selection:bg-primary-container selection:text-on-primary">
                {currentVersion?.json_schema || MOCK_ORDER_COMPLETED_SCHEMA_V120}
              </pre>
            </div>
          </section>
        )}

        {/* TAB 3: Data Lake Bridge (Iceberg & Delta Lake DDL) */}
        {activeTab === 'bridge' && (
          <section className="flex flex-col gap-3">
            <div className="p-3 rounded-DEFAULT bg-surface-container-low border border-outline-variant flex items-center justify-between">
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setDdlFormat('iceberg')}
                  className={`px-3 py-1.5 rounded font-label-sm text-label-sm font-semibold transition-colors flex items-center gap-1.5 ${
                    ddlFormat === 'iceberg'
                      ? 'bg-primary-container text-on-primary'
                      : 'bg-surface-container text-on-surface-variant hover:text-on-surface'
                  }`}
                >
                  <span className="material-symbols-outlined text-[16px]">layers</span>
                  <span>Apache Iceberg (v2)</span>
                </button>
                <button
                  onClick={() => setDdlFormat('delta')}
                  className={`px-3 py-1.5 rounded font-label-sm text-label-sm font-semibold transition-colors flex items-center gap-1.5 ${
                    ddlFormat === 'delta'
                      ? 'bg-primary-container text-on-primary'
                      : 'bg-surface-container text-on-surface-variant hover:text-on-surface'
                  }`}
                >
                  <span className="material-symbols-outlined text-[16px]">change_circle</span>
                  <span>Delta Lake 3.0 (UniForm)</span>
                </button>
              </div>

              <button
                onClick={() =>
                  copyToClipboard(
                    ddlFormat === 'iceberg' ? MOCK_ICEBERG_DDL : MOCK_DELTA_DDL,
                    `${ddlFormat.toUpperCase()} DDL`
                  )
                }
                className="h-7 px-3 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-label-sm text-label-sm inline-flex items-center gap-1.5 transition-colors"
              >
                <span className="material-symbols-outlined text-[14px]">content_copy</span>
                <span>Copy SQL DDL</span>
              </button>
            </div>

            <div className="rounded-DEFAULT border border-outline-variant bg-[#0a0e16] p-4 font-mono text-[12px] leading-relaxed text-[#dfe2ee] overflow-x-auto shadow-lg">
              <pre>{ddlFormat === 'iceberg' ? MOCK_ICEBERG_DDL : MOCK_DELTA_DDL}</pre>
            </div>
          </section>
        )}

        {/* TAB 4: SemVer Evolution Timeline */}
        {activeTab === 'evolution' && (
          <section className="flex flex-col gap-4">
            <div className="p-space-md rounded-DEFAULT bg-surface-container-low border border-outline-variant flex flex-col gap-4">
              <div className="flex items-center justify-between border-b border-outline-variant/60 pb-3">
                <div className="flex items-center gap-2">
                  <span className="material-symbols-outlined text-tertiary text-[18px]">
                    history_edu
                  </span>
                  <h3 className="font-headline-md text-headline-md text-on-surface">
                    Version Progression &amp; Governance Audit Log
                  </h3>
                </div>
                <span className="font-label-sm text-label-sm text-outline">
                  3 historical snapshots recorded
                </span>
              </div>

              <div className="flex flex-col gap-4 pl-2 border-l-2 border-outline-variant/60 ml-3">
                {versions.map((ver, idx) => (
                  <div key={ver.semver} className="relative flex flex-col gap-1 pl-4">
                    <span
                      className={`absolute -left-[23px] top-1 w-3 h-3 rounded-full border-2 border-surface ${
                        ver.status === 'approved'
                          ? 'bg-tertiary'
                          : ver.status === 'deprecated'
                          ? 'bg-error'
                          : 'bg-outline'
                      }`}
                    ></span>

                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-code-inline text-[13px] font-bold text-primary">
                        v{ver.semver}
                      </span>
                      <span
                        className={`px-1.5 py-0.2 rounded text-[10px] font-label-sm uppercase font-semibold ${
                          ver.status === 'approved'
                            ? 'bg-tertiary/10 text-tertiary border border-tertiary/30'
                            : ver.status === 'deprecated'
                            ? 'bg-error-container/20 text-error border border-error/30'
                            : 'bg-surface-container text-outline'
                        }`}
                      >
                        {ver.status}
                      </span>
                      <span className="text-outline text-label-sm">•</span>
                      <span className="text-outline text-label-sm font-label-sm">
                        {ver.created_at.slice(0, 10)}
                      </span>
                    </div>

                    <p className="text-body-sm text-on-surface mt-0.5">
                      {ver.changelog || 'Contract schema revision.'}
                    </p>

                    <div className="flex items-center gap-3 text-outline text-[11px] font-label-sm mt-1">
                      <span>
                        Author: <strong className="text-on-surface-variant">{ver.author || 'Alex Rivera'}</strong>
                      </span>
                      {ver.approver && (
                        <>
                          <span>•</span>
                          <span>
                            Approver: <strong className="text-tertiary">{ver.approver}</strong>
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  );
};
