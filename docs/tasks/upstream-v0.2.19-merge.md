# Upstream v0.2.19 merge into cce-deploy

Date: 2026-10-30 (Asia/Tokyo). Scope: source integration and verification.

## Source

- Branch baseline: `fe0858821300143dc98cfcbd72b1f2415750657e`.
- Official [v0.2.19 release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.19):
  `dece26153621959ccd7635814f4b2d08304e681a`.
- Common upstream ancestor: `ac1b8f1271be0d674fa6f074403c1410f30a3a81`.
- 71 incoming upstream commits.

## Adopted upstream changes

The release adds scoped administrator API keys with operation/resource
permissions, expiry and IP allowlists; GPT-6.1 Terra; configurable Gemini
same-account retries; per-image OpenAI/Composite billing; refreshed Gemini
model catalogs and mapped model discovery; Grok model-catalog diagnostics;
model-list singleflight; DeepSeek/OpenCode and Antigravity compatibility fixes;
image upload limits; pagination overflow protection; channel web-search
platform preservation; and current dependency/security updates.

Migrations 247 and 248 add scoped admin API-key storage and the group image
count-billing switch. Runtime VERSION is corrected to 0.2.19 because the
upstream tag still stores 0.2.18.

## Preserved cce-deploy behavior

The upstream release deletes many files that belong to this fork. The merge
restores the full CPR relay, management-only quota and credential-health checks,
received-usage accounting, Codex wire/identity protections, Insights backend and
frontend/read-access rules, CCE image release workflow and operations command,
local setup scripts, conventions, journal and regression tests. No deletion of
those fork-owned files is accepted.

The Antigravity streaming conflict takes upstream's final malformed-function
error detection and retains the fork's diagnostic record and error mapping.
Other CCE changes remain source-compatible with the upstream additions.

## Validation boundary

Run the repository's backend unit/integration/build checks, frontend critical
and full tests/build, Insights checks/build, local setup and CCE helper tests,
security scans and migration immutability checks. Validate CI, Insights CI and
Security Scan against the final cce-deploy SHA. This task creates no release
images and changes no production resources.

## Security validation follow-up

The frontend audit's new `decode-uri-component` advisory is fixed by the
0.5.1 override and lockfile update. Vue I18n 9.14.5 has no patched release for
the reported `srcdoc` advisory; the audit records time-bounded exceptions for
`vue-i18n`, `@intlify/core-base` and `@intlify/shared`. Locale messages are
repository-controlled and the application does not pass user-controlled content
to iframe `srcdoc`; migration to a patched Vue I18n major remains tracked by
the exception expiry.
