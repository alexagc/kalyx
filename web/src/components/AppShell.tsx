import React, { useState, useEffect } from 'react';

interface AppShellProps {
  currentPath: string;
  onNavigate: (path: string) => void;
  children: React.ReactNode;
  onProposeClick?: () => void;
}

export const AppShell: React.FC<AppShellProps> = ({
  currentPath,
  onNavigate,
  children,
  onProposeClick,
}) => {
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  // Keyboard shortcut listener for Cmd+K / Ctrl+K
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setSearchOpen((prev) => !prev);
      }
      if (e.key === 'Escape') {
        setSearchOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const navItems = [
    {
      name: 'Event Contracts',
      path: '/events',
      icon: 'schema',
    },
    {
      name: 'Governance & Reviews',
      path: '/reviews',
      icon: 'verified_user',
      badge: '2 pending',
    },
    {
      name: 'Export & DDL Center',
      path: '/export',
      icon: 'sync_alt',
    },
    {
      name: 'Access Tokens & Settings',
      path: '/settings',
      icon: 'key',
    },
  ];

  // Helper to determine breadcrumb title
  const getBreadcrumbTitle = () => {
    if (currentPath.startsWith('/events/') && currentPath.endsWith('/propose')) {
      return 'Proposals / Schema Diff';
    }
    if (currentPath.startsWith('/events/')) {
      return 'Contract Detail & Workbench';
    }
    if (currentPath === '/reviews') {
      return 'Governance & Approvals';
    }
    if (currentPath === '/export') {
      return 'Export Center & DDL';
    }
    if (currentPath === '/settings') {
      return 'Access Tokens & Settings';
    }
    return 'Contracts Catalog';
  };

  return (
    <div className="bg-surface font-body-md text-body-md text-on-surface antialiased min-h-screen">
      {/* 1. FIXED LEFT SIDEBAR */}
      <aside className="fixed left-0 top-0 h-screen w-60 bg-surface-container-lowest border-r border-outline-variant z-50 flex flex-col justify-between select-none">
        <div className="flex flex-col">
          {/* Brand Header */}
          <div className="h-14 px-space-md flex items-center justify-between border-b border-outline-variant">
            <div
              className="flex items-center gap-space-sm cursor-pointer"
              onClick={() => onNavigate('/events')}
            >
              <span className="text-2xl" role="img" aria-label="Kalyx logo">🌸</span>
              <span className="font-headline-md text-headline-md text-on-surface tracking-tight font-semibold">
                Kalyx
              </span>
            </div>
            <span className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container-high border border-outline-variant text-on-surface-variant font-label-sm text-label-sm">
              v1.0
            </span>
          </div>

          {/* SQLite WAL Engine Status Badge */}
          <div className="px-space-md py-space-sm border-b border-outline-variant/60 bg-surface-container-lowest">
            <div className="inline-flex items-center gap-1.5 px-2 py-1 rounded bg-surface-container-low border border-outline-variant/50 w-full">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-tertiary opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-tertiary-container"></span>
              </span>
              <span className="font-label-sm text-label-sm text-on-surface font-medium truncate">
                SQLite WAL Active
              </span>
            </div>
          </div>

          {/* Navigation Links */}
          <div className="px-space-xs py-space-sm">
            <div className="px-space-sm py-1 font-label-sm text-label-sm text-outline uppercase tracking-wider">
              Registry Engine
            </div>
            <nav className="flex flex-col gap-0.5 mt-1">
              {navItems.map((item) => {
                const isActive =
                  currentPath === item.path ||
                  (item.path === '/events' && currentPath.startsWith('/events'));
                return (
                  <button
                    key={item.path}
                    onClick={() => onNavigate(item.path)}
                    className={`flex items-center justify-between px-space-sm py-2 rounded-r-DEFAULT transition-colors text-left w-full ${
                      isActive
                        ? 'bg-surface-container-high text-primary border-l-2 border-primary-container font-medium'
                        : 'text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <span className="material-symbols-outlined text-[18px]">
                        {item.icon}
                      </span>
                      <span className="font-body-md text-body-md">{item.name}</span>
                    </div>
                    {item.badge && (
                      <span className="px-1.5 py-0.2 rounded bg-secondary-container/20 border border-secondary/30 text-secondary font-label-sm text-label-sm">
                        {item.badge}
                      </span>
                    )}
                  </button>
                );
              })}
            </nav>
          </div>
        </div>

        {/* User Footer Card (Alex Rivera / Checkout Core) */}
        <div className="p-space-sm border-t border-outline-variant bg-surface-container-lowest">
          <div className="p-space-xs rounded-DEFAULT bg-surface-container-low border border-outline-variant flex items-center justify-between gap-2">
            <div className="flex items-center gap-2 min-w-0">
              <div className="w-8 h-8 rounded-full bg-primary flex-shrink-0 flex items-center justify-center text-on-primary font-bold text-xs">
                AR
              </div>
              <div className="min-w-0 flex flex-col">
                <div className="flex items-center gap-1.5">
                  <span className="font-body-md text-body-md text-on-surface font-medium truncate leading-tight">
                    Alex Rivera
                  </span>
                </div>
                <span className="font-label-sm text-label-sm text-outline truncate leading-tight">
                  alex@company.com
                </span>
                <span className="mt-0.5 inline-block font-label-sm text-[10px] text-primary-fixed-dim bg-surface-container px-1 rounded border border-outline-variant/60 w-fit leading-none py-0.5">
                  Checkout Core
                </span>
              </div>
            </div>
            <button
              onClick={() => onNavigate('/settings')}
              className="p-1 text-on-surface-variant hover:text-on-surface rounded hover:bg-surface-container transition-colors"
              title="Session Settings"
            >
              <span className="material-symbols-outlined text-[16px]">more_vert</span>
            </button>
          </div>
        </div>
      </aside>

      {/* 2. MAIN LAYOUT WITH STICKY TOP HEADER */}
      <div className="pl-60 min-h-screen flex flex-col bg-surface">
        <header className="fixed top-0 left-60 right-0 h-14 bg-surface-container-lowest/95 backdrop-blur-md border-b border-outline-variant z-40 px-space-lg flex items-center justify-between gap-space-md">
          {/* Breadcrumb Header */}
          <div className="flex items-center gap-space-sm">
            <span
              className="cursor-pointer text-xl"
              onClick={() => onNavigate('/events')}
              title="Kalyx Home"
            >
              🌸
            </span>
            <span
              className="font-body-md text-body-md text-outline cursor-pointer hover:text-on-surface transition-colors"
              onClick={() => onNavigate('/events')}
            >
              Kalyx
            </span>
            <span className="text-outline-variant">/</span>
            <span className="font-body-md text-body-md text-on-surface font-medium">
              {getBreadcrumbTitle()}
            </span>
          </div>

          {/* Centered Global Search Input (Cmd+K) */}
          <div className="flex-1 max-w-lg mx-space-md">
            <div
              className="relative flex items-center w-full cursor-pointer"
              onClick={() => setSearchOpen(true)}
            >
              <span className="material-symbols-outlined absolute left-2.5 text-outline text-[16px]">
                search
              </span>
              <input
                className="w-full h-8 pl-8 pr-12 rounded-DEFAULT bg-surface-container-low border border-outline-variant text-on-surface placeholder:text-outline font-body-sm text-body-sm focus:outline-none focus:border-primary transition-colors cursor-pointer"
                placeholder="Search contracts, schemas, fields, CE types..."
                type="text"
                readOnly
                value={searchQuery}
              />
              <div className="absolute right-2 flex items-center gap-0.5 pointer-events-none">
                <kbd className="px-1.5 py-0.5 rounded-DEFAULT bg-surface-container border border-outline-variant font-label-sm text-[10px] text-on-surface-variant">
                  ⌘K
                </kbd>
              </div>
            </div>
          </div>

          {/* Right Header Navigation & CTAs */}
          <div className="flex items-center gap-space-md">
            <a
              className="flex items-center gap-1 font-body-sm text-body-sm text-on-surface-variant hover:text-primary transition-colors"
              href="https://github.com/alexagc/kalyx"
              target="_blank"
              rel="noreferrer"
            >
              <span className="material-symbols-outlined text-[16px]">terminal</span>
              <span>CLI Docs</span>
              <span className="material-symbols-outlined text-[12px]">north_east</span>
            </a>

            <div className="inline-flex items-center gap-1.5 px-2 py-1 rounded bg-surface-container border border-outline-variant">
              <span className="w-1.5 h-1.5 rounded-full bg-tertiary"></span>
              <span className="font-label-sm text-label-sm text-on-surface-variant">4ms ping</span>
            </div>

            <button
              onClick={() => {
                if (onProposeClick) {
                  onProposeClick();
                } else {
                  onNavigate('/events/orders.order.completed/propose');
                }
              }}
              className="h-8 px-space-md rounded-DEFAULT bg-tertiary-container text-on-tertiary hover:bg-tertiary font-body-sm text-body-sm font-semibold inline-flex items-center gap-1.5 transition-colors shadow-sm"
            >
              <span className="material-symbols-outlined text-[16px]">add</span>
              <span>Propose Event</span>
            </button>

            <button
              onClick={() => onNavigate('/settings')}
              className="w-8 h-8 rounded-full bg-primary flex items-center justify-center text-on-primary font-bold text-xs"
              title="Alex Rivera (Checkout Core)"
            >
              AR
            </button>
          </div>
        </header>

        {/* 3. MAIN CONTENT CONTAINER */}
        <main className="w-full pt-14 bg-surface min-h-[calc(100vh-3.5rem)]">
          {children}
        </main>
      </div>

      {/* 4. MODAL: COMMAND PALETTE (CMD+K) */}
      {searchOpen && (
        <div
          className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-start justify-center pt-20 px-4"
          onClick={() => setSearchOpen(false)}
        >
          <div
            className="w-full max-w-xl rounded-DEFAULT bg-surface-container-low border border-outline-variant shadow-2xl overflow-hidden flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-3 border-b border-outline-variant flex items-center gap-2 bg-surface-container-lowest">
              <span className="material-symbols-outlined text-outline text-[18px]">search</span>
              <input
                autoFocus
                type="text"
                className="w-full bg-transparent text-on-surface placeholder:text-outline font-body-md text-body-md focus:outline-none"
                placeholder="Search contracts, CloudEvent types, schemas, or actions..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              <span className="font-label-sm text-[11px] text-outline px-1.5 py-0.5 rounded bg-surface-container border border-outline-variant">
                ESC
              </span>
            </div>

            <div className="p-2 flex flex-col gap-1 max-h-80 overflow-y-auto">
              <div className="px-2 py-1 text-[10px] uppercase font-semibold text-outline tracking-wider">
                Event Contracts
              </div>

              {[
                { name: 'OrderCompleted', type: 'orders.order.completed.v1', domain: 'Orders' },
                { name: 'PaymentProcessed', type: 'payments.payment.processed.v1', domain: 'Payments' },
                { name: 'CartAbandoned', type: 'orders.cart.abandoned.v1', domain: 'Orders' },
                { name: 'UserRegistered', type: 'users.user.registered.v1', domain: 'Users' },
                { name: 'InventoryReserved', type: 'inventory.item.reserved.v1', domain: 'Inventory' },
              ]
                .filter(
                  (c) =>
                    !searchQuery ||
                    c.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                    c.type.toLowerCase().includes(searchQuery.toLowerCase()) ||
                    c.domain.toLowerCase().includes(searchQuery.toLowerCase())
                )
                .map((contract) => (
                  <button
                    key={contract.type}
                    onClick={() => {
                      setSearchOpen(false);
                      onNavigate(`/events/${contract.name.toLowerCase()}`);
                    }}
                    className="flex items-center justify-between p-2 rounded hover:bg-surface-container text-left transition-colors"
                  >
                    <div className="flex items-center gap-2">
                      <span className="material-symbols-outlined text-primary text-[18px]">
                        data_object
                      </span>
                      <div>
                        <div className="font-medium text-on-surface text-body-sm">
                          {contract.name}
                        </div>
                        <div className="font-mono text-outline text-[11px]">
                          {contract.type}
                        </div>
                      </div>
                    </div>
                    <span className="px-1.5 py-0.5 rounded bg-surface-container-high text-on-surface-variant text-[10px] font-label-sm">
                      {contract.domain}
                    </span>
                  </button>
                ))}

              <div className="border-t border-outline-variant/60 my-1 pt-1">
                <div className="px-2 py-1 text-[10px] uppercase font-semibold text-outline tracking-wider">
                  Quick Actions
                </div>
                <button
                  onClick={() => {
                    setSearchOpen(false);
                    onNavigate('/reviews');
                  }}
                  className="w-full flex items-center gap-2 p-2 rounded hover:bg-surface-container text-left text-body-sm text-on-surface transition-colors"
                >
                  <span className="material-symbols-outlined text-secondary text-[16px]">
                    pending_actions
                  </span>
                  <span>View Pending Governance Approvals (2)</span>
                </button>
                <button
                  onClick={() => {
                    setSearchOpen(false);
                    onNavigate('/export');
                  }}
                  className="w-full flex items-center gap-2 p-2 rounded hover:bg-surface-container text-left text-body-sm text-on-surface transition-colors"
                >
                  <span className="material-symbols-outlined text-primary text-[16px]">
                    sync_alt
                  </span>
                  <span>Export Apache Iceberg & Delta Lake DDL</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
