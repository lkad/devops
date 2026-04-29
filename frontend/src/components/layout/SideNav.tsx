import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  MonitorSmartphone,
  Server,
  GitBranch,
  FileText,
  Bell,
  Activity,
  Container,
  FolderKanban,
  ChevronLeft,
  Settings,
} from 'lucide-react'
import { useUIStore } from '@/stores/uiStore'
import styles from './SideNav.module.css'

interface NavItem {
  path: string
  label: string
  icon: React.ComponentType<{ className?: string }>
}

interface NavSection {
  title: string
  items: NavItem[]
}

const navSections: NavSection[] = [
  {
    title: 'Operations',
    items: [
      { path: '/', label: 'Dashboard', icon: LayoutDashboard },
      { path: '/devices', label: 'Devices', icon: MonitorSmartphone },
      { path: '/physical-hosts', label: 'Hosts', icon: Server },
      { path: '/pipelines', label: 'Pipelines', icon: GitBranch },
    ],
  },
  {
    title: 'Observability',
    items: [
      { path: '/logs', label: 'Logs', icon: FileText },
      { path: '/alerts', label: 'Alerts', icon: Bell },
      { path: '/alerts/history', label: 'Alert History', icon: Activity },
    ],
  },
  {
    title: 'Kubernetes',
    items: [
      { path: '/k8s', label: 'Clusters', icon: Container },
    ],
  },
  {
    title: 'Management',
    items: [
      { path: '/projects', label: 'Projects', icon: FolderKanban },
      { path: '/reports/finops', label: 'FinOps', icon: Activity },
      { path: '/audit-logs', label: 'Audit Logs', icon: FileText },
      { path: '/settings', label: 'Settings', icon: Settings },
    ],
  },
]

export function SideNav() {
  const isSideNavCollapsed = useUIStore((state) => state.isSideNavCollapsed)
  const toggleSideNav = useUIStore((state) => state.toggleSideNav)

  return (
    <nav className={`${styles.sidenav} ${isSideNavCollapsed ? styles.sidenavCollapsed : ''}`}>
      <div className={styles.header}>
        <div className={styles.logo}>
          <div className={styles.logoIcon}>DT</div>
          <span className={styles.logoText}>DevOps Toolkit</span>
        </div>
        <button
          type="button"
          onClick={toggleSideNav}
          className={styles.toggleButton}
          aria-label={isSideNavCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          <ChevronLeft />
        </button>
      </div>

      <div className={styles.content}>
        {navSections.map((section) => (
          <div key={section.title} className={styles.section}>
            <div className={styles.sectionTitle}>{section.title}</div>
            <ul className={styles.navList}>
              {section.items.map((item) => (
                <li key={item.path}>
                  <NavLink
                    to={item.path}
                    className={({ isActive }) =>
                      `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
                    }
                    end={item.path === '/'}
                  >
                    <span className={styles.navIcon}>
                      <item.icon />
                    </span>
                    <span className={styles.navText}>{item.label}</span>
                  </NavLink>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </nav>
  )
}
