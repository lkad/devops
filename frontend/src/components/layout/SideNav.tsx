// SideNav — left rail with module entries. Per docs/i18n/SPEC.md §4,
// labels are read from the i18n common namespace.
import { useTranslation } from 'react-i18next';
import { NavLink } from 'react-router-dom';

const items = [
  { to: '/', labelKey: 'nav.dashboard', icon: '◫' },
  { to: '/projects', labelKey: 'nav.projects', icon: '⌥' },
  { to: '/services', labelKey: 'nav.services', icon: '⬢' },
  { to: '/devices', labelKey: 'nav.devices', icon: '◇' },
  { to: '/physical-hosts', labelKey: 'nav.physical-hosts', icon: '☷' },
  { to: '/k8s', labelKey: 'nav.k8s-clusters', icon: '⬡' },
  { to: '/discovery', labelKey: 'nav.discovery', icon: '◎' },
  { to: '/pipelines', labelKey: 'nav.pipelines', icon: '▶' },
  { to: '/logs', labelKey: 'nav.logs', icon: '☰' },
  { to: '/metrics', labelKey: 'nav.metrics', icon: '◐' },
  { to: '/alerts', labelKey: 'nav.alerts', icon: '⚠' },
  { to: '/audit', labelKey: 'nav.audit', icon: '✓' },
];

const linkStyle = (active: boolean): React.CSSProperties => ({
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--sp-3)',
  padding: 'var(--sp-2) var(--sp-4)',
  color: active ? 'var(--color-primary)' : 'var(--color-text-secondary)',
  background: active ? 'var(--color-primary-muted)' : 'transparent',
  borderLeft: active ? '2px solid var(--color-primary)' : '2px solid transparent',
  fontSize: 'var(--fs-small)',
  fontWeight: active ? 600 : 400,
  textDecoration: 'none',
  transition: 'background 0.1s',
});

export function SideNav() {
  const { t } = useTranslation('common');
  return (
    <nav
      style={{
        width: 'var(--sidenav-w)',
        background: 'var(--color-surface)',
        borderRight: '1px solid var(--color-border)',
        flexShrink: 0,
        padding: 'var(--sp-4) 0',
        overflow: 'auto',
      }}
    >
      {items.map((it) => (
        <NavLink
          key={it.to}
          to={it.to}
          end={it.to === '/'}
          style={({ isActive }) => linkStyle(isActive)}
        >
          <span style={{ width: 16, textAlign: 'center', fontSize: 14, opacity: 0.7 }}>{it.icon}</span>
          <span>{t(it.labelKey)}</span>
        </NavLink>
      ))}
    </nav>
  );
}
