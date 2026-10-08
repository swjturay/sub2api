import { useEffect, useState } from "react";
import { ApiRequestError, AuthSessionChangedError, sessionSnapshot, verifySession } from "./auth";
import type { User } from "./types";

// Identity checks continue when chart auto-refresh is off. Hidden pages are
// unmounted and revalidated before protected content is shown again.
export function useInsightsSession(path: string) {
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState("");
  const [checking, setChecking] = useState(true);
  useEffect(() => {
    let active = true, generation = 0;
    const verify = async () => {
      const current = ++generation;
      try {
        const next = await verifySession(sessionSnapshot());
        if (active && current === generation) {
          setUser(next); setError(""); setChecking(false);
        }
      } catch (e) {
        if (!active || current !== generation) return;
        if (e instanceof AuthSessionChangedError) { void verify(); return; }
        setUser(null); setError((e as Error).message); setChecking(false);
        if (e instanceof ApiRequestError && e.status === 401) {
          window.location.assign(`/login?redirect=${encodeURIComponent("/insights" + path)}`);
        }
      }
    };
    const revalidate = () => { setChecking(true); void verify(); };
    const storage = (e: StorageEvent) => {
      if (e.key === null || ["auth_token", "refresh_token"].includes(e.key)) revalidate();
    };
    const visibility = () => {
      setChecking(true);
      if (document.hidden) generation++;
      else void verify();
    };
    void verify();
    const timer = window.setInterval(() => { if (!document.hidden) void verify(); }, 60_000);
    window.addEventListener("storage", storage);
    window.addEventListener("insights-auth-revalidate", revalidate);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      active = false; generation++;
      window.clearInterval(timer);
      window.removeEventListener("storage", storage);
      window.removeEventListener("insights-auth-revalidate", revalidate);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, [path]);
  return { user, error, checking };
}
