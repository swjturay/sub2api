# Insights desktop design system

## Direction

A desktop analytics workspace with a compact navigation row, clear numerical hierarchy and charts as the main visual focus. Light mode uses a neutral canvas, white surfaces, deep ink and indigo/teal data accents. Dark mode is designed through the same semantic tokens. Existing metrics, meanings and permissions remain authoritative.

## Approved composition

- One approximately 64px top navigation row; no left sidebar.
- A desktop content grid with wider usable space on large displays and consistent gutters.
- Compact date-range, granularity and searchable multi-selection controls instead of always-expanded native multi-select lists.
- Department overview keeps all 9 metrics in one coherent group: 3 primary values and 6 compact supporting values. Formulas move to focusable help instead of expanding the metric height.
- Department and personal history pair the main trend (8 grid columns) with compact model distribution (4 columns). The distribution retains all models in an accessible full-detail dialog.
- Pareto and TOP10 stay adjacent; model performance retains its independent model selection.
- Personal daily subscriptions remain separate quota ledgers. Today Token composition is labeled clearly. Annual heatmap remains independent of model filtering.
- Gateway quality, user analysis, retention and preference are distinct sections. Uncollected periods, unknown values and genuine errors have different presentations.
- Model cards prioritize identity and useful available information; a persistent comparison tray opens the 2–4-model comparison in a large dialog. Pricing preserves labels, units and tier conditions.

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
