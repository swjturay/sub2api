import { useEffect, useState, type RefObject } from "react";

export function useDeferredLoad(ref: RefObject<Element | null>, active: boolean, fallbackMs = 900) {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    if (!active || enabled) return;
    const section = ref.current;
    if (!section) return;
    if (!("IntersectionObserver" in window)) {
      const timer = globalThis.setTimeout(() => setEnabled(true), fallbackMs);
      return () => globalThis.clearTimeout(timer);
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        setEnabled(true);
        observer.disconnect();
      }
    }, { rootMargin: "420px" });
    observer.observe(section);
    return () => observer.disconnect();
  }, [active, enabled, fallbackMs, ref]);
  return enabled;
}
