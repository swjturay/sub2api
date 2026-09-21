import { createSharedComposable, useTimestamp } from '@vueuse/core'

export const NOW_TICKER_INTERVAL_MS = 30_000

// Lazy and scope-owned: one timer for all mounted rows, disposed with the last row.
export const useNowTicker = createSharedComposable(() => useTimestamp({ interval: NOW_TICKER_INTERVAL_MS }))
