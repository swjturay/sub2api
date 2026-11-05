/* eslint-disable react-hooks/exhaustive-deps */
import { useCallback, useEffect, useRef, useState } from "react";
import { REFRESH_INTERVAL_MS } from "./config";

const CACHE_TTL_MS = 60_000;
const CACHE_MAX_ENTRIES = 200;
const cache = new Map<string, { data: unknown; at: number }>();

function writeCache(key: string, data: unknown, at: number) {
  cache.delete(key);
  cache.set(key, { data, at });
  while (cache.size > CACHE_MAX_ENTRIES) {
    const oldest = cache.keys().next().value;
    if (oldest === undefined) break;
    cache.delete(oldest);
  }
}

export function useRemote<T>(
  loader: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
  auto = true,
  cacheKey?: string,
  enabled = true,
) {
  const cached = cacheKey ? cache.get(cacheKey) : undefined;
  const initial = cached && Date.now() - cached.at < CACHE_TTL_MS
    ? cached.data as T
    : null;
  const [data, setData] = useState<T | null>(initial);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(initial === null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(cached ? new Date(cached.at) : null);
  const [dataKey, setDataKey] = useState<string | undefined>(initial !== null ? cacheKey : undefined);
  const generation = useRef(0);
  const dataRef = useRef<T | null>(initial);
  const controllerRef = useRef<AbortController | null>(null);
  const inFlightRef = useRef<Promise<void> | null>(null);
  const keyRef = useRef(cacheKey);
  dataRef.current = data;

  const refresh = useCallback(() => {
    if (inFlightRef.current) return inFlightRef.current;
    const current = ++generation.current;
    const controller = new AbortController();
    controllerRef.current = controller;
    setLoading(dataRef.current === null);
    const task = (async () => {
      try {
        const next = await loader(controller.signal);
        if (current !== generation.current) return;
        setData(next);
        setDataKey(cacheKey);
        dataRef.current = next;
        setError(null);
        const at = new Date();
        setUpdatedAt(at);
        if (cacheKey) writeCache(cacheKey, next, at.getTime());
        window.dispatchEvent(new CustomEvent("insights-refresh-success", { detail: { at } }));
      } catch (cause) {
        if (current === generation.current && (cause as Error).name !== "AbortError") {
          setError((cause as Error).message);
        }
      } finally {
        if (current === generation.current) {
          setLoading(false);
          inFlightRef.current = null;
          window.dispatchEvent(new CustomEvent("insights-refresh-settled"));
        }
        if (controllerRef.current === controller) controllerRef.current = null;
      }
    })();
    inFlightRef.current = task;
    return task;
  }, deps);

  useEffect(() => {
    if (!enabled) return;
    if (keyRef.current !== cacheKey) {
      const entry = cacheKey ? cache.get(cacheKey) : undefined;
      const next = entry && Date.now() - entry.at < CACHE_TTL_MS
        ? entry.data as T
        : null;
      keyRef.current = cacheKey;
      dataRef.current = next;
      setData(next);
      setDataKey(next !== null ? cacheKey : undefined);
      setUpdatedAt(entry ? new Date(entry.at) : null);
      setLoading(next === null);
    }
    void refresh();
    return () => {
      generation.current++;
      controllerRef.current?.abort();
      controllerRef.current = null;
      inFlightRef.current = null;
    };
  }, [refresh, cacheKey, enabled]);

  useEffect(() => {
    if (!enabled) return;
    const manual = () => void refresh();
    window.addEventListener("insights-manual-refresh", manual);
    return () => window.removeEventListener("insights-manual-refresh", manual);
  }, [enabled, refresh]);

  useEffect(() => {
    if (!enabled || !auto || loading) return;
    let timer: number | undefined;
    const schedule = () => {
      window.clearTimeout(timer);
      if (!document.hidden) timer = window.setTimeout(() => void refresh(), REFRESH_INTERVAL_MS);
    };
    const visible = () => {
      if (!document.hidden) void refresh();
      schedule();
    };
    schedule();
    document.addEventListener("visibilitychange", visible);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", visible);
    };
  }, [enabled, auto, refresh, loading, updatedAt]);

  return { data, dataKey, error, loading, updatedAt, refresh };
}
