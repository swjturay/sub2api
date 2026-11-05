# CCE Production Release

The production release is owned by the `sub2api-production` pipeline in the
HiSpark CodeArts project. The pipeline builds from an exact `cce-deploy`
revision, pushes immutable images to ACR, and starts an in-cluster release Job.
It does not use GitHub Actions or an operations host.

## CodeArts release

The pipeline has two stages: Build and Deploy. It is manual-only, allows one
run at a time, and has no second approval gate. The Build task clones the
public GitHub branch, pins its full SHA, runs the release checks, builds the
Backend, Insights, and release-runner images, then uploads one immutable Job
manifest to CodeArts Artifact. A private CodeArts endpoint stores the ACR
credential; credentials never enter this repository, an image, or the release
artifact.

The release Job maintains two fixed application slots. It prepares and checks
the inactive slot before changing either production Service selector. After
cutover it probes all public routes for three minutes while the old Pods remain
running, then scales the old slot to zero. A failed candidate or public probe
restores the old selectors before the candidate is removed. The Job continues
to a safe terminal state if the pipeline UI is closed or its observer is
stopped.

CodeArts Deploy retains the immutable artifact used by each deployment. Native
rollback selects the previous successful deployment record and applies its
artifact through the same release Job. Completed Jobs have a short TTL so an
older artifact can create its Job again. Database migrations remain
forward-only and are not reversed by an application rollback.

The permanent namespace bootstrap is
`deploy/cce/codearts/rbac.yaml`. It grants only the release ServiceAccount's
namespace-scoped Deployment, Service, Pod, Endpoint and lock-ConfigMap access,
plus the required network egress. Routine releases apply only the generated
ConfigMap and Job.

The release gate intentionally contains only production-relevant checks:

- backend unit tests, build, lint, vulnerability scan, and real PostgreSQL
  Insights/migration regressions;
- frontend lint, typecheck, critical regressions and production dependency
  audit;
- complete Insights typecheck, lint, test and build;
- CCE release helper and renderer tests.

The Python-version matrix, Apple container checks, generic Compose/Caddy tests,
and GitHub release-matrix helpers are not release gates because they do not
validate the CCE images or cutover transaction.

## Legacy GitHub archive

The GitHub workflow remains available as a legacy image archive path, but the
CodeArts production pipeline does not invoke it or inspect its result.

### Build and archive

If the source commit has no CI runs (for example after restoring Actions on a
disabled fork), run all three validation workflows against `cce-deploy` before
building the release:

```sh
gh workflow run backend-ci.yml --ref cce-deploy
gh workflow run insights-ci.yml --ref cce-deploy
gh workflow run security-scan.yml --ref cce-deploy
```

Verify each run's full source SHA matches the intended branch head and that
all three succeed. Manual validation preserves the same jobs and permissions
as push validation; it does not bypass the image workflow's CI gate.

`.github/workflows/cce-clean-image.yml` builds the backend and Insights images
in parallel and pushes only immutable linux/amd64 images to Alibaba ACR.

Use either of these release entry points:

- Push a tag named `cce-v<version>` from a commit on `cce-deploy` to build that
  exact commit automatically.
- Run `CCE ACR release archive` manually with `source_ref=cce-deploy`, the
  branch-head full commit SHA, and version.

`cce-deploy` is the sole deployment source branch. The workflow rejects manual
release refs other than `cce-deploy` and rejects tags whose commits are not
reachable from that branch.

The workflow uploads `cce-release-manifest.json` plus its SHA-256 checksum as a
small Actions artifact. Image tar archives and GHCR copies are not produced.

### Deploy from the operations host

Install or refresh the repository-owned deployment command from the same reviewed release source before rollout:

```sh
sudo deploy/cce/install.sh
```

Transfer the release manifest artifact to the operations host, verify its
checksum, and run:

```sh
sha256sum -c cce-release-manifest.json.sha256
sudo sub2api-cce-release --manifest ./cce-release-manifest.json
```

For a manifest inspection without changing Kubernetes resources:

```sh
sudo sub2api-cce-release \
  --manifest ./cce-release-manifest.json \
  --dry-run
```

Use `--expected-current-sha <full-sha>` when a release must fail if production
has moved since approval.

## Release behavior

The command performs one bounded release:

1. Validates both ACR images are immutable digests from the production
   repository and were built from the same source SHA.
2. Updates the backend with native rolling deployment, a five-second endpoint
   propagation delay, and a 60-second application shutdown window. The production
   statistics start is fixed by `INSIGHTS_STATISTICS_START_DATE=2026-06-01` in
   the platform timezone, so later calendar dates without usage rows are zero-use days.
3. Updates Insights directly to the new image. It removes compatibility init
   containers, the `previous-assets` volume, and all old-version mounts.
4. Runs one 30-second final observation across deployment state, pod restart
   counts, public routes, authentication boundaries, and the current hashed UI
   asset.
5. Rolls back both Deployments to their previous Kubernetes revisions if any
   rollout or final check fails.

Every run is serialized with `/tmp/sub2api-cce-release.lock`. Audit JSON is
written under `~/sub2api-releases/` by default. Database migrations remain
forward-only; review migration changes before approving a production manifest.
