const configured = Number(import.meta.env.VITE_INSIGHTS_REFRESH_INTERVAL_SECONDS);
export const REFRESH_INTERVAL_SECONDS = Number.isFinite(configured) && configured >= 5 ? Math.floor(configured) : 60;
export const REFRESH_INTERVAL_MS = REFRESH_INTERVAL_SECONDS * 1000;
