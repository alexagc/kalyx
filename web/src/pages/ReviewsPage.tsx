import React, { useState } from 'react';
import { Proposal } from '../types';
import { MOCK_PROPOSALS } from '../mockData';

interface ReviewsPageProps {
  onNavigate: (path: string) => void;
  onShowToast: (message: string, type?: 'success' | 'info' | 'error') => void;
}

export const ReviewsPage: React.FC<ReviewsPageProps> = ({ onNavigate, onShowToast }) => {
  const [proposals, setProposals] = useState<Proposal[]>(MOCK_PROPOSALS);
  const [selectedQueue, setSelectedQueue] = useState<'my_team' | 'data_steward'>('my_team');
  const [selectedProposal, setSelectedProposal] = useState<Proposal | null>(proposals[0]);
  const [reviewNotes, setReviewNotes] = useState<string>('');

  const filteredProposals = proposals.filter((p) => {
    if (selectedQueue === 'my_team') return !p.isBreaking;
    return p.isBreaking;
  });

  const handleApprove = (proposal: Proposal) => {
    setProposals((prev) =>
      prev.map((p) => (p.id === proposal.id ? { ...p, status: 'approved' } : p))
    );
    onShowToast(`Contract proposal ${proposal.id} approved and published to registry!`, 'success');
  };

  const handleReject = (proposal: Proposal) => {
    setProposals((prev) =>
      prev.map((p) => (p.id === proposal.id ? { ...p, status: 'rejected' } : p))
    );
    onShowToast(`Proposal ${proposal.id} marked as Changes Requested.`, 'error');
  };

  return (
    <div className="flex flex-col w-full pb-16">
      <div className="px-space-lg py-space-md flex flex-col gap-space-lg max-w-[1720px] mx-auto w-full">
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-space-md border-b border-outline-variant/40 pb-4">
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2 font-label-sm text-label-sm">
              <span className="text-primary uppercase tracking-wider">Governance</span>
              <span className="text-outline-variant">/</span>
              <span className="text-on-surface-variant">Review Queues</span>
            </div>
            <h1 className="font-headline-lg text-headline-lg text-on-surface font-semibold tracking-tight">
              Governance &amp; Approvals Dashboard
            </h1>
          </div>

          <div className="flex items-center gap-2">
            <span className="px-2.5 py-1 rounded bg-secondary-container/20 border border-secondary/30 text-secondary font-label-sm text-label-sm font-semibold">
              2 Pending Action Items
            </span>
          </div>
        </div>

        {/* Queue Selector Tabs */}
        <div className="flex items-center gap-2 border-b border-outline-variant">
          <button
            onClick={() => {
              setSelectedQueue('my_team');
              const found = proposals.find((p) => !p.isBreaking);
              if (found) setSelectedProposal(found);
            }}
            className={`px-4 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
              selectedQueue === 'my_team'
                ? 'border-primary text-primary'
                : 'border-transparent text-on-surface-variant hover:text-on-surface'
            }`}
          >
            <span className="material-symbols-outlined text-[17px]">groups</span>
            <span>Pending My Team (Checkout Core)</span>
            <span className="px-1.5 py-0.2 rounded-full bg-surface-container-high text-on-surface-variant font-label-sm text-[10px]">
              1
            </span>
          </button>

          <button
            onClick={() => {
              setSelectedQueue('data_steward');
              const found = proposals.find((p) => p.isBreaking);
              if (found) setSelectedProposal(found);
            }}
            className={`px-4 py-2.5 border-b-2 font-body-md text-body-md font-medium flex items-center gap-2 transition-colors ${
              selectedQueue === 'data_steward'
                ? 'border-secondary text-secondary'
                : 'border-transparent text-on-surface-variant hover:text-on-surface'
            }`}
          >
            <span className="material-symbols-outlined text-[17px]">shield</span>
            <span>Global Data Steward Queue (Breaking Changes)</span>
            <span className="px-1.5 py-0.2 rounded-full bg-secondary-container/30 text-secondary font-label-sm text-[10px]">
              1
            </span>
          </button>
        </div>

        {/* Proposals List and Detail Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
          {/* Left Column: Proposals List */}
          <div className="lg:col-span-5 flex flex-col gap-3">
            {filteredProposals.map((prop) => {
              const isSelected = selectedProposal?.id === prop.id;
              return (
                <div
                  key={prop.id}
                  onClick={() => setSelectedProposal(prop)}
                  className={`p-4 rounded-DEFAULT border cursor-pointer transition-all ${
                    isSelected
                      ? 'border-primary bg-surface-container shadow-md'
                      : 'border-outline-variant bg-surface-container-low hover:bg-surface-container'
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs text-primary font-bold">{prop.id}</span>
                        <span className="font-semibold text-on-surface text-body-md">
                          {prop.eventName}
                        </span>
                      </div>
                      <div className="text-outline text-body-sm mt-0.5 font-mono text-[11px]">
                        {prop.branch}
                      </div>
                    </div>

                    <span
                      className={`px-2 py-0.5 rounded font-label-sm text-[10px] uppercase font-bold ${
                        prop.isBreaking
                          ? 'bg-error-container/20 text-error border border-error/30'
                          : 'bg-tertiary/10 text-tertiary border border-tertiary/20'
                      }`}
                    >
                      {prop.isBreaking ? 'Major Breaking' : 'Minor Compatible'}
                    </span>
                  </div>

                  <p className="text-body-sm text-on-surface-variant line-clamp-2 mt-2">
                    {prop.justification}
                  </p>

                  <div className="flex items-center justify-between text-outline text-[11px] font-label-sm mt-3 pt-2 border-t border-outline-variant/40">
                    <span>
                      By: <strong className="text-on-surface">{prop.author}</strong> ({prop.team})
                    </span>
                    <span>
                      {prop.baseVersion} ➔{' '}
                      <strong className="text-primary">{prop.proposedVersion}</strong>
                    </span>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Right Column: Detailed Proposal Review & Resolution */}
          {selectedProposal && (
            <div className="lg:col-span-7 flex flex-col gap-4 p-5 rounded-DEFAULT border border-outline-variant bg-surface-container-low">
              <div className="flex items-start justify-between border-b border-outline-variant/60 pb-3">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-sm text-primary font-bold">
                      {selectedProposal.id}
                    </span>
                    <h2 className="font-headline-md text-headline-md text-on-surface font-semibold">
                      {selectedProposal.eventName}
                    </h2>
                  </div>
                  <div className="flex items-center gap-2 text-outline font-label-sm text-[11px] mt-1">
                    <span>Target: main ({selectedProposal.baseVersion})</span>
                    <span>•</span>
                    <span>Proposed: {selectedProposal.proposedVersion}</span>
                    <span>•</span>
                    <span>Author: {selectedProposal.author}</span>
                  </div>
                </div>

                <button
                  onClick={() =>
                    onNavigate(`/events/${selectedProposal.eventId}/propose`)
                  }
                  className="h-8 px-3 rounded bg-surface-container-high hover:bg-surface-bright text-primary border border-outline-variant font-label-sm text-label-sm inline-flex items-center gap-1.5 transition-colors"
                >
                  <span className="material-symbols-outlined text-[16px]">compare</span>
                  <span>Inspect Diff &amp; Schema</span>
                </button>
              </div>

              {/* Breaking Warning if applicable */}
              {selectedProposal.isBreaking && (
                <div className="p-3 rounded bg-error-container/20 border border-error/40 flex items-start gap-2 text-error">
                  <span className="material-symbols-outlined text-[20px] mt-0.5">warning</span>
                  <div className="text-body-sm">
                    <strong className="font-semibold">Breaking Change Requires Data Steward Approval:</strong>
                    <ul className="list-disc list-inside mt-1 space-y-0.5 text-xs text-on-surface">
                      {selectedProposal.breakingReasons.map((r, i) => (
                        <li key={i}>{r}</li>
                      ))}
                    </ul>
                  </div>
                </div>
              )}

              {/* Justification */}
              <div>
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Engineering Justification
                </span>
                <p className="text-body-sm text-on-surface bg-surface-container-lowest p-3 rounded mt-1 leading-relaxed border border-outline-variant/50">
                  {selectedProposal.justification}
                </p>
              </div>

              {/* Diff Statistics */}
              <div>
                <span className="font-label-sm text-label-sm text-outline uppercase tracking-wider">
                  Diff Impact Summary
                </span>
                <div className="grid grid-cols-3 gap-3 mt-1.5">
                  <div className="p-2.5 rounded bg-surface-container border border-outline-variant/50 text-center">
                    <div className="text-tertiary font-bold font-mono text-base">
                      +{selectedProposal.diffSummary.linesAdded}
                    </div>
                    <div className="text-outline text-[11px] font-label-sm">Lines Added</div>
                  </div>
                  <div className="p-2.5 rounded bg-surface-container border border-outline-variant/50 text-center">
                    <div className="text-error font-bold font-mono text-base">
                      -{selectedProposal.diffSummary.linesDeleted}
                    </div>
                    <div className="text-outline text-[11px] font-label-sm">Lines Deleted</div>
                  </div>
                  <div className="p-2.5 rounded bg-surface-container border border-outline-variant/50 text-center">
                    <div className="text-primary font-bold font-mono text-base">
                      {selectedProposal.diffSummary.fieldsAdded}
                    </div>
                    <div className="text-outline text-[11px] font-label-sm">Fields Added</div>
                  </div>
                </div>
              </div>

              {/* Reviewer Resolution Form */}
              <div className="flex flex-col gap-2 pt-2 border-t border-outline-variant/60">
                <label className="font-label-sm text-label-sm text-on-surface font-medium uppercase tracking-wider">
                  Reviewer Feedback / Audit Notes
                </label>
                <textarea
                  value={reviewNotes}
                  onChange={(e) => setReviewNotes(e.target.value)}
                  placeholder="Add approval audit note or state requested alterations..."
                  rows={3}
                  className="w-full p-2.5 bg-[#0a0e16] border border-outline-variant rounded text-on-surface font-body-sm text-body-sm focus:outline-none focus:border-primary transition-colors"
                />

                <div className="flex items-center justify-end gap-3 mt-2">
                  <button
                    type="button"
                    onClick={() => handleReject(selectedProposal)}
                    className="px-3.5 py-1.5 rounded bg-error-container/20 hover:bg-error-container/30 text-error border border-error/30 font-body-sm text-body-sm font-medium transition-colors"
                  >
                    Reject / Request Changes
                  </button>

                  <button
                    type="button"
                    onClick={() => handleApprove(selectedProposal)}
                    className="px-4 py-1.5 rounded bg-tertiary-container hover:bg-tertiary text-on-tertiary font-body-sm text-body-sm font-semibold inline-flex items-center gap-1.5 transition-colors shadow-sm"
                  >
                    <span className="material-symbols-outlined text-[16px]">check_circle</span>
                    <span>Approve &amp; Publish Contract</span>
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
