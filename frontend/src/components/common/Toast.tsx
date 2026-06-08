// Toast — minimal non-blocking notification, 3s auto-dismiss.
import { useState, useEffect, useCallback, createContext, useContext, ReactNode } from 'react';

interface ToastItem { id: number; message: string; tone: 'info' | 'success' | 'error' }
interface ToastCtx { push: (message: string, tone?: ToastItem['tone']) => void }

const Ctx = createContext<ToastCtx>({ push: () => {} });

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const push = useCallback((message: string, tone: ToastItem['tone'] = 'info') => {
    const id = Date.now() + Math.random();
    setItems((cur) => [...cur, { id, message, tone }]);
    setTimeout(() => setItems((cur) => cur.filter((i) => i.id !== id)), 3000);
  }, []);
  return (
    <Ctx.Provider value={{ push }}>
      {children}
      <div style={{ position: 'fixed', top: 16, right: 16, zIndex: 2000, display: 'flex', flexDirection: 'column', gap: 8 }}>
        {items.map((t) => (
          <div
            key={t.id}
            style={{
              background: 'var(--color-surface-elevated)',
              border: '1px solid var(--color-border)',
              borderLeft: `3px solid ${t.tone === 'error' ? 'var(--color-error)' : t.tone === 'success' ? 'var(--color-success)' : 'var(--color-primary)'}`,
              borderRadius: 'var(--radius-md)',
              padding: 'var(--sp-3) var(--sp-4)',
              color: 'var(--color-text)',
              fontSize: 'var(--fs-small)',
              boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
              maxWidth: 400,
            }}
          >{t.message}</div>
        ))}
      </div>
    </Ctx.Provider>
  );
}

export function useToast() { return useContext(Ctx); }
