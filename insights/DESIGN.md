# Insights visual system

## Direction

Insights is a mature operational analytics product. The interface uses a restrained blue accent, neutral layered surfaces, compact controls and dense but readable data hierarchy. Charts and tables remain the visual focus; chrome stays quiet.

## Structure

- A two-row sticky top bar replaces the previous sidebar and mobile drawer.
- The first row holds product identity, account context and global actions.
- The second row holds the four permission-aware module tabs and refresh controls.
- Main content is centered at a 1500px maximum width with 22px section rhythm.
- Filters use a single compact toolbar. Related KPIs form one continuous metric strip instead of equal floating cards.

## Components and tokens

- Primary blue is reserved for active navigation, primary actions, focus and selected states.
- Surfaces use `--surface`, `--surface-subtle` and `--surface-muted`; borders use `--border` and `--border-strong`.
- Panels use 12px corners and a short 8px shadow at most. Controls use 9–10px corners.
- State transitions run for 180–190ms and become effectively instant under reduced-motion preferences.
- Small non-zero metrics preserve significant digits. Timestamps use the API platform timezone.

## References

The implementation draws on the compact information hierarchy of BeautifulUI insight and records patterns, beUI tabs/theme/calendar details, transitions.dev state transitions, RareUI interaction polish, and shadcn dashboard control rhythm. It keeps native HTML controls and existing ECharts behavior for accessibility and reliability.

## Constraints retained

All routes, permissions, filters, coverage/error states, dark mode, automatic refresh, account switching and real API contracts remain unchanged. The product adds no export controls, invented comparisons or decorative dashboard statistics.

## Acceptance scope

The user's September 23, 2026 update makes PC the delivery target. Verify 1366, 1440 and 1920 CSS-pixel widths in light and dark themes. Retain existing responsive behavior, but mobile-specific refinement is no longer a release requirement.
