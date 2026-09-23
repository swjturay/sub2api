# Insights PC redesign validation

Date:2026-09-23. Base:4eb2620eadf2ebe56134ffd6fffeb105222de50e.
Branch:codex/insights-pc-polish.

This record concerns the frontend redesign only. No backend, billing, database,
role/authentication contract or production deployment changed in this work.
The previously deployed Insights release remains separate from this candidate.

## Automated checks

- pnpm --dir insights test:17 files,56 tests passed.
- pnpm --dir insights lint:passed.
- pnpm --dir insights typecheck:passed.
- pnpm --dir insights build:passed.
- git diff --check:passed.
- The production build contains full third-party notices and the self-hosted
 48.25KB Inter Latin font. No remote font request is required.
- The existing large-bundle advisory remains; route/chart code splitting was not
 part of this visual redesign.

New behavioral regressions cover valid date-range application, platform-day
presets across the UTC date boundary, text-only/native option compatibility,
actual multi-select checked state, bounded comparison selection, safe price
conversion/tier preservation and dialog focus restoration.

## Browser evidence

The gpt-5.6-sol/high independent validation worker used an isolated Playwright
session against the real local same-origin edge at127.0.0.1:4187 and the isolated
synthetic backend. It covered4 pages×3 desktop sizes×2 themes (24 combinations):
1366×768,1440×900,1920×1080 in light and dark.

The matrix found no page-level horizontal overflow or unintended non-overlay
content extending beyond the viewport. Chart canvases had nonzero dimensions.
Department data preserved6 members,5 active members,235 usage records and9,631
output tokens for the fixture interval2026-09-17–2026-09-23 (Asia/Shanghai).

Final root measurements at1440×900:

| Element | Previous | Redesigned |
| --- | ---: | ---: |
| Top navigation |119px|64px|
| Filter toolbar |148px|60px|
| Main trend top |762px|about462px|
| Main trend bottom |1174px|about814px|

All9 department metrics and the whole main trend fit in the first900px viewport.
Small measurement differences between browsers reflect fonts/rendering; the
first-viewport requirement was independently verified.

Final interaction checks passed:

- Searchable model/department selection, actual checked state and Escape return
 to the trigger; independent performance selection remains separate.
- Historical date range followed by Today resolves to the current platform day,
 September23,2026, rather than the previous selected end date.
- Comparison and model-detail Escape restore the original opener;2→1 removal
 falls back to model search when the comparison opener becomes disabled.
- All4 comparison trends render, known pricing is labeled per million Token and
 original values/unknown syntax remain available.
- Personal log sizes20,50,100 are available;50 and100 selections work.
- Ordinary-user navigation contains only Personal and Models; the admin page
 redirects to Personal. Direct local personal API returns200 and admin API403.
- User-menu automatic refresh is a Radix menuitemcheckbox reachable by Home/
 arrow keys; Space toggles it while keeping the menu open.
- Hostile model labels remain inert text. No paid model requests were made.

## Review artifacts

Artifacts are ignored local files, not production or public model data:

- output/playwright/pc-polish/:24 desktop matrix screenshots.
- output/playwright/pc-polish-final/departments-light-1440x900.png
- output/playwright/pc-polish-final/departments-dark-1440x900.png
- output/playwright/pc-polish-final/models-comparison-light-1440x900.png
- output/playwright/pc-polish-final/models-profile-light-1440x900.png
- output/playwright/pc-polish-final/personal-dark-1920x1080.png

Image input was unavailable to the agents. Screenshots were captured for human
review; validation conclusions are based on DOM measurements, accessibility
snapshots and actual browser interactions, not claimed pixel inspection.
Production OAuth/2FA and production load were not exercised by this frontend
revision. No mobile refinement was requested or counted as a release gate.
