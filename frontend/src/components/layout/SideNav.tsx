// SideNav — left rail with module entries.
import { NavLink } from 'react-router-dom';

const items = [
  { to: '/', label: 'Dashboard', icon: '◫' },
  { to: '/projects', label: 'Projects', icon: '⌥' },
  { to: '/services', label: 'Services', icon: '⬢' },
  { to: '/devices', label: 'Devices', icon: '◇' },
  { to: '/physical-hosts', label: 'Physical Hosts', icon: '☷' },
  { to: '/k8s', label: 'K8s Clusters', icon: '⬡' },
  { to: '/discovery', label: 'Discovery', icon: '◎' },
  { to: '/pipelines', label: 'Pipelines', icon: '▶' },
  { to: '/logs', label: 'Logs', icon: '☰' },
  { to: '/metrics', label: 'Metrics', icon: '◐' },
  { to: '/alerts', label: 'Alerts', icon: '⚠' },
  { to: '/audit', label: 'Audit', icon: '✓' },
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
          <span>{it.label}</span>
        </NavLink>
      ))}
    </nav>
  );
}
