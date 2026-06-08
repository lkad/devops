// useApi — declarative data fetcher with loading/error state.
// Aborts in-flight request on unmount or when deps change.
import { useEffect, useState, useRef } from 'react';
import { apiGet, ApiError } from '../api/client';

export interface UseApiResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

export function useApi<T = any>(path: string | null, query?: Record<string, any>): UseApiResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const aborterRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!path) return;
    aborterRef.current?.abort();
    const aborter = new AbortController();
    aborterRef.current = aborter;
    setLoading(true);
    setError(null);
    apiGet<T>(path, query, aborter.signal)
      .then((d) => { if (!aborter.signal.aborted) setData(d); })
      .catch((e: any) => { if (!aborter.signal.aborted) setError(e?.message ?? 'fetch failed'); })
      .finally(() => { if (!aborter.signal.aborted) setLoading(false); });
    return () => aborter.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, tick, JSON.stringify(query)]);

  return { data, loading, error, reload: () => setTick((n) => n + 1) };
}
