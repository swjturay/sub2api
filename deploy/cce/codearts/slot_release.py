#!/usr/bin/env python3
"""Run a bounded blue/green Sub2API release from inside the CCE namespace."""

from __future__ import annotations

import argparse
import copy
import datetime as dt
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import uuid

from release import (
    ANNOTATION_PREFIX,
    ReleaseError,
    build_backend_patch,
    build_insights_patch,
    load_manifest,
    named_container,
    public_checks,
    release_annotations,
    run,
    save_json,
    utc_now,
)


SLOTS = ("blue", "green")
SLOT_LABEL = f"{ANNOTATION_PREFIX}release-slot"
LOCK_NAME = "sub2api-release-lock"
LOCK_STALE_SECONDS = 30 * 60


def slot_deployment_name(base: str, slot: str) -> str:
    if slot not in SLOTS:
        raise ReleaseError(f"unsupported release slot: {slot}")
    return f"{base}-{slot}"


def other_slot(active: str | None) -> str:
    if active is None:
        return "blue"
    if active not in SLOTS:
        raise ReleaseError(f"unsupported active release slot: {active}")
    return "green" if active == "blue" else "blue"


def active_slot(selector: dict) -> str | None:
    value = selector.get(SLOT_LABEL)
    if value is None:
        return None
    if value not in SLOTS:
        raise ReleaseError(f"service has unsupported {SLOT_LABEL}={value!r}")
    return value


def sanitized_deployment(
    source: dict,
    *,
    name: str,
    slot: str,
    image: str,
    container_name: str,
    annotations: dict[str, str],
    insights_configmap: str = "",
) -> dict:
    if container_name == "sub2api":
        patch = build_backend_patch(source, image, annotations)
    elif container_name == "insights":
        patch = build_insights_patch(source, image, annotations, insights_configmap)
    else:
        raise ReleaseError(f"unsupported release container {container_name!r}")

    source_spec = copy.deepcopy(source["spec"])
    source_spec.update(copy.deepcopy(patch["spec"]))
    selector = copy.deepcopy(source_spec["selector"])
    match_labels = selector.setdefault("matchLabels", {})
    match_labels[SLOT_LABEL] = slot
    source_spec["selector"] = selector
    template = source_spec["template"]
    template_metadata = template.setdefault("metadata", {})
    template_labels = copy.deepcopy(
        source["spec"]["template"].get("metadata", {}).get("labels", {})
    )
    template_labels[SLOT_LABEL] = slot
    template_metadata["labels"] = template_labels
    template_annotations = template_metadata.setdefault("annotations", {})
    template_annotations.update(annotations)
    source_spec["replicas"] = max(1, int(source["spec"].get("replicas", 1)))
    source_spec["revisionHistoryLimit"] = min(3, int(source_spec.get("revisionHistoryLimit", 3)))
    source_spec.pop("paused", None)

    labels = copy.deepcopy(source.get("metadata", {}).get("labels", {}))
    labels[SLOT_LABEL] = slot
    return {
        "apiVersion": source.get("apiVersion", "apps/v1"),
        "kind": "Deployment",
        "metadata": {
            "name": name,
            "namespace": source["metadata"]["namespace"],
            "labels": labels,
            "annotations": annotations,
        },
        "spec": source_spec,
    }


def preview_service(source: dict, *, name: str, slot: str) -> dict:
    spec = copy.deepcopy(source["spec"])
    for key in (
        "clusterIP",
        "clusterIPs",
        "healthCheckNodePort",
        "ipFamilies",
        "ipFamilyPolicy",
    ):
        spec.pop(key, None)
    spec["type"] = "ClusterIP"
    spec.pop("externalTrafficPolicy", None)
    spec.pop("internalTrafficPolicy", None)
    spec["selector"] = copy.deepcopy(source["spec"].get("selector", {}))
    spec["selector"].pop("pod-template-hash", None)
    spec["selector"][SLOT_LABEL] = slot
    return {
        "apiVersion": "v1",
        "kind": "Service",
        "metadata": {
            "name": name,
            "namespace": source["metadata"]["namespace"],
            "labels": copy.deepcopy(source.get("metadata", {}).get("labels", {})),
        },
        "spec": spec,
    }


class SlotController:
    def __init__(self, namespace: str, audit_dir: Path):
        self.namespace = namespace
        self.audit_dir = audit_dir
        self.kubectl = ["kubectl", "-n", namespace]

    def command(self, *args: str, timeout: int = 180, check: bool = True):
        return run([*self.kubectl, *args], timeout=timeout, check=check)

    def get(self, kind: str, name: str) -> dict:
        result = self.command("get", kind, name, "-o", "json")
        return json.loads(result.stdout)

    def apply(self, value: dict) -> None:
        result = subprocess.run(
            [*self.kubectl, "apply", "--server-side", "--field-manager=sub2api-release", "-f", "-"],
            input=json.dumps(value),
            text=True,
            capture_output=True,
            timeout=180,
            check=False,
        )
        if result.returncode:
            raise ReleaseError(f"kubectl apply failed: {result.stderr.strip()}")

    def patch(self, kind: str, name: str, patch: dict) -> None:
        self.command("patch", kind, name, "--type", "merge", "-p", json.dumps(patch))

    def scale(self, name: str, replicas: int) -> None:
        self.command("scale", "deployment", name, f"--replicas={replicas}")

    def scale_if_exists(self, name: str, replicas: int) -> None:
        found = self.command("get", "deployment", name, check=False)
        if found.returncode == 0:
            self.scale(name, replicas)

    def rollout(self, name: str, timeout: int = 420) -> None:
        self.command(
            "rollout", "status", f"deployment/{name}", f"--timeout={timeout}s", timeout=timeout + 15
        )

    def delete(self, kind: str, name: str) -> None:
        self.command("delete", kind, name, "--ignore-not-found=true", check=False)

    def wait_slot_absent(self, slot: str, timeout: int = 120) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            result = self.command("get", "pods", "-l", f"{SLOT_LABEL}={slot}", "-o", "json")
            pods = [
                pod
                for pod in json.loads(result.stdout)["items"]
                if not pod.get("metadata", {}).get("deletionTimestamp")
            ]
            if not pods:
                return
            time.sleep(2)
        raise ReleaseError(f"pods for release slot {slot} did not terminate")

    def wait_service_endpoints(self, name: str, minimum: int, timeout: int = 60) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            result = self.command("get", "endpoints", name, "-o", "json", check=False)
            if result.returncode == 0:
                value = json.loads(result.stdout)
                addresses = sum(
                    len(subset.get("addresses", [])) for subset in value.get("subsets", [])
                )
                if addresses >= minimum:
                    return
            time.sleep(1)
        raise ReleaseError(f"service {name} did not publish {minimum} ready endpoints")

    def acquire_lock(self, release_id: str) -> None:
        now = int(time.time())
        lock = {
            "apiVersion": "v1",
            "kind": "ConfigMap",
            "metadata": {"name": LOCK_NAME, "namespace": self.namespace},
            "data": {"release_id": release_id, "created_at": str(now)},
        }
        result = subprocess.run(
            [*self.kubectl, "create", "-f", "-"],
            input=json.dumps(lock),
            text=True,
            capture_output=True,
            timeout=30,
            check=False,
        )
        if result.returncode == 0:
            return
        try:
            current = self.get("configmap", LOCK_NAME)
            created_at = int(current.get("data", {}).get("created_at", "0"))
        except (ReleaseError, ValueError, json.JSONDecodeError):
            created_at = now
        if now - created_at <= LOCK_STALE_SECONDS:
            raise ReleaseError("another CCE release is already running")
        self.delete("configmap", LOCK_NAME)
        retry = subprocess.run(
            [*self.kubectl, "create", "-f", "-"],
            input=json.dumps(lock),
            text=True,
            capture_output=True,
            timeout=30,
            check=False,
        )
        if retry.returncode:
            raise ReleaseError(f"failed to replace stale release lock: {retry.stderr.strip()}")

    def release_lock(self, release_id: str) -> None:
        try:
            current = self.get("configmap", LOCK_NAME)
        except ReleaseError:
            return
        if current.get("data", {}).get("release_id") == release_id:
            self.delete("configmap", LOCK_NAME)


def unique_ready_hash(controller: SlotController, deployment: dict) -> str:
    labels = deployment["spec"]["selector"]["matchLabels"]
    selector = ",".join(f"{key}={value}" for key, value in sorted(labels.items()))
    result = controller.command("get", "pods", "-l", selector, "-o", "json")
    pods = json.loads(result.stdout)["items"]
    hashes = {
        pod.get("metadata", {}).get("labels", {}).get("pod-template-hash")
        for pod in pods
        if not pod.get("metadata", {}).get("deletionTimestamp")
        and pod.get("status", {}).get("phase") == "Running"
        and all(item.get("ready") for item in pod.get("status", {}).get("containerStatuses", []))
    }
    hashes.discard(None)
    if len(hashes) != 1:
        raise ReleaseError(
            f"deployment {deployment['metadata']['name']} does not have one clean ready pod hash"
        )
    return next(iter(hashes))


def wait_http(url: str, expected: int, timeout: int = 90) -> bytes:
    deadline = time.monotonic() + timeout
    last_error = "no response"
    while time.monotonic() < deadline:
        request = urllib.request.Request(url, headers={"User-Agent": "sub2api-cce-slot-release/1"})
        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                body = response.read()
                if response.status == expected:
                    return body
                last_error = f"HTTP {response.status}"
        except urllib.error.HTTPError as error:
            if error.code == expected:
                return error.read()
            last_error = f"HTTP {error.code}"
        except (OSError, urllib.error.URLError) as error:
            last_error = str(error)
        time.sleep(2)
    raise ReleaseError(f"probe {url} did not return {expected}: {last_error}")


def candidate_checks() -> dict:
    backend_health = wait_http("http://sub2api-release-preview:8080/health", 200)
    wait_http("http://sub2api-release-preview:8080/api/v1/settings/public", 200)
    insights_index = wait_http("http://sub2api-insights-release-preview:8080/insights/", 200)
    return {"backend_health_bytes": len(backend_health), "insights_index_bytes": len(insights_index)}


def resilient_public_checks(attempts: int = 3) -> dict:
    last_error: Exception | None = None
    for attempt in range(attempts):
        try:
            return public_checks()
        except Exception as error:  # noqa: BLE001 - retain final probe failure
            last_error = error
            if attempt + 1 < attempts:
                time.sleep(1)
    assert last_error is not None
    raise last_error


def verify_deployment(controller: SlotController, name: str, container: str, image: str) -> dict:
    deployment = controller.get("deployment", name)
    desired = int(deployment["spec"].get("replicas", 1))
    status = deployment.get("status", {})
    for field in ("updatedReplicas", "readyReplicas", "availableReplicas"):
        if int(status.get(field, 0)) != desired:
            raise ReleaseError(f"deployment {name} {field} is not {desired}")
    if named_container(deployment["spec"]["template"]["spec"], container).get("image") != image:
        raise ReleaseError(f"deployment {name} does not use the expected immutable image")
    return {"name": name, "replicas": desired, "image": image}


def service_selector(base: dict, slot: str, pod_hash: str = "") -> dict:
    selector = copy.deepcopy(base["spec"].get("selector", {}))
    selector.pop("pod-template-hash", None)
    selector.pop(SLOT_LABEL, None)
    if pod_hash:
        selector["pod-template-hash"] = pod_hash
    if slot:
        selector[SLOT_LABEL] = slot
    return selector


def release(args: argparse.Namespace) -> dict:
    manifest = load_manifest(args.manifest.resolve())
    release_id = uuid.uuid4().hex[:20]
    release_name = f"{dt.datetime.now(dt.timezone.utc):%Y%m%dT%H%M%SZ}-{manifest['source_sha'][:12]}"
    audit_dir = args.audit_root / release_name
    audit_dir.mkdir(parents=True, mode=0o700)
    controller = SlotController(args.namespace, audit_dir)
    annotations = release_annotations(manifest, release_id)
    annotations[f"{ANNOTATION_PREFIX}release-system"] = "codearts-slot-v1"
    controller.acquire_lock(release_id)

    switched = False
    deadline_reached = False
    candidate_names: list[str] = []
    preview_names = ["sub2api-release-preview", "sub2api-insights-release-preview"]
    originals: dict[str, dict] = {}
    pinned_selectors: dict[str, dict] = {}
    active_names: dict[str, str] = {}
    inactive = ""

    try:
        backend_service = controller.get("service", args.backend_service)
        insights_service = controller.get("service", args.insights_service)
        originals = {
            args.backend_service: copy.deepcopy(backend_service["spec"]["selector"]),
            args.insights_service: copy.deepcopy(insights_service["spec"]["selector"]),
        }
        backend_active = active_slot(backend_service["spec"]["selector"])
        insights_active = active_slot(insights_service["spec"]["selector"])
        if backend_active != insights_active:
            raise ReleaseError("backend and Insights services are on different release slots")
        active = backend_active
        inactive = other_slot(active)
        active_names = {
            "backend": args.backend_deployment if active is None else slot_deployment_name(args.backend_deployment, active),
            "insights": args.insights_deployment if active is None else slot_deployment_name(args.insights_deployment, active),
        }
        backend_before = controller.get("deployment", active_names["backend"])
        insights_before = controller.get("deployment", active_names["insights"])
        current_shas = {
            value.get("metadata", {}).get("annotations", {}).get(f"{ANNOTATION_PREFIX}source-revision")
            for value in (backend_before, insights_before)
        }
        if len(current_shas) != 1:
            raise ReleaseError("active backend and Insights source revisions differ")
        if args.expected_current_sha and current_shas != {args.expected_current_sha}:
            raise ReleaseError(
                f"production source changed: {sorted(str(value) for value in current_shas)}"
            )

        backend_candidate_name = slot_deployment_name(args.backend_deployment, inactive)
        insights_candidate_name = slot_deployment_name(args.insights_deployment, inactive)
        candidate_names = [backend_candidate_name, insights_candidate_name]
        backend_candidate = sanitized_deployment(
            backend_before,
            name=backend_candidate_name,
            slot=inactive,
            image=manifest["images"]["backend"]["immutable_image"],
            container_name="sub2api",
            annotations=annotations,
        )
        insights_candidate = sanitized_deployment(
            insights_before,
            name=insights_candidate_name,
            slot=inactive,
            image=manifest["images"]["insights"]["immutable_image"],
            container_name="insights",
            annotations=annotations,
            insights_configmap=args.insights_configmap,
        )
        save_json(audit_dir / "manifest.json", manifest)
        save_json(audit_dir / "backend-before.json", backend_before)
        save_json(audit_dir / "insights-before.json", insights_before)
        save_json(audit_dir / "backend-candidate.json", backend_candidate)
        save_json(audit_dir / "insights-candidate.json", insights_candidate)
        if args.dry_run:
            return {
                "phase": "dry_run_complete",
                "active_slot": active or "legacy",
                "candidate_slot": inactive,
                "audit_dir": str(audit_dir),
            }

        if active is None:
            for service_name, service, deployment in (
                (args.backend_service, backend_service, backend_before),
                (args.insights_service, insights_service, insights_before),
            ):
                pod_hash = unique_ready_hash(controller, deployment)
                pinned = service_selector(service, "", pod_hash)
                controller.patch("service", service_name, {"spec": {"selector": pinned}})
                pinned_selectors[service_name] = pinned
                controller.patch("deployment", deployment["metadata"]["name"], {"spec": {"paused": True}})
        else:
            pinned_selectors = copy.deepcopy(originals)

        controller.apply(backend_candidate)
        controller.apply(insights_candidate)
        controller.apply(
            preview_service(backend_service, name=preview_names[0], slot=inactive)
        )
        controller.apply(
            preview_service(insights_service, name=preview_names[1], slot=inactive)
        )
        controller.rollout(backend_candidate_name)
        controller.rollout(insights_candidate_name)
        backend_state = verify_deployment(
            controller,
            backend_candidate_name,
            "sub2api",
            manifest["images"]["backend"]["immutable_image"],
        )
        insights_state = verify_deployment(
            controller,
            insights_candidate_name,
            "insights",
            manifest["images"]["insights"]["immutable_image"],
        )
        candidate = candidate_checks()

        controller.patch(
            "service",
            args.backend_service,
            {"spec": {"selector": service_selector(backend_service, inactive)}},
        )
        controller.patch(
            "service",
            args.insights_service,
            {"spec": {"selector": service_selector(insights_service, inactive)}},
        )
        switched = True
        controller.wait_service_endpoints(args.backend_service, backend_state["replicas"])
        controller.wait_service_endpoints(args.insights_service, insights_state["replicas"])

        samples = []
        deadline = time.monotonic() + args.drain_seconds
        while True:
            sample = {"at": utc_now(), **resilient_public_checks()}
            samples.append(sample)
            if time.monotonic() >= deadline:
                break
            time.sleep(min(args.probe_interval, max(0, deadline - time.monotonic())))
        deadline_reached = True

        controller.scale(active_names["backend"], 0)
        controller.scale(active_names["insights"], 0)
        for name in preview_names:
            controller.delete("service", name)

        result = {
            "phase": "success",
            "at": utc_now(),
            "source_sha": manifest["source_sha"],
            "version": manifest["version"],
            "active_slot_before": active or "legacy",
            "active_slot_after": inactive,
            "drain_seconds": args.drain_seconds,
            "forcedDrain": True,
            "backend": backend_state,
            "insights": insights_state,
            "candidate": candidate,
            "public_samples": len(samples),
            "audit_dir": str(audit_dir),
        }
        save_json(audit_dir / "probes.json", samples)
        save_json(audit_dir / "success.json", result)
        return result
    except Exception as error:
        rollback_errors = []
        if switched and not deadline_reached:
            for service_name, selector in pinned_selectors.items():
                try:
                    controller.patch("service", service_name, {"spec": {"selector": selector}})
                except Exception as rollback_error:  # noqa: BLE001
                    rollback_errors.append(f"restore {service_name}: {rollback_error}")
        candidate_stopped = False
        if not deadline_reached:
            for name in candidate_names:
                try:
                    controller.scale_if_exists(name, 0)
                except Exception as rollback_error:  # noqa: BLE001
                    rollback_errors.append(f"scale {name}: {rollback_error}")
            if inactive:
                try:
                    controller.wait_slot_absent(inactive)
                    candidate_stopped = True
                except Exception as rollback_error:  # noqa: BLE001
                    rollback_errors.append(f"wait {inactive}: {rollback_error}")
        if not deadline_reached and candidate_stopped and active_slot(originals.get(args.backend_service, {})) is None:
            for service_name, selector in originals.items():
                try:
                    controller.patch("service", service_name, {"spec": {"selector": selector}})
                except Exception as rollback_error:  # noqa: BLE001
                    rollback_errors.append(f"unpin {service_name}: {rollback_error}")
            for name in active_names.values():
                try:
                    controller.patch("deployment", name, {"spec": {"paused": False}})
                except Exception as rollback_error:  # noqa: BLE001
                    rollback_errors.append(f"unpause {name}: {rollback_error}")
        for name in preview_names:
            controller.delete("service", name)
        failure = {
            "phase": (
                "cleanup_failed_after_cutover"
                if deadline_reached
                else "rolled_back" if not rollback_errors else "rollback_incomplete"
            ),
            "at": utc_now(),
            "error": str(error),
            "rollback_errors": rollback_errors,
        }
        save_json(audit_dir / "failure.json", failure)
        raise ReleaseError(json.dumps(failure, sort_keys=True)) from error
    finally:
        controller.release_lock(release_id)


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description=__doc__)
    result.add_argument("--manifest", type=Path, required=True)
    result.add_argument("--namespace", default=os.environ.get("POD_NAMESPACE", "sub2api-prod"))
    result.add_argument("--backend-deployment", default="sub2api")
    result.add_argument("--backend-service", default="sub2api")
    result.add_argument("--insights-deployment", default="sub2api-insights")
    result.add_argument("--insights-service", default="sub2api-insights")
    result.add_argument("--insights-configmap", default="sub2api-insights-nginx")
    result.add_argument("--expected-current-sha", default="")
    result.add_argument("--drain-seconds", type=int, default=180)
    result.add_argument("--probe-interval", type=int, default=3)
    result.add_argument("--audit-root", type=Path, default=Path("/audit"))
    result.add_argument("--dry-run", action="store_true")
    return result


def main() -> int:
    args = parser().parse_args()
    if args.drain_seconds < 0 or args.drain_seconds > 180:
        raise ReleaseError("drain-seconds must be between 0 and 180")
    if args.probe_interval < 1 or args.probe_interval > 30:
        raise ReleaseError("probe-interval must be between 1 and 30")
    args.audit_root.mkdir(parents=True, exist_ok=True)
    print(json.dumps(release(args), sort_keys=True), flush=True)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (ReleaseError, json.JSONDecodeError, OSError, subprocess.TimeoutExpired) as error:
        print(f"release failed: {error}", file=os.sys.stderr)
        raise SystemExit(1)
