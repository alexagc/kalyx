import React, { useState } from 'react';
import { PATToken } from '../types';
import { MOCK_PAT_TOKENS, MOCK_TEAM_MAPPINGS } from '../mockData';

interface SettingsPageProps {
  onNavigate: (path: string) => void;
  onShowToast: (message: string, type?: 'success' | 'info' | 'error') => void;
}

export const SettingsPage: React.FC<SettingsPageProps> = ({ onShowToast }) => {
  const [tokens, setTokens] = useState<PATToken[]>(MOCK_PAT_TOKENS);
  const [newTokenModalOpen, setNewTokenModalOpen] = useState(false);
  const [tokenName, setTokenName] = useState('');
  const [generatedToken, setGeneratedToken] = useState<string | null>(null);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard?.writeText(text);
    onShowToast(`Copied ${label} to clipboard`, 'success');
  };

  const handleGenerateToken = (e: React.FormEvent) => {
    e.preventDefault();
    if (!tokenName.trim()) {
      onShowToast('Please provide a name for the token', 'error');
      return;
    }

    const randomSecret = Array.from({ length: 32 }, () =>
      Math.floor(Math.random() * 16).toString(16)
    ).join('');
    const fullToken = `kalyx_sec_${randomSecret}`;

    const newToken: PATToken = {
      id: `pat-00${tokens.length + 1}`,
      name: tokenName.trim(),
      token_preview: `kalyx_sec_${randomSecret.slice(0, 4)}...${randomSecret.slice(-4)}`,
      created_at: new Date().toISOString(),
      last_used_at: 'Never',
      scopes: ['events:read', 'events:validate'],
    };

    setTokens([newToken, ...tokens]);
    setGeneratedToken(fullToken);
    setTokenName('');
    onShowToast('New PAT token generated successfully!', 'success');
  };

  const handleRevokeToken = (id: string) => {
    setTokens(tokens.filter((t) => t.id !== id));
    onShowToast('Access token revoked.', 'info');
  };

  return (
    <div className="flex flex-col w-full pb-16">
      <div className="px-space-lg py-space-md flex flex-col gap-space-lg max-w-[1720px] mx-auto w-full">
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-space-md border-b border-outline-variant/40 pb-4">
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2 font-label-sm text-label-sm">
              <span className="text-primary uppercase tracking-wider">Settings</span>
              <span className="text-outline-variant">/</span>
              <span className="text-on-surface-variant">Access Tokens &amp; Teams</span>
            </div>
            <h1 className="font-headline-lg text-headline-lg text-on-surface font-semibold tracking-tight">
              Access Tokens &amp; Settings
            </h1>
          </div>
        </div>

        {/* 1. PERSONAL ACCESS TOKENS (PAT) */}
        <div className="p-5 rounded-DEFAULT border border-outline-variant bg-surface-container-low flex flex-col gap-4">
          <div className="flex items-center justify-between border-b border-outline-variant/60 pb-3">
            <div className="flex items-center gap-2">
              <span className="material-symbols-outlined text-primary text-[20px]">key</span>
              <div>
                <h2 className="font-headline-md text-headline-md text-on-surface font-semibold">
                  Personal Access Tokens (PAT)
                </h2>
                <p className="text-body-sm text-outline">
                  Service tokens for CI/CD pipelines, kalyxctl CLI authentication, and automated schema validation.
                </p>
              </div>
            </div>

            <button
              onClick={() => {
                setGeneratedToken(null);
                setNewTokenModalOpen(true);
              }}
              className="h-8 px-3 rounded bg-tertiary-container hover:bg-tertiary text-on-tertiary font-body-sm text-body-sm font-semibold inline-flex items-center gap-1.5 transition-colors shadow-sm"
            >
              <span className="material-symbols-outlined text-[16px]">add</span>
              <span>Generate New Token</span>
            </button>
          </div>

          {/* Tokens Table */}
          <div className="overflow-x-auto rounded border border-outline-variant/60 bg-surface-container-lowest">
            <table className="w-full text-left border-collapse text-body-sm">
              <thead>
                <tr className="h-9 bg-surface-container border-b border-outline-variant font-label-sm text-outline uppercase tracking-wider">
                  <th className="px-4 py-1">Token Name &amp; Description</th>
                  <th className="px-4 py-1">Secret Preview</th>
                  <th className="px-4 py-1">Created</th>
                  <th className="px-4 py-1">Last Used</th>
                  <th className="px-4 py-1 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-outline-variant/30 text-on-surface">
                {tokens.map((token) => (
                  <tr key={token.id} className="h-11 hover:bg-surface-container/60 transition-colors">
                    <td className="px-4 py-2 font-medium">
                      <div className="flex items-center gap-2">
                        <span className="material-symbols-outlined text-outline text-[16px]">
                          token
                        </span>
                        <span>{token.name}</span>
                      </div>
                    </td>
                    <td className="px-4 py-2">
                      <code className="px-2 py-0.5 rounded bg-surface-container text-primary font-mono text-xs">
                        {token.token_preview}
                      </code>
                    </td>
                    <td className="px-4 py-2 text-outline text-xs font-label-sm">
                      {token.created_at.slice(0, 10)}
                    </td>
                    <td className="px-4 py-2 text-outline text-xs font-label-sm">
                      {token.last_used_at}
                    </td>
                    <td className="px-4 py-2 text-right">
                      <button
                        onClick={() => handleRevokeToken(token.id)}
                        className="px-2.5 py-1 rounded hover:bg-error-container/20 text-error text-xs font-label-sm transition-colors"
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* 2. OIDC TEAM MAPPINGS */}
        <div className="p-5 rounded-DEFAULT border border-outline-variant bg-surface-container-low flex flex-col gap-4">
          <div className="flex items-center gap-2 border-b border-outline-variant/60 pb-3">
            <span className="material-symbols-outlined text-secondary text-[20px]">
              badge
            </span>
            <div>
              <h2 className="font-headline-md text-headline-md text-on-surface font-semibold">
                OIDC Claim Group Mappings &amp; RBAC
              </h2>
              <p className="text-body-sm text-outline">
                Teams and permission roles automatically resolved from your Corporate Identity Provider (Okta / Google Workspace / Azure AD).
              </p>
            </div>
          </div>

          <div className="overflow-x-auto rounded border border-outline-variant/60 bg-surface-container-lowest">
            <table className="w-full text-left border-collapse text-body-sm">
              <thead>
                <tr className="h-9 bg-surface-container border-b border-outline-variant font-label-sm text-outline uppercase tracking-wider">
                  <th className="px-4 py-1">IdP Claim Group (JWT 'groups')</th>
                  <th className="px-4 py-1">Kalyx Team</th>
                  <th className="px-4 py-1">Role &amp; Permissions</th>
                  <th className="px-4 py-1">Active Members</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-outline-variant/30 text-on-surface">
                {MOCK_TEAM_MAPPINGS.map((tm) => (
                  <tr key={tm.idp_group} className="h-11 hover:bg-surface-container/60 transition-colors">
                    <td className="px-4 py-2 font-mono text-xs text-primary">
                      {tm.idp_group}
                    </td>
                    <td className="px-4 py-2 font-medium">{tm.kalyx_team}</td>
                    <td className="px-4 py-2">
                      <span
                        className={`px-2 py-0.5 rounded text-xs font-label-sm ${
                          tm.role.includes('Gatekeeper')
                            ? 'bg-secondary-container/20 text-secondary border border-secondary/30'
                            : 'bg-surface-container text-on-surface-variant'
                        }`}
                      >
                        {tm.role}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-outline text-xs font-label-sm">
                      {tm.members_count} engineers
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* MODAL: GENERATE NEW PAT */}
      {newTokenModalOpen && (
        <div
          className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4"
          onClick={() => setNewTokenModalOpen(false)}
        >
          <div
            className="w-full max-w-lg rounded-DEFAULT bg-surface-container-low border border-outline-variant shadow-2xl p-5 flex flex-col gap-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-outline-variant/60 pb-3">
              <h3 className="font-headline-md text-headline-md text-on-surface font-semibold flex items-center gap-2">
                <span className="material-symbols-outlined text-primary">key</span>
                <span>Generate Personal Access Token</span>
              </h3>
              <button
                onClick={() => setNewTokenModalOpen(false)}
                className="text-outline hover:text-on-surface"
              >
                <span className="material-symbols-outlined text-[18px]">close</span>
              </button>
            </div>

            {generatedToken ? (
              <div className="flex flex-col gap-3">
                <div className="p-3 rounded bg-amber-950/30 border border-amber-800/60 text-amber-300 text-body-sm">
                  <strong>Important:</strong> Copy this secret now. It will not be displayed again.
                </div>

                <div className="flex items-center justify-between p-3 rounded bg-[#070a0f] border border-outline-variant font-mono text-xs text-primary break-all">
                  <span>{generatedToken}</span>
                  <button
                    onClick={() => copyToClipboard(generatedToken, 'Personal Access Token')}
                    className="ml-3 px-2 py-1 rounded bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface text-xs shrink-0"
                  >
                    Copy
                  </button>
                </div>

                <button
                  onClick={() => setNewTokenModalOpen(false)}
                  className="w-full mt-2 h-8 rounded bg-primary-container text-on-primary font-body-sm font-semibold hover:bg-primary transition-colors"
                >
                  Done
                </button>
              </div>
            ) : (
              <form onSubmit={handleGenerateToken} className="flex flex-col gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-label-sm font-label-sm text-outline uppercase tracking-wider">
                    Token Name / Service
                  </label>
                  <input
                    type="text"
                    required
                    autoFocus
                    value={tokenName}
                    onChange={(e) => setTokenName(e.target.value)}
                    placeholder="e.g., GitLab CI - Payments Worker"
                    className="h-8 px-3 rounded bg-surface-container-lowest border border-outline-variant text-on-surface font-body-sm focus:outline-none focus:border-primary"
                  />
                </div>

                <div className="flex flex-col gap-1">
                  <span className="text-label-sm font-label-sm text-outline uppercase tracking-wider">
                    Scopes Granted
                  </span>
                  <div className="flex flex-col gap-1.5 p-2 rounded bg-surface-container-lowest border border-outline-variant text-xs">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="checkbox" defaultChecked disabled className="rounded text-primary" />
                      <span>events:read (Discover contracts and fetch JSON schemas)</span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="checkbox" defaultChecked disabled className="rounded text-primary" />
                      <span>events:validate (Validate live CloudEvents against schemas)</span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input type="checkbox" defaultChecked disabled className="rounded text-primary" />
                      <span>proposals:write (Submit new schema proposal branches)</span>
                    </label>
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2 mt-2 pt-2 border-t border-outline-variant/60">
                  <button
                    type="button"
                    onClick={() => setNewTokenModalOpen(false)}
                    className="px-3 py-1.5 rounded bg-surface-container hover:bg-surface-container-high text-on-surface-variant text-body-sm"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-1.5 rounded bg-tertiary-container hover:bg-tertiary text-on-tertiary font-body-sm font-semibold shadow-sm"
                  >
                    Generate Token
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
