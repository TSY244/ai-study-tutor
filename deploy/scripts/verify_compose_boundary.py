#!/usr/bin/env python3

import json
import sys
from collections.abc import Mapping
from typing import Any


RUNTIME_HOME = "/var/lib/ai-study-tutor"
SETTINGS_FILE = f"{RUNTIME_HOME}/.claude/settings.json"
GATEWAY_DIR = "/config/claude-gateway"
GATEWAY_FILE = f"{GATEWAY_DIR}/keys.json"
LEGACY_SECRET = "anthropic_api_key"


class BoundaryViolation(ValueError):
    pass


def _mapping(value: Any) -> Mapping[str, Any]:
    return value if isinstance(value, Mapping) else {}


def _service(config: Mapping[str, Any], name: str, issues: set[str]) -> Mapping[str, Any]:
    services = _mapping(config.get("services"))
    service = services.get(name)
    if not isinstance(service, Mapping):
        issues.add(f"{name}.service")
        return {}
    return service


def _environment(service: Mapping[str, Any]) -> Mapping[str, Any]:
    return _mapping(service.get("environment"))


def _volumes(service: Mapping[str, Any]) -> list[Mapping[str, Any]]:
    volumes = service.get("volumes")
    if not isinstance(volumes, list):
        return []
    return [volume for volume in volumes if isinstance(volume, Mapping)]


def _secret_sources(service: Mapping[str, Any]) -> set[str]:
    secrets = service.get("secrets")
    if not isinstance(secrets, list):
        return set()
    sources: set[str] = set()
    for secret in secrets:
        if isinstance(secret, str):
            sources.add(secret)
        elif isinstance(secret, Mapping) and isinstance(secret.get("source"), str):
            sources.add(secret["source"])
    return sources


def _has_writable_app_data_mount(service: Mapping[str, Any]) -> bool:
    return any(
        volume.get("type") == "volume"
        and volume.get("source") == "app_data"
        and volume.get("target") == RUNTIME_HOME
        and volume.get("read_only", False) is False
        for volume in _volumes(service)
    )


def _has_gateway_mount(service: Mapping[str, Any]) -> bool:
    return any(volume.get("target") == GATEWAY_DIR for volume in _volumes(service))


def _has_legacy_settings_mount(service: Mapping[str, Any]) -> bool:
    for volume in _volumes(service):
        target = volume.get("target")
        if isinstance(target, str) and (target == "/claude-code" or target.startswith("/claude-code/")):
            return True
    return False


def validate_compose_boundary(config: dict) -> None:
    if not isinstance(config, Mapping):
        raise BoundaryViolation("compose boundary validation failed: compose.document")

    issues: set[str] = set()
    services = _mapping(config.get("services"))
    api = _service(config, "api", issues)
    worker = _service(config, "worker", issues)
    web = _service(config, "web", issues)
    runtime_services = {"api": api, "worker": worker}

    expected_runtime_environment = {
        "HOME": RUNTIME_HOME,
        "APP_DATA_DIR": RUNTIME_HOME,
        "CLAUDE_CODE_SETTINGS_FILE": SETTINGS_FILE,
    }
    for name, service in runtime_services.items():
        environment = _environment(service)
        if any(environment.get(key) != value for key, value in expected_runtime_environment.items()):
            issues.add(f"{name}.runtime_environment")
        if not _has_writable_app_data_mount(service):
            issues.add(f"{name}.app_data_mount")
        if environment.get("CLAUDE_GATEWAY_CONFIG_FILE") != GATEWAY_FILE or not _has_gateway_mount(
            service
        ):
            issues.add(f"{name}.gateway")

    api_environment = _environment(api)
    worker_environment = _environment(worker)
    api_base_url = api_environment.get("ANTHROPIC_BASE_URL")
    worker_base_url = worker_environment.get("ANTHROPIC_BASE_URL")
    if (
        not isinstance(api_base_url, str)
        or not api_base_url.strip()
        or api_base_url != worker_base_url
    ):
        issues.add("runtime.base_url")

    if api_environment.get("CLAUDE_GATEWAY_WRITE_ENABLED") != "true":
        issues.add("gateway.write_gate")
    for name, service in services.items():
        if name != "api" and "CLAUDE_GATEWAY_WRITE_ENABLED" in _environment(_mapping(service)):
            issues.add("gateway.write_gate")

    for name, service in services.items():
        if name in runtime_services:
            continue
        normalized_service = _mapping(service)
        if (
            "CLAUDE_GATEWAY_CONFIG_FILE" in _environment(normalized_service)
            or _has_gateway_mount(normalized_service)
        ):
            issues.add("gateway.service_boundary")

    legacy_secret_services = {
        name
        for name, service in services.items()
        if LEGACY_SECRET in _secret_sources(_mapping(service))
    }
    if legacy_secret_services != {"worker"}:
        issues.add("legacy_secret.boundary")

    for service in services.values():
        environment = _environment(_mapping(service))
        if "ANTHROPIC_API_KEY" in environment:
            issues.add("environment.plain_api_key")
        if "CLAUDE_CODE_SETTINGS_DIR" in environment:
            issues.add("legacy_settings.boundary")
        if _has_legacy_settings_mount(_mapping(service)):
            issues.add("legacy_settings.boundary")

    web_environment = _environment(web)
    if any("CLAUDE" in key.upper() or "ANTHROPIC" in key.upper() for key in web_environment):
        issues.add("web.claude_environment")
    for volume in _volumes(web):
        target = volume.get("target")
        if volume.get("source") == "app_data" or (
            isinstance(target, str)
            and (target == RUNTIME_HOME or target.startswith(f"{RUNTIME_HOME}/"))
        ):
            issues.add("web.runtime_mount")

    api_allowlist = api_environment.get("URL_ALLOWLIST")
    worker_allowlist = worker_environment.get("URL_ALLOWLIST")
    if (
        not isinstance(api_allowlist, str)
        or not api_allowlist.strip()
        or api_allowlist != worker_allowlist
    ):
        issues.add("url_policy.allowlist")
    if api_environment.get("URL_API_DNS_VALIDATION") not in ("false", False):
        issues.add("url_policy.api_dns_validation")

    if issues:
        raise BoundaryViolation(
            "compose boundary validation failed: " + ", ".join(sorted(issues))
        )


def main() -> int:
    try:
        config = json.load(sys.stdin)
    except (json.JSONDecodeError, OSError):
        print("compose boundary validation failed: compose.invalid_json", file=sys.stderr)
        return 1

    try:
        validate_compose_boundary(config)
    except BoundaryViolation as error:
        print(error, file=sys.stderr)
        return 1

    print("secret boundary verified: legacy anthropic_api_key is mounted only by worker")
    print(
        "gateway boundary verified: api and worker share writable runtime Claude settings; web is clean"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
