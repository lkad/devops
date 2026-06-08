// AppShell — top-level layout: Header + SideNav + main slot.
import { Outlet, useNavigate } from 'react-router-dom';
import { useEffect } from 'react';
import { Header } from './Header';
import { SideNav } from './SideNav';
import { useAuth } from '../../stores/auth';

export function AppShell() {
  const user = useAuth((s) => s.user);
  const hydrated = useAuth((s) => s.hydrated);
  const restore = useAuth((s) => s.restore);
  const nav = useNavigate();

  useEffect(() => { restore(); }, [restore]);

  // Only redirect to /login AFTER restore has read the JWT. The
  // restore is asynchronous (zustand setState is async), so without
  // the hydrated gate a full page reload bounces to /login even when
  // a valid JWT is in localStorage.
  useEffect(() => {
    if (hydrated && !user) nav('/login', { replace: true });
  }, [hydrated, user, nav]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <Header />
      <div style={{ display: 'flex', flex: 1, minHeight: 0 }}>
        <SideNav />
        <main style={{ flex: 1, overflow: 'auto', padding: 'var(--sp-6)' }}>
          <div style={{ maxWidth: 'var(--main-max)', margin: '0 auto' }}>
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
