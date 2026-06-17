// Login — centered card on a near-black background.
// Posts credentials to the auth store, navigates to "/" on success.

import { useTranslation } from 'react-i18next';
import { useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../stores/auth';
import { Button } from '../components/common/Button';
import { FormField } from '../components/common/FormField';

const cardStyle: React.CSSProperties = {
  width: 420,
  maxWidth: 'calc(100vw - var(--sp-7))',
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius-lg)',
  padding: 'var(--sp-7)',
  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
};

const brandStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--sp-3)',
  marginBottom: 'var(--sp-6)',
  justifyContent: 'center',
};

const logoStyle: React.CSSProperties = {
  width: 40,
  height: 40,
  borderRadius: 'var(--radius-md)',
  background: 'linear-gradient(135deg, var(--color-primary) 0%, #6366f1 100%)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  color: 'var(--color-bg)',
  fontWeight: 700,
  fontSize: 20,
};

const nameStyle: React.CSSProperties = {
  fontWeight: 700,
  fontSize: 'var(--fs-h2)',
  letterSpacing: -0.3,
};

const errorStyle: React.CSSProperties = {
  background: 'rgba(239, 68, 68, 0.1)',
  border: '1px solid rgba(239, 68, 68, 0.3)',
  color: 'var(--color-error)',
  fontSize: 'var(--fs-small)',
  padding: 'var(--sp-3)',
  borderRadius: 'var(--radius-sm)',
  marginBottom: 'var(--sp-4)',
};

const hintStyle: React.CSSProperties = {
  marginTop: 'var(--sp-5)',
  padding: 'var(--sp-3)',
  background: 'var(--color-bg)',
  border: '1px solid var(--color-border-subtle)',
  borderRadius: 'var(--radius-sm)',
  fontSize: 'var(--fs-caption)',
  color: 'var(--color-text-muted)',
  lineHeight: 1.5,
};

const inputDisabledStyle: React.CSSProperties = {
  opacity: 0.6,
  cursor: 'not-allowed',
};

export function Login() {
  const { t } = useTranslation('login');
  const nav = useNavigate();
  const login = useAuth((s) => s.login);
  const loading = useAuth((s) => s.loading);
  const error = useAuth((s) => s.error);

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await login(username, password);
      nav('/');
    } catch {
      // error is surfaced via useAuth().error
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        background: 'var(--color-bg)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 'var(--sp-6)',
        fontFamily: 'var(--font-ui)',
      }}
    >
      <form onSubmit={onSubmit} style={cardStyle}>
        <div style={brandStyle}>
          <div style={logoStyle}>D</div>
          {/* Brand name stays English per SPEC §5 */}
          <div style={nameStyle}>DevOps Toolkit</div>
        </div>

        <h1
          style={{
            fontSize: 'var(--fs-h3)',
            color: 'var(--color-text-secondary)',
            fontWeight: 500,
            textAlign: 'center',
            marginBottom: 'var(--sp-6)',
          }}
        >
          {t('subtitle')}
        </h1>

        {error && <div role="alert" style={errorStyle}>{error}</div>}

        <FormField label={t('form.username-label')}>
          {(s) => (
            <input
              type="text"
              autoComplete="username"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={loading}
              style={loading ? { ...s, ...inputDisabledStyle } : s}
            />
          )}
        </FormField>

        <FormField label={t('form.password-label')}>
          {(s) => (
            <input
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={loading}
              style={loading ? { ...s, ...inputDisabledStyle } : s}
            />
          )}
        </FormField>

        <Button
          variant="primary"
          type="submit"
          disabled={loading || !username || !password}
          style={{ width: '100%', justifyContent: 'center', padding: '8px 14px' }}
        >
          {loading ? t('form.submitting') : t('form.submit')}
        </Button>

        <div style={hintStyle}>
          {t('hint', { user1: 'test_admin', user2: 'test_viewer', pass: 'test' })}
        </div>
      </form>
    </div>
  );
}
