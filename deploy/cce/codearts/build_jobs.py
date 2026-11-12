#!/usr/bin/env python3
"""Generate single-responsibility CodeArts Build task request bodies."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path


PROJECT_ID = "83bea33296754139a25d3eb97382d52c"
SOURCE_ENDPOINT = "a9787fe3be8749bb98da5b3f074dc325"
REGISTRY_ENDPOINT = "51013a427351443d9e36959e1229e6de"
SHELL_MODULE = "devcloud2018.codeci_action_20017.action"
DOCKER_MODULE = "devcloud2018.codeci_action_20004.action"
UPLOAD_MODULE = "devcloud2018.codeci_action_20018.action"

FETCH_SOURCE = r'''set -eu
case "$SOURCE_SHA" in
  [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]) ;;
  *) echo "SOURCE_SHA must be one full lowercase commit" >&2; exit 1 ;;
esac
case "$RELEASE_VERSION" in
  *[!A-Za-z0-9._-]*|'') echo "RELEASE_VERSION is unsafe" >&2; exit 1 ;;
esac
rm -rf sub2api-source source.tar.gz
attempt=1
while [ "$attempt" -le 5 ]; do
  if curl --fail --location --silent --show-error --connect-timeout 15 --max-time 300 \
      --output source.tar.gz "https://codeload.github.com/swjturay/sub2api/tar.gz/$SOURCE_SHA"; then
    top=$(tar -tzf source.tar.gz | awk -F/ 'NR == 1 { print $1 }')
    tar -xzf source.tar.gz
    mv "$top" sub2api-source
    break
  fi
  if [ "$attempt" -eq 5 ]; then
    echo "failed to fetch exact source archive after 5 attempts" >&2
    exit 1
  fi
  sleep $((attempt * 5))
  attempt=$((attempt + 1))
done
cd sub2api-source
PINNED_SOURCE_SHA="$SOURCE_SHA" sh deploy/cce/codearts/prepare_source.sh "$RELEASE_VERSION"
'''


def parameter(name: str, default: str, *, sensitive: bool = False) -> dict:
    return {
        "name": "hudson.model.StringParameterDefinition",
        "params": [
            {"name": "name", "value": name},
            {"name": "type", "value": "normalparam"},
            {"name": "defaultValue", "value": default},
            {"name": "staticVar", "value": "false"},
            {"name": "sensitiveVar", "value": "true" if sensitive else "false"},
            {"name": "description", "value": ""},
            {"name": "deletion", "value": "false"},
            {"name": "defaults", "value": "false"},
        ],
    }


def shell_step(name: str, command: str) -> dict:
    return {
        "properties": {
            "image": "shell4.2.46-git1.8.3-zip6.00",
            "preCondition": "SUCCESS",
            "command": command,
        },
        "module_id": SHELL_MODULE,
        "name": name,
        "version": None,
        "enable": True,
    }


def docker_step(name: str, dockerfile: str, context: str, tag: str) -> dict:
    return {
        "properties": {
            "registryUrl": {"displayName": "其它镜像仓库", "value": "other"},
            "setMetaData": "no",
            "image": "docker20.10",
            "buildArgList": [],
            "cache": "true",
            "imageName": "sub2api",
            "selectRegion": "",
            "contextPath": f"sub2api-source/{context}".rstrip("/"),
            "iamEndPoint": {},
            "authUser": {},
            "dockerFilePath": dockerfile,
            "organization": "rayless",
            "tag": tag,
            "endpointUuid": {"displayName": "sub2api-acr", "uuid": REGISTRY_ENDPOINT},
            "preCondition": "SUCCESS",
        },
        "module_id": DOCKER_MODULE,
        "name": name,
        "version": None,
        "enable": True,
    }


def upload_step(name: str, path: str) -> dict:
    return {
        "properties": {
            "image": "shell4.2.46-git1.8.3-zip6.00",
            "buildVersion": "${RELEASE_VERSION}",
            "file": path,
            "name": "",
            "remainOriginPath": "FLAT",
            "uploadTool": "curl",
            "customUploadPath": "sub2api",
            "preCondition": "SUCCESS",
        },
        "module_id": UPLOAD_MODULE,
        "name": name,
        "version": None,
        "enable": True,
    }


def base_task(name: str, steps: list[dict], *, secret_values: tuple[str, str] | None = None) -> dict:
    parameters = [parameter("SOURCE_SHA", "UNSET"), parameter("RELEASE_VERSION", "UNSET")]
    if secret_values is not None:
        parameters.extend(
            [
                parameter("ACR_USERNAME", secret_values[0], sensitive=True),
                parameter("ACR_PASSWORD", secret_values[1], sensitive=True),
            ]
        )
    return {
        "arch": "x86-64",
        "build_config_type": "ACTION",
        "build_if_code_updated": False,
        "flavor": "DEFAULT",
        "host_type": "devcloud",
        "job_name": name,
        "project_id": PROJECT_ID,
        "parameters": parameters,
        "scms": [
            {
                "url": "https://gitcode.com/gcw_1EmgBpdh/hs-flash-web.git",
                "branch": "main",
                "depth": "1",
                "source": "wukong",
                "is_auto_build": False,
                "scm_type": "general_git",
                "end_point_id": SOURCE_ENDPOINT,
                "build_type": "branch",
                "enable_git_lfs": False,
            }
        ],
        "steps": [shell_step("锁定精确cce-deploy源码", FETCH_SOURCE), *steps],
        "timeout": {"limit": "60", "unit": "minute"},
    }


def task_specs(acr_username: str = "", acr_password: str = "") -> dict[str, dict]:
    validation = {
        "validate-release": "release",
        "validate-frontend": "frontend",
        "validate-insights": "insights",
        "validate-backend": "backend",
    }
    result = {
        key: base_task(
            f"sub2api-{key}-v3",
            [
                docker_step(
                    f"{key}验证",
                    f"./deploy/cce/codearts/Dockerfile.validate.{suffix}",
                    "",
                    f"{key}-codearts-${{RELEASE_VERSION}}",
                )
            ],
        )
        for key, suffix in validation.items()
    }
    result["build-backend"] = base_task(
        "sub2api-build-backend-v3",
        [docker_step("构建Backend镜像", "./Dockerfile", "", "backend-codearts-${RELEASE_VERSION}")],
    )
    result["build-insights"] = base_task(
        "sub2api-build-insights-v3",
        [
            docker_step(
                "构建Insights镜像",
                "./Dockerfile",
                "insights",
                "insights-codearts-${RELEASE_VERSION}",
            )
        ],
    )
    finalize = shell_step(
        "生成固定发布制品",
        """set -eu
set +x
cd sub2api-source
export ACR_USERNAME='${ACR_USERNAME}'
export ACR_PASSWORD='${ACR_PASSWORD}'
sh deploy/cce/codearts/finalize_release.sh "$RELEASE_VERSION"
unset ACR_USERNAME ACR_PASSWORD
""",
    )
    result["finalize"] = base_task(
        "sub2api-finalize-bundle-v3",
        [
            finalize,
            upload_step("上传固定发布Bundle", "sub2api-source/release/sub2api-release-bundle.tar.gz"),
            upload_step(
                "上传Bundle校验和",
                "sub2api-source/release/sub2api-release-bundle.tar.gz.sha256",
            ),
            upload_step("上传不可变镜像清单", "sub2api-source/release/cce-release-manifest.json"),
        ],
        secret_values=(acr_username, acr_password),
    )
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()
    username = os.environ.get("ACR_USERNAME", "")
    password = os.environ.get("ACR_PASSWORD", "")
    args.output_dir.mkdir(parents=True, exist_ok=True)
    for key, body in task_specs(username, password).items():
        (args.output_dir / f"{key}.json").write_text(
            json.dumps({"body": body}, indent=2, ensure_ascii=False) + "\n",
            encoding="utf-8",
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
