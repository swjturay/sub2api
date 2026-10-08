# Local client setup

The Unix and Windows bootstraps prefer an existing Python 3.8 or later.
Python 3.8–3.10 downloads the small TOML parser below from the same Sub2API
site as the helper, only for Codex. Python 3.11+ uses `tomllib`. Claude and
OpenCode use JSON and need no TOML dependency. No pip install, administrator
access or Python upgrade is required. Existing configurations are still
validated and backed up before replacement.

If no compatible Python exists, the bootstrap uses its private portable runtime
cache or downloads a checksum-pinned runtime. Unix downloads show their stage
and progress, have a 10-second connection timeout and bounded retries. Helper
downloads have a 60-second per-request limit; portable Python uses 180 seconds.
The temporary helper and partial downloads are removed on exit.

## Bundled TOML parser

`tomli-2.2.1-py3-none-any.whl` is the unmodified, pure-Python wheel from
[Tomli 2.2.1 on PyPI](https://pypi.org/project/tomli/2.2.1/). It supports Python
3.8+, is 14,257 bytes, and includes its MIT license and package metadata.
Python imports the wheel directly as a ZIP; it is not installed globally.
The helper verifies its SHA-256 before importing it:

```text
cb55c73c5f4408779d0cf3eef9f762b9c9f147a77de7b258bef0a5628adc85cc
```

Ship the wheel beside the helper at `/scripts/` in every frontend build. When
updating it, verify the release's PyPI digest, preserve its license, and update
the filename in both bootstraps and the helper's filename/checksum together.

## Validation

From the repository root:

```sh
sh -n frontend/public/scripts/sub2api-local-setup.sh
python3 -m unittest discover -s frontend/public/scripts/tests -v
pnpm --dir frontend exec vitest run src/utils/__tests__/localSetupCommand.spec.ts
```

Bootstrap tests run real Python with fixture downloads, isolated configuration
and cache directories, including a piped Bash invocation. They cover existing
config preservation, invalid TOML, parser integrity, missing Python, symlinked
cached runtimes and temporary-file cleanup without changing user configuration.
