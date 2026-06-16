// Header — 56px top bar. Brand on the left, user info on the right.
import { useTranslation } from 'react-i18next';
import { useAuth } from '../../stores/auth';
import { useNavigate } from 'react-router-dom';
import { LanguageSwitcher } from '../common/LanguageSwitcher';

export function Header() {
  const { t } = useTranslation('common');
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
        <span style={{ fontWeight: 600, fontSize: 'var(--fs-h3)' }}>{t('app.title')}</span>
        <span style={{ color: 'var(--color-text-muted)', fontSize: 'var(--fs-caption)', marginLeft: 8 }}>
          {t('app.tagline')}
        </span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--sp-4)' }}>
        <LanguageSwitcher />
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
            >{t('user.logout')}</button>
          </>
        )}
      </div>
    </header>
  );
}
