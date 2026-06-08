// Header — 56px top bar. Brand on the left, user info on the right.
import { useAuth } from '../../stores/auth';
import { useNavigate } from 'react-router-dom';

export function Header() {
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);
  const nav = useNavigate();

  const onLogout = () => {
    logout();
    nav('/login');
  };

  return (
    <header
      style={{
        height: 'var(--header-h)',
        background: 'var(--color-surface)',
        borderBottom: '1px solid var(--color-border)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 var(--sp-6)',
        flexShrink: 0,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-3)' }}>
        <div
          style={{
            width: 28, height: 28, borderRadius: 'var(--radius-md)',
            background: 'linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            color: 'var(--color-bg)', fontWeight: 700, fontSize: 14,
          }}
        >D</div>
        <span style={{ fontWeight: 600, fontSize: 'var(--fs-h3)' }}>DevOps Toolkit</span>
        <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-caption)', marginLeft: 8 }}>
          foundation
        </span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-4)' }}>
        {user && (
          <>
            <span style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--fs-small)' }}>
              {user.username}
            </span>
            <span
              className="mono"
              style={{
                fontSize: 'var(--fs-caption)',
                color: 'var(--color-primary)',
                background: 'var(--color-primary-muted)',
                padding: '2px 8px',
                borderRadius: 'var(--radius-sm)',
              }}
            >{user.role}</span>
            <button
              onClick={onLogout}
              style={{
                background: 'transparent',
                border: '1px solid var(--color-border)',
                color: 'var(--color-text-secondary)',
                padding: '4px 12px',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                fontSize: 'var(--fs-small)',
              }}
            >Logout</button>
          </>
        )}
      </div>
    </header>
  );
}
