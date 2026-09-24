# Sub2API Insights

Independent React and TypeScript application served at `/insights/`. It shares
the existing Sub2API browser session and calls same-origin Insights APIs. Tokens
are never placed in URLs.

## Development

Requirements: Node 24 and pnpm 10.

Run `pnpm install --frozen-lockfile`, then use `pnpm dev`, `pnpm lint`,
`pnpm typecheck`, `pnpm test`, and `pnpm build` as the standard checks.

The Vite server listens on `http://localhost:4178/insights/` and proxies all paths outside `/insights/` (including native login, callbacks, assets, and `/api`)
to `INSIGHTS_API_PROXY_TARGET` (default `http://127.0.0.1:8080`). Production static hosting must route every
`/insights/*` path to `/insights/index.html`, while preserving `/api/*` for the
Sub2API backend.

Set `VITE_INSIGHTS_REFRESH_INTERVAL_SECONDS` at build time to configure automatic
refresh. Values must be at least 5 seconds; invalid values use the 60-second default.

Tests may define local fixtures. Production source files never fall back to demo
data: failed or incomplete endpoints remain visible as error or coverage states.




For an isolated backend on a different port, set `INSIGHTS_API_PROXY_TARGET` before starting Vite. Native login must be served through that same Vite origin; browsing only a static frontend on a separate origin does not share the original login session. `VITE_INSIGHTS_REFRESH_INTERVAL_SECONDS` sets the optional refresh interval (default 60 seconds).

When previewing against an older backend, set `VITE_INSIGHTS_PREVIEW_FIXTURES=true`. This adds deterministic development-only data for the subscription recent-hour pulse and derives model stacks from the returned aggregate model totals when bucket-level model data is unavailable. Usage identities reported with an `unknown` provider are also reconciled when the model name has one unique catalog identity, which keeps mixed-version previews aligned with the catalog. Production builds never use the synthetic preview series.
