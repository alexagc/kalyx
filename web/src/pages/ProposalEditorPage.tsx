import React, { useState, useEffect } from 'react';
import { api } from '../api';
import {
  MOCK_ORDER_COMPLETED_SCHEMA_V120,
  MOCK_ORDER_COMPLETED_PROPOSED_SCHEMA,
  MOCK_ORDER_COMPLETED_BREAKING_SCHEMA,
} from '../mockData';

interface ProposalEditorPageProps {
  eventId: string;
  onNavigate: (path: string) => void;
  onShowToast: (message: string, type?: 'success' | 'info' | 'error') => void;
}

export const ProposalEditorPage: React.FC<ProposalEditorPageProps> = ({
  eventId,
  onNavigate,
  onShowToast,
}) => {
  const baseSchema = MOCK_ORDER_COMPLETED_SCHEMA_V120;
  const [draftSchema, setDraftSchema] = useState<string>(MOCK_ORDER_COMPLETED_PROPOSED_SCHEMA);
  const [isBreaking, setIsBreaking] = useState(false);
  const [rulesDrawerOpen, setRulesDrawerOpen] = useState(false);
  const [justification, setJustification] = useState(
    'Adding optional `loyalty_tier` and fulfillment `notes` to support the Q3 Global Loyalty Program rollout in the checkout-worker microservice. These fields are nullable and defaulted in producer payload.'
  );
  const [icebergChecked, setIcebergChecked] = useState(true);
  const [kafkaChecked, setKafkaChecked] = useState(true);
  const [diffViewMode, setDiffViewMode] = useState<'split' | 'unified'>('split');
  const [lintStatus, setLintStatus] = useState<'valid' | 'invalid'>('valid');

  // Trigger compatibility check on draft change
  useEffect(() => {
    const timer = setTimeout(() => {
      try {
        JSON.parse(draftSchema);
        setLintStatus('valid');
        api.checkCompatibility(baseSchema, draftSchema).then((res) => {
          setIsBreaking(!res.is_compatible);
        });
      } catch {
        setLintStatus('invalid');
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [draftSchema, baseSchema]);

  const handleSimulateSafe = () => {
    setDraftSchema(MOCK_ORDER_COMPLETED_PROPOSED_SCHEMA);
    setIsBreaking(false);
    onShowToast('Simulating safe backward-compatible addition (v1.3.0)', 'info');
  };

  const handleSimulateBreaking = () => {
    setDraftSchema(MOCK_ORDER_COMPLETED_BREAKING_SCHEMA);
    setIsBreaking(true);
    onShowToast('Simulating breaking deletion requiring major bump (v2.0.0)', 'error');
  };

  const handleRevert = () => {
    setDraftSchema(MOCK_ORDER_COMPLETED_SCHEMA_V120);
    setIsBreaking(false);
    onShowToast('Reverted draft to active approved schema (v1.2.0)', 'info');
  };

  const handleSubmitProposal = (e: React.FormEvent) => {
    e.preventDefault();
    if (lintStatus === 'invalid') {
      onShowToast('Cannot submit: JSON Schema has syntax errors', 'error');
      return;
    }
    if (!justification.trim()) {
      onShowToast('Please provide an engineering change justification', 'error');
      return;
    }

    onShowToast(
      isBreaking
        ? 'Proposal submitted for Data Steward and Team Lead review (Major Breaking)'
        : 'Proposal submitted for Team Lead review (Minor Compatible)',
      'success'
    );
    onNavigate('/reviews');
  };

  return (
    <div className="flex flex-col w-full pb-16">
      {/* 1. TOP CONTEXT & HEADER BAR */}
      <section className="w-full bg-surface-container-lowest px-space-lg py-space-md border-b border-outline-variant/40">
        <div className="flex flex-col gap-space-sm max-w-[1720px] mx-auto">
          {/* Breadcrumbs & Status Flag */}
          <div className="flex items-center justify-between flex-wrap gap-space-sm">
            <div className="flex items-center gap-1.5 font-label-sm text-label-sm">
              <button
                onClick={() => onNavigate('/events')}
                className="text-outline hover:text-on-surface transition-colors"
              >
                Kalyx
              </button>
              <span className="text-outline-variant">/</span>
              <button
                onClick={() => onNavigate('/events')}
                className="text-outline hover:text-on-surface transition-colors"
              >
                Contracts Catalog
              </button>
              <span className="text-outline-variant">/</span>
              <button
                onClick={() => onNavigate(`/events/${eventId || 'orders.order.completed'}`)}
                className="text-primary hover:underline font-medium"
              >
                OrderCompleted
              </button>
              <span className="text-outline-variant">/</span>
              <span className="text-outline">Proposals</span>
              <span className="text-outline-variant">/</span>
              <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container text-primary-fixed-dim font-label-sm">
                #PR-104 (Draft)
              </span>
            </div>

            {/* Live Engine Lint Pill */}
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded bg-surface-container-low border border-tertiary-container/40 shadow-sm">
              <span className="relative flex h-2 w-2">
                <span
                  className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-80 ${
                    lintStatus === 'valid' ? 'bg-tertiary' : 'bg-error'
                  }`}
                ></span>
                <span
                  className={`relative inline-flex rounded-full h-2 w-2 ${
                    lintStatus === 'valid' ? 'bg-tertiary' : 'bg-error'
                  }`}
                ></span>
              </span>
              <span
                className={`font-label-sm text-label-sm font-semibold tracking-wide uppercase ${
                  lintStatus === 'valid' ? 'text-tertiary' : 'text-error'
                }`}
              >
                Schema Lint
              </span>
              <span className="text-outline-variant">|</span>
              <span className="font-label-sm text-label-sm text-on-surface">
                {lintStatus === 'valid'
                  ? 'Valid JSONSchema Draft 2020-12'
                  : 'JSON Syntax Error in Draft'}
              </span>
            </div>
          </div>

          {/* Title & Target Branch Matrix */}
          <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-space-md mt-1">
            <div className="flex flex-col gap-1">
              <div className="flex items-center flex-wrap gap-2.5">
                <h1 className="font-headline-lg text-headline-lg text-on-surface tracking-tight flex items-center gap-2">
                  <span>Propose Schema Update:</span>
                  <span className="text-primary font-code-inline text-[19px]">OrderCompleted</span>
                </h1>
                <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-surface-container-high border border-outline-variant/60 font-label-sm text-label-sm text-on-surface-variant">
                  <span className="material-symbols-outlined text-[14px] text-tertiary">
                    alt_route
                  </span>
                  <span>proposal/add-notes-field</span>
                </div>
                <div className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-DEFAULT bg-surface-container-high border border-outline-variant/60 font-label-sm text-label-sm text-on-surface-variant">
                  <span className="material-symbols-outlined text-[14px] text-primary">
                    my_location
                  </span>
                  <span>
                    target: <strong className="text-on-surface font-semibold">main (v1.2.0)</strong>
                  </span>
                </div>
              </div>
              <p className="font-body-sm text-body-sm text-outline max-w-4xl">
                Modifying active schema{' '}
                <span className="font-code-inline text-[11px] text-primary">v1.2.0</span> to propose
                a new contract version. Changes are continuously evaluated against Apache Iceberg
                (v1.4+) &amp; Delta Lake 3.0 backward evolution constraints.
              </p>
            </div>

            {/* Header Quick Action Bar */}
            <div className="flex items-center gap-2 flex-wrap self-start lg:self-center">
              <button
                type="button"
                onClick={handleRevert}
                className="h-8 px-2.5 rounded-DEFAULT bg-surface-container hover:bg-surface-container-high border border-outline-variant/80 text-on-surface-variant hover:text-on-surface font-body-sm text-body-sm inline-flex items-center gap-1.5 transition-colors"
                title="Revert draft changes to base version"
              >
                <span className="material-symbols-outlined text-[15px]">undo</span>
                <span>Revert Changes</span>
              </button>

              <div className="inline-flex p-0.5 rounded bg-surface-container-lowest border border-outline-variant">
                <button
                  type="button"
                  onClick={() => setDiffViewMode('split')}
                  className={`px-2 py-1 rounded-DEFAULT font-label-sm text-label-sm flex items-center gap-1 ${
                    diffViewMode === 'split'
                      ? 'bg-surface-container text-primary font-medium'
                      : 'text-outline hover:text-on-surface'
                  }`}
                  title="Side-by-side split view"
                >
                  <span className="material-symbols-outlined text-[14px]">view_column</span>
                  <span>Split</span>
                </button>
                <button
                  type="button"
                  onClick={() => setDiffViewMode('unified')}
                  className={`px-2 py-1 rounded-DEFAULT font-label-sm text-label-sm flex items-center gap-1 ${
                    diffViewMode === 'unified'
                      ? 'bg-surface-container text-primary font-medium'
                      : 'text-outline hover:text-on-surface'
                  }`}
                  title="Unified editor view"
                >
                  <span className="material-symbols-outlined text-[14px]">table_rows</span>
                  <span>Unified</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 2. SPLIT MONACO DIFF CONTAINER */}
      <div className="w-full px-space-lg py-space-md flex flex-col gap-space-lg max-w-[1720px] mx-auto">
        <div className="w-full rounded-DEFAULT border border-outline-variant/80 bg-surface-container-lowest shadow-xl flex flex-col overflow-hidden">
          {/* Panes Metadata Bar */}
          <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-outline-variant/70 border-b border-outline-variant/80 bg-surface-container-low text-label-sm font-label-sm select-none">
            {/* Left Pane Header (Active Schema) */}
            <div className="px-space-md py-2 flex items-center justify-between text-outline">
              <div className="flex items-center gap-2">
                <span className="material-symbols-outlined text-[15px] text-outline">lock</span>
                <span className="text-on-surface font-medium">OrderCompleted.schema.json</span>
                <span className="px-1.5 py-0.2 rounded bg-surface-container text-on-surface-variant font-code-inline text-[10px]">
                  v1.2.0 • SHA: 7f49a2c
                </span>
                <span className="text-outline text-[11px]">(Active Approved)</span>
              </div>
              <span className="text-[11px] uppercase tracking-wider text-outline-variant font-semibold">
                Read Only
              </span>
            </div>

            {/* Right Pane Header (Draft Workspace) */}
            <div className="px-space-md py-2 flex items-center justify-between bg-surface-container-low text-on-surface">
              <div className="flex items-center gap-2">
                <span className="material-symbols-outlined text-[15px] text-tertiary">
                  edit_note
                </span>
                <span className="text-on-surface font-medium">OrderCompleted.draft.json</span>
                <span className="px-1.5 py-0.2 rounded bg-tertiary-container/20 border border-tertiary/30 text-tertiary font-code-inline text-[10px]">
                  {isBreaking ? 'v2.0.0-propose.1' : 'v1.3.0-propose.1'}
                </span>
                <span className="text-tertiary text-[11px] flex items-center gap-1 font-medium">
                  <span className="w-1.5 h-1.5 rounded-full bg-tertiary animate-pulse"></span>
                  In-Memory Buffer
                </span>
              </div>
              <div className="flex items-center gap-3">
                <span className="font-code-inline text-[11px] text-primary">
                  UTF-8 • JSONSchema 2020-12
                </span>
                <span className="text-outline-variant">|</span>
                <span className="font-code-inline text-[11px] text-outline">Spaces: 2</span>
              </div>
            </div>
          </div>

          {/* Split Code Body */}
          <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-outline-variant/60 font-mono text-[12px] leading-5 bg-[#0a0e16] text-[#dfe2ee]">
            {/* Left Pane (Base) */}
            <div className="p-3 bg-[#070a0f] overflow-x-auto whitespace-pre selection:bg-surface-container-high">
              <pre className="font-mono text-[12px] leading-relaxed text-[#dfe2ee]">
                {baseSchema}
              </pre>
            </div>

            {/* Right Pane (Editable Draft) */}
            <div className="relative p-2 bg-[#0b0f17]">
              <textarea
                value={draftSchema}
                onChange={(e) => setDraftSchema(e.target.value)}
                rows={28}
                spellCheck={false}
                className="w-full h-full bg-transparent text-[#dfe2ee] font-mono text-[12px] leading-relaxed resize-none focus:outline-none p-1 selection:bg-primary-container selection:text-on-primary-container"
              />
            </div>
          </div>

          {/* Diff Footer Summary Bar */}
          <div className="px-space-md py-1.5 bg-surface-container border-t border-outline-variant/60 flex items-center justify-between text-label-sm font-label-sm text-outline">
            <div className="flex items-center gap-3">
              {isBreaking ? (
                <span className="text-error font-semibold flex items-center gap-1">
                  <span className="material-symbols-outlined text-[14px]">warning</span>
                  Breaking Deletions / Constraints
                </span>
              ) : (
                <span className="text-tertiary font-semibold flex items-center gap-1">
                  <span className="material-symbols-outlined text-[14px]">add_circle</span>
                  +10 lines added
                </span>
              )}
              <span className="text-outline-variant">•</span>
              <span>{isBreaking ? '2 deletions' : '0 deletions'}</span>
              <span className="text-outline-variant">•</span>
              <span className="text-on-surface">
                {isBreaking ? '1 field removed' : '2 fields introduced'}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span>
                Schema Validation:{' '}
                <span
                  className={`font-medium ${
                    lintStatus === 'valid' ? 'text-tertiary' : 'text-error'
                  }`}
                >
                  {lintStatus === 'valid' ? 'PASSED' : 'ERROR'}
                </span>
              </span>
              <span className="text-outline-variant">•</span>
              <span>Memory Footprint: 2.1 KB</span>
            </div>
          </div>
        </div>

        {/* 3. REAL-TIME SEMVER DIAGNOSTIC & COMPATIBILITY ENGINE RIBBON */}
        <div className="w-full flex flex-col gap-space-sm">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="material-symbols-outlined text-tertiary text-[18px]">verified</span>
              <h2 className="font-headline-md text-headline-md text-on-surface font-semibold">
                Automated SemVer &amp; Data Lake Compatibility Engine
              </h2>
              <span className="px-2 py-0.5 rounded bg-tertiary-container/15 text-tertiary font-label-sm text-label-sm border border-tertiary/20">
                Continuous Verifier 2.8
              </span>
            </div>

            {/* Simulation Mode Switcher for Engineer Testing */}
            <div className="flex items-center gap-2 bg-surface-container-low px-2 py-1 rounded border border-outline-variant">
              <span className="font-label-sm text-label-sm text-outline">Simulate:</span>
              <button
                type="button"
                onClick={handleSimulateSafe}
                className={`px-2 py-0.5 rounded font-label-sm text-label-sm font-semibold transition-all ${
                  !isBreaking
                    ? 'bg-tertiary-container text-on-tertiary shadow-sm'
                    : 'text-outline hover:text-on-surface'
                }`}
              >
                Safe Add (v1.3.0)
              </button>
              <button
                type="button"
                onClick={handleSimulateBreaking}
                className={`px-2 py-0.5 rounded font-label-sm text-label-sm font-semibold transition-all ${
                  isBreaking
                    ? 'bg-error-container text-error shadow-sm'
                    : 'text-outline hover:text-on-surface'
                }`}
              >
                Breaking Delete (v2.0.0)
              </button>
            </div>
          </div>

          {/* Active Analysis Card (Green Minor vs Red Major) */}
          <div
            className={`w-full rounded-DEFAULT border p-space-md shadow-lg flex flex-col lg:flex-row gap-space-md items-start lg:items-center justify-between transition-all ${
              isBreaking
                ? 'border-error/40 bg-gradient-to-r from-surface-container-low via-surface-container-low to-error-container/10'
                : 'border-tertiary/40 bg-gradient-to-r from-surface-container-low via-surface-container-low to-tertiary-container/5'
            }`}
          >
            <div className="flex items-start gap-space-md">
              <div
                className={`w-10 h-10 rounded-DEFAULT border flex-shrink-0 flex items-center justify-center ${
                  isBreaking
                    ? 'bg-error-container/20 border-error/40 text-error'
                    : 'bg-tertiary-container/20 border-tertiary/40 text-tertiary'
                }`}
              >
                <span className="material-symbols-outlined text-[24px]">
                  {isBreaking ? 'gpp_maybe' : 'published_with_changes'}
                </span>
              </div>

              <div className="flex flex-col gap-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <span
                    className={`px-2 py-0.5 rounded border font-label-sm text-label-sm font-bold tracking-wide ${
                      isBreaking
                        ? 'bg-error-container/30 border-error/50 text-error'
                        : 'bg-tertiary-container/20 border-tertiary/40 text-tertiary'
                    }`}
                  >
                    {isBreaking
                      ? 'MAJOR BREAKING CHANGE DETECTED'
                      : 'MINOR COMPATIBLE CHANGE DETECTED'}
                  </span>
                  <span className="font-code-inline text-[11px] text-outline">
                    {isBreaking ? 'RULE: REMOVED_FIELD' : 'RULE: ADD_OPTIONAL_FIELD'}
                  </span>
                  <span className="text-outline-variant">•</span>
                  <span className="font-body-sm text-body-sm text-on-surface">
                    Target Recommended:{' '}
                    <strong
                      className={`font-code-inline text-label-md ${
                        isBreaking ? 'text-error' : 'text-primary'
                      }`}
                    >
                      {isBreaking ? 'v2.0.0' : 'v1.3.0'}
                    </strong>{' '}
                    (from v1.2.0)
                  </span>
                </div>

                <p className="font-body-md text-body-md text-on-surface">
                  {isBreaking
                    ? 'Removed property customer_id and customer_email from payload contract. Existing consumers expecting these fields will fail deserialization.'
                    : 'Added optional properties loyalty_tier and notes without altering existing required constraints.'}
                </p>

                <p className="font-body-sm text-body-sm text-outline">
                  <strong className="text-on-surface">Data Lake Bridge Impact:</strong>{' '}
                  {isBreaking
                    ? 'Column deletions in Apache Iceberg require migration script and explicit Data Steward sign-off.'
                    : 'Fully backward compatible with Apache Iceberg Schema Evolution (Iceberg column IDs 109 & 110 reserved). Requires Checkout Domain Lead approval.'}
                </p>
              </div>
            </div>

            <div className="flex flex-col sm:flex-row items-center gap-2 self-stretch lg:self-center">
              <div className="flex flex-col text-right pr-2 hidden sm:flex">
                <span className="font-label-sm text-label-sm text-outline">Governance Gate</span>
                <span
                  className={`font-label-sm text-label-sm font-semibold ${
                    isBreaking ? 'text-error' : 'text-tertiary'
                  }`}
                >
                  {isBreaking ? 'Requires Data Steward Gate' : 'Ready for Review'}
                </span>
              </div>
              <button
                type="button"
                onClick={() => setRulesDrawerOpen(!rulesDrawerOpen)}
                className="w-full sm:w-auto h-8 px-space-md rounded-DEFAULT bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-body-sm text-body-sm inline-flex items-center justify-center gap-1.5 transition-colors"
              >
                <span className="material-symbols-outlined text-[16px]">fact_check</span>
                <span>View Rule Checklist ({isBreaking ? '2/4' : '4/4'})</span>
              </button>
            </div>
          </div>

          {/* Collapsible Automated Rule Checklist */}
          {rulesDrawerOpen && (
            <div className="w-full rounded-DEFAULT border border-outline-variant/60 bg-surface-container-low p-space-md grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-space-md animate-fadeIn">
              {/* Rule 1 */}
              <div className="flex items-start gap-2.5 p-space-sm rounded bg-surface-container/60 border border-outline-variant/30">
                <span className="material-symbols-outlined text-tertiary text-[18px] mt-0.5">
                  check_circle
                </span>
                <div className="flex flex-col">
                  <span className="font-label-sm text-label-sm text-on-surface font-semibold">
                    No Non-Default Required
                  </span>
                  <span className="font-body-sm text-body-sm text-outline mt-0.5">
                    Existing payload parsers won't fail with missing keys.
                  </span>
                  <span className="font-label-sm text-[10px] text-tertiary mt-1">STATUS: PASS</span>
                </div>
              </div>

              {/* Rule 2 */}
              <div className="flex items-start gap-2.5 p-space-sm rounded bg-surface-container/60 border border-outline-variant/30">
                <span
                  className={`material-symbols-outlined text-[18px] mt-0.5 ${
                    isBreaking ? 'text-error' : 'text-tertiary'
                  }`}
                >
                  {isBreaking ? 'cancel' : 'check_circle'}
                </span>
                <div className="flex flex-col">
                  <span className="font-label-sm text-label-sm text-on-surface font-semibold">
                    Zero Deletions / Renames
                  </span>
                  <span className="font-body-sm text-body-sm text-outline mt-0.5">
                    {isBreaking
                      ? 'Fields were deleted from contract properties.'
                      : 'Active fields preserved without broken consumers.'}
                  </span>
                  <span
                    className={`font-label-sm text-[10px] mt-1 ${
                      isBreaking ? 'text-error' : 'text-tertiary'
                    }`}
                  >
                    STATUS: {isBreaking ? 'FAIL' : 'PASS'}
                  </span>
                </div>
              </div>

              {/* Rule 3 */}
              <div className="flex items-start gap-2.5 p-space-sm rounded bg-surface-container/60 border border-outline-variant/30">
                <span className="material-symbols-outlined text-tertiary text-[18px] mt-0.5">
                  check_circle
                </span>
                <div className="flex flex-col">
                  <span className="font-label-sm text-label-sm text-on-surface font-semibold">
                    Type Promotion Widening
                  </span>
                  <span className="font-body-sm text-body-sm text-outline mt-0.5">
                    No narrowing of integer or string types detected.
                  </span>
                  <span className="font-label-sm text-[10px] text-tertiary mt-1">STATUS: PASS</span>
                </div>
              </div>

              {/* Rule 4 */}
              <div className="flex items-start gap-2.5 p-space-sm rounded bg-surface-container/60 border border-outline-variant/30">
                <span className="material-symbols-outlined text-tertiary text-[18px] mt-0.5">
                  check_circle
                </span>
                <div className="flex flex-col">
                  <span className="font-label-sm text-label-sm text-on-surface font-semibold">
                    Automated PII Tagger
                  </span>
                  <span className="font-body-sm text-body-sm text-outline mt-0.5">
                    Added properties classified non-sensitive.
                  </span>
                  <span className="font-label-sm text-[10px] text-tertiary mt-1">STATUS: PASS</span>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* 4. PROPOSAL SUBMISSION & GOVERNANCE REVIEW FORM */}
        <form
          onSubmit={handleSubmitProposal}
          className="w-full rounded-DEFAULT border border-outline-variant/80 bg-surface-container-lowest p-space-lg shadow-lg flex flex-col gap-space-lg"
        >
          <div className="flex items-center justify-between border-b border-outline-variant/60 pb-space-sm">
            <div className="flex items-center gap-2">
              <span className="material-symbols-outlined text-primary text-[20px]">
                assignment_turned_in
              </span>
              <h3 className="font-headline-md text-headline-md text-on-surface font-semibold">
                Proposal Submission &amp; Governance Review
              </h3>
            </div>
            <span className="font-label-sm text-label-sm text-outline">
              Target Repository: <strong className="text-on-surface">github.com/company/event-contracts</strong>
            </span>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-12 gap-space-lg">
            {/* Left: Form Inputs & Description */}
            <div className="lg:col-span-8 flex flex-col gap-space-md">
              <div className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between">
                  <label
                    className="font-label-sm text-label-sm text-on-surface font-medium uppercase tracking-wider"
                    htmlFor="changeJustification"
                  >
                    Engineering Change Justification <span className="text-secondary">*</span>
                  </label>
                  <span className="font-label-sm text-[11px] text-outline">Markdown Enabled</span>
                </div>

                <textarea
                  id="changeJustification"
                  value={justification}
                  onChange={(e) => setJustification(e.target.value)}
                  className="w-full p-space-md bg-[#0a0e16] border border-outline-variant rounded text-on-surface font-body-md text-body-md focus:outline-none focus:border-primary transition-colors resize-y leading-relaxed"
                  rows={4}
                  placeholder="Explain the architectural rationale behind this contract modification..."
                />
              </div>

              {/* Downstream Safety Confirmations */}
              <div className="flex flex-col gap-2 pt-1">
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Downstream Verification Checklist
                </span>

                <label className="flex items-start gap-2.5 cursor-pointer select-none group">
                  <input
                    type="checkbox"
                    checked={icebergChecked}
                    onChange={(e) => setIcebergChecked(e.target.checked)}
                    className="mt-0.5 w-4 h-4 rounded bg-surface-container border-outline-variant text-primary focus:ring-0 focus:ring-offset-0 transition-colors"
                  />
                  <div className="flex flex-col">
                    <span className="font-body-sm text-body-sm text-on-surface group-hover:text-primary transition-colors">
                      I have verified that downstream Apache Iceberg (v1.4.1) &amp; Delta Lake tables
                      auto-evolve without column type collisions.
                    </span>
                    <span className="font-label-sm text-label-sm text-outline">
                      Iceberg schema evolution verified via catalog shadow engine.
                    </span>
                  </div>
                </label>

                <label className="flex items-start gap-2.5 cursor-pointer select-none group">
                  <input
                    type="checkbox"
                    checked={kafkaChecked}
                    onChange={(e) => setKafkaChecked(e.target.checked)}
                    className="mt-0.5 w-4 h-4 rounded bg-surface-container border-outline-variant text-primary focus:ring-0 focus:ring-offset-0 transition-colors"
                  />
                  <div className="flex flex-col">
                    <span className="font-body-sm text-body-sm text-on-surface group-hover:text-primary transition-colors">
                      Validated backward compatibility against active Kafka consumer groups (
                      <code className="text-primary font-code-inline text-[11px]">
                        orders-fulfillment-v1
                      </code>
                      ,{' '}
                      <code className="text-primary font-code-inline text-[11px]">
                        bi-lakehouse-cdc
                      </code>
                      ).
                    </span>
                    <span className="font-label-sm text-label-sm text-outline">
                      Zero deserialization failures simulated against last 50,000 consumer offsets.
                    </span>
                  </div>
                </label>
              </div>
            </div>

            {/* Right: Designated Reviewers & Policy Pipeline */}
            <div className="lg:col-span-4 flex flex-col gap-space-md bg-surface-container-low p-space-md rounded-DEFAULT border border-outline-variant/60">
              <div className="flex flex-col gap-1">
                <span className="font-label-sm text-label-sm text-on-surface font-semibold uppercase tracking-wider">
                  Designated Governance Reviewers
                </span>
                <span className="font-body-sm text-body-sm text-outline">
                  Assigned by Kalyx Codeowners routing rules
                </span>
              </div>

              {/* Reviewer Cards */}
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between p-2 rounded bg-surface-container border border-outline-variant/50">
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-full bg-primary-container text-on-primary-container font-label-sm text-[11px] font-bold flex items-center justify-center">
                      SA
                    </div>
                    <div className="flex flex-col min-w-0">
                      <span className="font-body-sm text-body-sm text-on-surface font-medium truncate">
                        @sarah-architect
                      </span>
                      <span className="font-label-sm text-[10px] text-outline truncate">
                        Lead Platform Architect • Required
                      </span>
                    </div>
                  </div>
                  <span className="px-1.5 py-0.5 rounded bg-surface-container-high text-outline text-[10px] font-label-sm">
                    Awaiting
                  </span>
                </div>

                <div className="flex items-center justify-between p-2 rounded bg-surface-container border border-outline-variant/50">
                  <div className="flex items-center gap-2.5 min-w-0">
                    <div className="w-7 h-7 rounded-full bg-secondary-container/60 text-secondary font-label-sm text-[11px] font-bold flex items-center justify-center">
                      MD
                    </div>
                    <div className="flex flex-col min-w-0">
                      <span className="font-body-sm text-body-sm text-on-surface font-medium truncate">
                        @marcus-data
                      </span>
                      <span className="font-label-sm text-[10px] text-outline truncate">
                        Analytics Engineering Lead • {isBreaking ? 'Mandatory Gate' : 'Required'}
                      </span>
                    </div>
                  </div>
                  <span
                    className={`px-1.5 py-0.5 rounded text-[10px] font-label-sm ${
                      isBreaking
                        ? 'bg-error-container/30 text-error font-semibold'
                        : 'bg-surface-container-high text-outline'
                    }`}
                  >
                    {isBreaking ? 'Gatekeeper' : 'Awaiting'}
                  </span>
                </div>
              </div>

              {/* Local Pre-Flight Status */}
              <div className="p-2.5 rounded bg-[#090d15] border border-outline-variant/40 flex flex-col gap-1.5">
                <div className="flex items-center justify-between">
                  <span className="font-label-sm text-[11px] text-outline uppercase font-semibold">
                    Local Pre-Flight Engine
                  </span>
                  <span className="font-label-sm text-[11px] text-tertiary">3 / 3 OK</span>
                </div>
                <div className="w-full bg-surface-container rounded-full h-1 overflow-hidden">
                  <div className="bg-tertiary h-1 w-full rounded-full"></div>
                </div>
              </div>
            </div>
          </div>

          {/* Form Actions Footer */}
          <div className="flex items-center justify-between pt-2 border-t border-outline-variant/60">
            <button
              type="button"
              onClick={() => onNavigate(`/events/${eventId || 'orders.order.completed'}`)}
              className="px-3 py-1.5 rounded bg-surface-container hover:bg-surface-container-high text-on-surface-variant hover:text-on-surface font-body-sm text-body-sm transition-colors"
            >
              Cancel &amp; Discard
            </button>

            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => onShowToast('Draft saved to local browser storage', 'info')}
                className="px-3 py-1.5 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface font-body-sm text-body-sm transition-colors"
              >
                Save Local Draft
              </button>

              <button
                type="submit"
                className={`px-4 py-1.5 rounded font-body-sm text-body-sm font-semibold inline-flex items-center gap-1.5 transition-colors shadow-sm ${
                  isBreaking
                    ? 'bg-secondary text-on-secondary hover:bg-secondary/90'
                    : 'bg-tertiary-container text-on-tertiary hover:bg-tertiary'
                }`}
              >
                <span className="material-symbols-outlined text-[16px]">send</span>
                <span>
                  {isBreaking ? 'Submit Breaking Change Proposal' : 'Submit for Team Review'}
                </span>
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
};
