import { describe, expect, it } from "vitest";
import { REFRESH_INTERVAL_MS, REFRESH_INTERVAL_SECONDS } from "./config";
describe("refresh interval configuration",()=>{it("uses one validated interval for timer and UI",()=>{expect(REFRESH_INTERVAL_SECONDS).toBeGreaterThanOrEqual(5);expect(REFRESH_INTERVAL_MS).toBe(REFRESH_INTERVAL_SECONDS*1000)})});
