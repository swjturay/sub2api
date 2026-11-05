#!/usr/bin/env python3
"""Resolve one ACR tag to an immutable digest without printing credentials."""

from __future__ import annotations

import argparse
import base64
import json
import os
import re
import urllib.error
import urllib.parse
import urllib.request


IMAGE_RE = re.compile(
    r"^(?P<registry>registry\.cn-shanghai\.aliyuncs\.com)/(?P<repository>rayless/sub2api):(?P<tag>[A-Za-z0-9_.-]+)$"
)
DIGEST_RE = re.compile(r"^sha256:[0-9a-f]{64}$")
AUTH_RE = re.compile(r'^Bearer\s+realm="([^"]+)",service="([^"]+)",scope="([^"]+)"$')
ACCEPT = ", ".join(
    (
        "application/vnd.oci.image.manifest.v1+json",
        "application/vnd.docker.distribution.manifest.v2+json",
        "application/vnd.docker.distribution.manifest.list.v2+json",
    )
)


def basic_header(username: str, password: str) -> str:
    value = base64.b64encode(f"{username}:{password}".encode()).decode("ascii")
    return f"Basic {value}"


def resolve(image: str, username: str, password: str) -> str:
    match = IMAGE_RE.fullmatch(image)
    if not match:
        raise ValueError("image must be a production ACR tag")
    registry = match.group("registry")
    repository = match.group("repository")
    tag = match.group("tag")
    url = f"https://{registry}/v2/{repository}/manifests/{tag}"
    request = urllib.request.Request(
        url,
        method="HEAD",
        headers={"Accept": ACCEPT, "Authorization": basic_header(username, password)},
    )
    try:
        response = urllib.request.urlopen(request, timeout=30)
    except urllib.error.HTTPError as error:
        if error.code != 401:
            raise
        challenge = error.headers.get("WWW-Authenticate", "")
        auth = AUTH_RE.fullmatch(challenge)
        if not auth:
            raise ValueError("registry returned an unsupported authentication challenge") from error
        realm, service, scope = auth.groups()
        token_url = realm + "?" + urllib.parse.urlencode({"service": service, "scope": scope})
        token_request = urllib.request.Request(
            token_url, headers={"Authorization": basic_header(username, password)}
        )
        with urllib.request.urlopen(token_request, timeout=30) as token_response:
            token_data = json.load(token_response)
        token = token_data.get("token") or token_data.get("access_token")
        if not token:
            raise ValueError("registry token response did not contain a token")
        request = urllib.request.Request(
            url, method="HEAD", headers={"Accept": ACCEPT, "Authorization": f"Bearer {token}"}
        )
        response = urllib.request.urlopen(request, timeout=30)
    with response:
        digest = response.headers.get("Docker-Content-Digest", "")
    if not DIGEST_RE.fullmatch(digest):
        raise ValueError("registry did not return an immutable sha256 digest")
    return f"{registry}/{repository}@{digest}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    args = parser.parse_args()
    username = os.environ.get("ACR_USERNAME", "")
    password = os.environ.get("ACR_PASSWORD", "")
    if not username or not password:
        raise SystemExit("ACR_USERNAME and ACR_PASSWORD are required")
    print(resolve(args.image, username, password))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
