import { Navigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/authStore'

interface ProtectedRouteProps {
  children: React.ReactNode
  requiredPermission?: string
}

export function ProtectedRoute({ children, requiredPermission }: ProtectedRouteProps) {
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated)
  const hasHydrated = useAuthStore((state) => state._hasHydrated)
  const user = useAuthStore((state) => state.user)

  // Wait for rehydration before checking auth
  if (!hasHydrated) {
    return (
      <div className="min-h-screen bg-[#0c1220] flex items-center justify-center">
        <div className="animate-spin w-8 h-8 border-2 border-[#22d3ee] border-t-transparent rounded-full" />
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  if (requiredPermission && user) {
    const hasPermission = user.permissions.includes('all') ||
      user.permissions.includes(requiredPermission)
    if (!hasPermission) {
      return <Navigate to="/" replace />
    }
  }

  return <>{children}</>
}
