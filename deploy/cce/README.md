# CCE Production Release

The image archive and production deployment are deliberately independent.
GitHub has ACR credentials only; it has no SSH credential, kubeconfig, or path
to the operations host.

## 1. Build and archive

`.github/workflows/cce-clean-image.yml` builds the backend and Insights images
in parallel and pushes only immutable linux/amd64 images to Alibaba ACR.

Use either of these release entry points:

- Push a tag named `cce-v<version>` to build that exact commit automatically.
- Run `CCE ACR release archive` manually with an exact source ref, full commit
  SHA, and version.

The workflow uploads `cce-release-manifest.json` plus its SHA-256 checksum as a
small Actions artifact. Image tar archives and GHCR copies are not produced.

## 2. Deploy from the operations host

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
