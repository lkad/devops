// API client — thin fetch wrapper.
// All paths are RELATIVE (no leading /) so the Vite dev proxy
// forwards them to the backend. In prod, the same paths work
// behind any reverse proxy that mounts the backend at /api.

export type ApiError = { code: string; message: string; details?: any };

const TOKEN_KEY = 'devops-toolkit.jwt';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string | null): void {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export interface ListResponse<T> {
  data: T[];
  pagination: {
    total: number;
    limit: number;
    offset: number;
    has_more: boolean;
  };
}

export interface ApiOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: any;
  query?: Record<string, string | number | boolean | undefined>;
  signal?: AbortSignal;
}

function buildQuery(q?: ApiOptions['query']): string {
  if (!q) return '';
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined || v === null) continue;
    params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `?${s}` : '';
}

export async function api<T = any>(path: string, opts: ApiOptions = {}): Promise<T> {
  const { method = 'GET', body, query, signal } = opts;
  const url = `api/v1/${path.replace(/^\/+/, '')}${buildQuery(query)}`;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  });

  const text = await res.text();
  let parsed: any = null;
  if (text) {
    try { parsed = JSON.parse(text); } catch { parsed = text; }
  }

  if (!res.ok) {
    const err: ApiError = parsed?.error ?? {
      code: 'INTERNAL_ERROR',
      message: `HTTP ${res.status}`,
    };
    const e: any = new Error(err.message);
    e.code = err.code;
    e.status = res.status;
    e.details = err.details;
    throw e;
  }
  return parsed as T;
}

export const apiGet = <T = any>(path: string, query?: ApiOptions['query'], signal?: AbortSignal) =>
  api<T>(path, { method: 'GET', query, signal });

export const apiPost = <T = any>(path: string, body?: any) =>
  api<T>(path, { method: 'POST', body });

export const apiPut = <T = any>(path: string, body?: any) =>
  api<T>(path, { method: 'PUT', body });

export const apiDelete = <T = any>(path: string) =>
  api<T>(path, { method: 'DELETE' });
