// authStore — current user + JWT, persisted to localStorage.
// On boot, decodes the cached JWT (no network call) to populate
// the user shell; the JWT is re-verified by the backend on every
// API request.

import { create } from 'zustand';
import { apiPost, getToken, setToken, ApiError } from '../api/client';

export type Role =
  | 'SuperAdmin'
  | 'Operator'
  | 'Developer'
  | 'Auditor'
  | 'ProjectAdmin';

export interface User {
  username: string;
  role: Role;
  // expiresAt is in seconds since epoch (JWT exp claim).
  expiresAt: number;
}

interface AuthState {
  user: User | null;
  loading: boolean;
  error: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  restore: () => void;
}

function decodeJwt(token: string): { exp: number; sub: string; usr: string; role: Role } | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1]));
    return {
      exp: payload.exp,
      sub: payload.sub,
      usr: payload.usr,
      role: payload.role,
    };
  } catch {
    return null;
  }
}

export const useAuth = create<AuthState>((set) => ({
  user: null,
  loading: false,
  error: null,

  login: async (username, password) => {
    set({ loading: true, error: null });
    try {
      const res = await apiPost<{ token: string; user: User; expires_at: number }>('auth/login', { username, password });
      setToken(res.token);
      set({ user: { ...res.user, expiresAt: res.expires_at }, loading: false });
    } catch (e: any) {
      const err: ApiError = { code: e?.code ?? 'INTERNAL_ERROR', message: e?.message ?? 'Login failed' };
      set({ loading: false, error: err.message });
      throw err;
    }
  },

  logout: () => {
    setToken(null);
    set({ user: null });
  },

  restore: () => {
    const tok = getToken();
    if (!tok) return;
    const claims = decodeJwt(tok);
    if (!claims) { setToken(null); return; }
    if (claims.exp * 1000 < Date.now()) { setToken(null); return; }
    set({ user: { username: claims.usr, role: claims.role, expiresAt: claims.exp } });
  },
}));
