import React, { useState, useEffect } from 'react';
import { AppShell } from './components/AppShell';
import { CatalogPage } from './pages/CatalogPage';
import { ContractDetailPage } from './pages/ContractDetailPage';
import { ProposalEditorPage } from './pages/ProposalEditorPage';
import { ReviewsPage } from './pages/ReviewsPage';
import { ExportPage } from './pages/ExportPage';
import { SettingsPage } from './pages/SettingsPage';

interface Toast {
  id: string;
  message: string;
  type: 'success' | 'info' | 'error';
}

export const App: React.FC = () => {
  // Simple client-side routing based on window.location
  const [currentPath, setCurrentPath] = useState<string>(() => {
    const p = window.location.pathname;
    return p && p !== '/' ? p : '/events';
  });

  const [toasts, setToasts] = useState<Toast[]>([]);

  useEffect(() => {
    const handlePopState = () => {
      const p = window.location.pathname;
      setCurrentPath(p && p !== '/' ? p : '/events');
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const navigate = (path: string) => {
    window.history.pushState({}, '', path);
    setCurrentPath(path);
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const showToast = (message: string, type: 'success' | 'info' | 'error' = 'info') => {
    const id = `${Date.now()}-${Math.random()}`;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 3500);
  };

  // Router dispatcher
  const renderView = () => {
    if (currentPath === '/reviews') {
      return <ReviewsPage onNavigate={navigate} onShowToast={showToast} />;
    }
    if (currentPath === '/export') {
      return <ExportPage onNavigate={navigate} onShowToast={showToast} />;
    }
    if (currentPath === '/settings') {
      return <SettingsPage onNavigate={navigate} onShowToast={showToast} />;
    }
    if (currentPath.startsWith('/events/') && currentPath.endsWith('/propose')) {
      const parts = currentPath.split('/');
      const eventId = parts[2] || 'orders.order.completed';
      return (
        <ProposalEditorPage
          eventId={eventId}
          onNavigate={navigate}
          onShowToast={showToast}
        />
      );
    }
    if (currentPath.startsWith('/events/') && currentPath !== '/events') {
      const parts = currentPath.split('/');
      const eventId = parts[2] || 'orders.order.completed';
      return (
        <ContractDetailPage
          eventId={eventId}
          onNavigate={navigate}
          onShowToast={showToast}
        />
      );
    }

    // Default: CatalogPage
    return <CatalogPage onNavigate={navigate} onShowToast={showToast} />;
  };

  return (
    <AppShell
      currentPath={currentPath}
      onNavigate={navigate}
      onProposeClick={() => navigate('/events/orders.order.completed/propose')}
    >
      {renderView()}

      {/* Toast Notification Container */}
      <div className="fixed bottom-5 right-5 z-50 flex flex-col gap-2 max-w-sm pointer-events-none">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`pointer-events-auto flex items-center gap-2.5 px-3.5 py-2.5 rounded-DEFAULT shadow-xl border text-body-sm transition-all animate-fadeIn ${
              toast.type === 'success'
                ? 'bg-surface-container border-tertiary/40 text-on-surface'
                : toast.type === 'error'
                ? 'bg-surface-container border-error/50 text-error'
                : 'bg-surface-container border-primary/40 text-on-surface'
            }`}
          >
            <span
              className={`material-symbols-outlined text-[18px] ${
                toast.type === 'success'
                  ? 'text-tertiary'
                  : toast.type === 'error'
                  ? 'text-error'
                  : 'text-primary'
              }`}
            >
              {toast.type === 'success'
                ? 'check_circle'
                : toast.type === 'error'
                ? 'error'
                : 'info'}
            </span>
            <span className="font-medium text-xs leading-relaxed">{toast.message}</span>
          </div>
        ))}
      </div>
    </AppShell>
  );
};
