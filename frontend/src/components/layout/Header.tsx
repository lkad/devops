import { useNavigate, NavLink } from 'react-router-dom'
import { LogOut, Calendar } from 'lucide-react'
import { useAuthStore } from '@/stores/authStore'
import styles from './Header.module.css'

export function Header() {
  const navigate = useNavigate()
  const { user, logout } = useAuthStore()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const currentDate = new Date().toLocaleDateString('en-US', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })

  return (
    <header className={styles.header}>
      <div className={styles.left}>
        <span className={styles.date}>
          <Calendar />
          {currentDate}
        </span>

        <div className={styles.logo}>
          <div className={styles.logoIcon}>DT</div>
          <span className={styles.logoText}>DevOps Toolkit</span>
        </div>

        <nav className={styles.nav}>
          <NavLink
            to="/"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
            end
          >
            Dashboard
          </NavLink>
          <NavLink
            to="/devices"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            Devices
          </NavLink>
          <NavLink
            to="/physical-hosts"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            Hosts
          </NavLink>
          <NavLink
            to="/pipelines"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            Pipelines
          </NavLink>
          <NavLink
            to="/k8s"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            K8s
          </NavLink>
          <NavLink
            to="/projects"
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            Projects
          </NavLink>
        </nav>
      </div>

      <div className={styles.right}>
        <div className={styles.user}>
          <span className={styles.username}>{user?.username || 'Guest'}</span>
          <button
            type="button"
            onClick={handleLogout}
            className={styles.logoutButton}
          >
            <LogOut />
            Logout
          </button>
        </div>
      </div>
    </header>
  )
}
