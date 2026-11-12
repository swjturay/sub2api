#!/usr/bin/env python3
"""Generate the reviewed parallel CodeArts Pipeline definition."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import re


JOB_ID_RE = re.compile(r"^[0-9a-f]{32}$")
PARALLEL_JOBS = (
    ("validate-release", "发布工具验证"),
    ("validate-frontend", "原站前端验证"),
    ("validate-insights", "Insights验证"),
    ("validate-backend", "Backend验证"),
    ("build-backend", "Backend镜像"),
    ("build-insights", "Insights镜像"),
)


def build_step(job_id: str, identifier: str) -> dict:
    if not JOB_ID_RE.fullmatch(job_id):
        raise ValueError(f"CodeArts Build job ID is invalid: {job_id!r}")
    return {
        "runtime_attribution": "agent",
        "multi_step_editable": 0,
        "official_task_version": "0.0.7",
        "icon_url": "",
        "name": "CodeArts构建",
        "task": "official_devcloud_cloudBuild",
        "business_type": "Build",
        "inputs": [
            {"key": "RELEASE_VERSION", "value": "${PIPELINE_NUMBER}"},
            {"key": "SOURCE_SHA", "value": "${SOURCE_SHA}"},
            {"key": "__sameProject__", "value": "true"},
            {"key": "__projectId__", "value": ""},
            {"key": "jobId", "value": job_id},
            {"key": "__repository__", "value": "hs-flash-web"},
            {"key": "artifactIdentifier", "value": ""},
        ],
        "env": [],
        "sequence": 0,
        "identifier": f"step-{identifier}",
        "endpoint_ids": [],
        "timeout-minutes": None,
        "continue-on-error": None,
    }


def build_job(key: str, name: str, job_id: str, sequence: int) -> dict:
    return {
        "id": "",
        "name": name,
        "identifier": f"job-{key}",
        "sequence": sequence,
        "async": None,
        "condition": None,
        "strategy": None,
        "timeout": "",
        "resource": None,
        "uses": None,
        "with": None,
        "secrets": None,
        "secrets_inherit": False,
        "outputs_define": None,
        "steps": [build_step(job_id, key)],
        "matrix_context": None,
        "concurrency": None,
        "env": None,
        "depends_on": [],
        "timeout-minutes": None,
        "continue-on-error": None,
    }


def stage(name: str, identifier: str, sequence: int, jobs: list[dict]) -> dict:
    return {
        "category": None,
        "name": name,
        "identifier": identifier,
        "run_condition": None,
        "type": None,
        "sequence": sequence,
        "parallel": None,
        "pre": [],
        "post": [],
        "jobs": jobs,
        "depends_on": [],
        "run_always": False,
        "fail_fast": True,
    }


def pipeline_definition(job_ids: dict[str, str]) -> dict:
    expected = {key for key, _ in PARALLEL_JOBS} | {"finalize"}
    if set(job_ids) != expected:
        raise ValueError(f"job ID map must contain exactly {sorted(expected)}")
    parallel = [
        build_job(key, name, job_ids[key], sequence)
        for sequence, (key, name) in enumerate(PARALLEL_JOBS)
    ]
    final = [build_job("finalize", "生成固定发布制品", job_ids["finalize"], 0)]
    return {
        "stages": [
            stage("并行验证与镜像构建", "stage-parallel-build", 0, parallel),
            stage("生成固定发布制品", "stage-finalize-bundle", 1, final),
        ]
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--job-ids", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    job_ids = json.loads(args.job_ids.read_text(encoding="utf-8"))
    args.output.write_text(
        json.dumps(pipeline_definition(job_ids), indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
