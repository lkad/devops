import { create } from 'zustand'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
  duration?: number
}

interface UIState {
  isSideNavCollapsed: boolean
  toasts: Toast[]
  toggleSideNav: () => void
  setSideNavCollapsed: (collapsed: boolean) => void
  addToast: (toast: Omit<Toast, 'id'>) => void
  removeToast: (id: string) => void
}

export const useUIStore = create<UIState>((set) => ({
  isSideNavCollapsed: false,
  toasts: [],

  toggleSideNav: () => {
    set((state) => ({ isSideNavCollapsed: !state.isSideNavCollapsed }))
  },

  setSideNavCollapsed: (collapsed: boolean) => {
    set({ isSideNavCollapsed: collapsed })
  },

  addToast: (toast) => {
    const id = `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
    const newToast = { ...toast, id }
    set((state) => ({ toasts: [...state.toasts, newToast] }))

    // Auto-dismiss after duration (default 3s)
    const duration = toast.duration ?? 3000
    setTimeout(() => {
      set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }))
    }, duration)
  },

  removeToast: (id) => {
    set((state) => ({ toasts: state.toasts.filter((t) => t.id !== id) }))
  },
}))
