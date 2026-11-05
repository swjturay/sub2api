import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { useDeferredLoad } from "./useDeferredLoad";

it("retries when the deferred section mounts after the initial loading state", async () => {
  const observe = vi.fn();
  const disconnect = vi.fn();
  class FakeObserver {
    constructor(private readonly callback: IntersectionObserverCallback) {}
    observe = (target: Element) => {
      observe(target);
      this.callback([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver);
    };
    disconnect = disconnect;
  }
  vi.stubGlobal("IntersectionObserver", FakeObserver);
  const ref = { current: null as Element | null };
  const hook = renderHook(({ active }: { active: boolean }) => useDeferredLoad(ref, active), { initialProps: { active: false } });
  expect(hook.result.current).toBe(false);
  const section = document.createElement("div");
  ref.current = section;
  await act(async () => hook.rerender({ active: true }));
  expect(hook.result.current).toBe(true);
  expect(observe).toHaveBeenCalledWith(section);
  expect(disconnect).toHaveBeenCalled();
  vi.unstubAllGlobals();
});
