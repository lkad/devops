import { Outlet } from 'react-router-dom'
import { Header } from './Header'
import { SideNav } from './SideNav'
import { useUIStore } from '@/stores/uiStore'
import styles from './AppShell.module.css'
import { useAuthStore } from '@/stores/authStore'
import { useEffect } from 'react'

export function AppShell() {
  const isSideNavCollapsed = useUIStore((state) => state.isSideNavCollapsed)
  const setHasHydrated = useAuthStore((state) => state.setHasHydrated)
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)

  // Handle hydration
  useEffect(() => {
    // Auth store handles its own hydration via persist middleware
    // Just mark UI store as ready
  }, [isAuthenticated, setHasHydrated])

  return (
    <div className={styles.appShell}>
      <SideNav />
      <main
        className={`${styles.main} ${isSideNavCollapsed ? styles.mainCollapsed : ''}`}
      >
        <Header />
        <div className={styles.content}>
          <Outlet />
        </div>
      </main>
    </div>
  )
}
