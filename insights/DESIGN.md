# Insights desktop design system

## Direction

A desktop analytics workspace with a compact navigation row, clear numerical hierarchy and charts as the main visual focus. Light mode uses a neutral canvas, white surfaces, deep ink and indigo/teal data accents. Dark mode is designed through the same semantic tokens. Existing metrics, meanings and permissions remain authoritative.

## Approved composition

- One approximately 64px top navigation row; no global left navigation sidebar. The model catalog has its own local filter rail.
- A desktop content grid with wider usable space on large displays and consistent gutters.
- Compact date-range, granularity and searchable multi-selection controls instead of always-expanded native multi-select lists.
- Department overview keeps all 9 metrics in three semantic columns: membership and requests; total and normalized Token consumption; output and cache composition. Each column has one primary value and two supporting values. Formulas use focusable help.
- Department history starts with a full-width department Token trend, followed by model Token trends (8 columns) and request-based model distribution (4 columns). Both trends default to stacked bars; each independently selects total/output Tokens and lines/stacked bars. Categories use actual per-bucket values; show the leading seven with a conserved remainder, preserve missing breakdowns as unknown, and label incomplete buckets. Personal history retains its existing trend/distribution layout. The model distribution retains all models in an accessible full-detail dialog.
- Pareto and TOP10 stay adjacent; model performance retains its independent model selection.
- Personal daily subscriptions remain separate quota ledgers. Today Token composition is labeled clearly. Annual heatmap remains independent of model filtering.
- Gateway quality, user analysis, retention and preference are distinct sections. Uncollected periods, unknown values and genuine errors have different presentations.
- The model catalog uses a search header, manufacturer/capability/input filters, sorting, card/list views and 12-item pagination. Group Gemini, Claude and other known model families by manufacturer rather than routing platform; cards, details and comparison labels use the matching manufacturer mark/name while preserving backend identities. Cards prioritize display names and exact invocation identifiers, context/capabilities, reference prices and aggregated performance. The existing repository artwork is reused. New API's pricing catalog is an information-hierarchy reference only; its source and assets are not vendored.
- Model selection survives filtering, paging and view changes; a persistent comparison tray opens the existing 2–4-model comparison. A right-side detail dialog separates specifications/performance from full pricing/sources and restores focus on close. Pricing preserves labels, units and tier conditions; cards never present conditional/tiered entries as an unconditional quote. Missing measurements remain unknown and sort after measured values, including genuine zero.
- Catalog and comparison performance use the last 24 hours. The 24-hour/7-day control lives inside each model's detail performance section and changes only that detail. Opening another detail resets to 24 hours. Capability filters cover declared reasoning, tools, structured output, image/audio/video inputs, file/PDF inputs, image output and an explicit 128K-token context threshold. Omit the price-count summary and explanatory catalog banners; retain metric definitions in tooltips.

## Component foundation

Use shadcn-style source components backed by Radix UI, cmdk and React Day Picker where appropriate. Their source/dep provenance is recorded in THIRD_PARTY_NOTICES.md. ECharts remains the chart engine. RareUI is an interaction reference only; its source is not vendored.

Public interfaces retain existing auth, refresh, data and role semantics. Portaled controls and dialogs support focus, keyboard navigation and Escape. Popover filters keep full labels discoverable. No exports or member drill-down are added.

## Tokens and presentation

- A small semantic surface/border/text palette and one primary action color.
- 12–14px labels/body, 26–28px page titles, 32–36px primary metrics; tabular numerals for values.
- 12px panel corners with restrained borders and shadows; consistent control height and spacing.
- Charts use 12px labels, subtle grids, exact tooltips and consistent number/date formatting. Incomplete buckets and nulls remain explicit.
- Motion is limited to short state transitions; reduced-motion users receive immediate stable content.
- Loading reserves meaningful structure; automatic refresh retains the last successful data without full-page flicker.

## Desktop acceptance

Verify 1366×768, 1440×900 and 1920×1080 in light/dark, plus a wide desktop inspection. At 1440×900 the department first viewport should expose filters, all overview metrics and the complete main trend. Test real-length model/department names, multiple subscriptions, 2–4-model comparison, empty/partial data and keyboard focus. Mobile refinement is outside the requested scope.

## Validation boundaries

Use the isolated local backend and synthetic test accounts for development checks. UI verification must not generate paid inference requests or modify production profiles. Screenshots are review artifacts; distinguish measured DOM/interaction checks from any actual pixel inspection.
