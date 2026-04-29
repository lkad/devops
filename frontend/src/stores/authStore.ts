import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

export interface User {
  username: string
  roles: string[]
  permissions: string[]
}

interface AuthState {
  token: string | null
  user: User | null
  isAuthenticated: boolean
  _hasHydrated: boolean
  login: (token: string, user: User) => void
  logout: () => void
  setUser: (user: User) => void
  setHasHydrated: (state: boolean) => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      isAuthenticated: false,
      _hasHydrated: false,

      login: (token: string, user: User) => {
        set({ token, user, isAuthenticated: true })
      },

      logout: () => {
        set({ token: null, user: null, isAuthenticated: false })
      },

      setUser: (user: User) => {
        set({ user })
      },

      setHasHydrated: (state: boolean) => {
        set({ _hasHydrated: state })
      },
    }),
    {
      name: 'auth-storage',
      storage: createJSONStorage(() => localStorage),
      onRehydrateStorage: () => (state) => {
        state?.setHasHydrated(true)
      },
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)