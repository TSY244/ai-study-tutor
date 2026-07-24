# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml==6.0.2"]
# ///
"""Publish and verify the OpenAPI contract.

Commands:
  generate  Expand x-common-response-headers from the design source
            (docs/web-rebuild/openapi.yaml) into every success/error
            Response and write the published contract api/openapi.yaml.
  verify    Fail unless api/openapi.yaml is byte-identical to a fresh
            generation, every published Response declares X-Request-ID,
            all local $refs resolve, and every operationId is unique.
  breaking  Compare the current published contract against a baseline and
            fail on operation/request/response/schema changes that would break
            the generated frontend client. Baseline resolution:
              1. OPENAPI_BASELINE=/path/to/base.yaml
              2. api/openapi.baseline.yaml (checked-in snapshot)
              3. git show HEAD:api/openapi.yaml
              4. no baseline -> explicit skip with exit 0

Deterministic: pinned PyYAML, fixed dump settings, no timestamps or host
state in the output. Run via the Makefile targets generate-openapi /
verify-openapi / check-openapi-breaking. Never edit api/openapi.yaml by hand.
"""

from __future__ import annotations

import copy
import os
import subprocess
import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
DESIGN_PATH = REPO_ROOT / "docs" / "web-rebuild" / "openapi.yaml"
PUBLISHED_PATH = REPO_ROOT / "api" / "openapi.yaml"

GENERATED_BANNER = """# GENERATED FILE - DO NOT EDIT.
# Source of truth (design source): docs/web-rebuild/openapi.yaml
# Regenerate with: make generate-openapi  (tools/openapi/openapi_contract.py generate)
# Verify with:     make verify-openapi
# Rules: see api/README.md

"""

HTTP_METHODS = ("get", "put", "post", "delete", "options", "head", "patch", "trace")
# Key order used when a Response object is rewritten with merged headers.
RESPONSE_KEY_ORDER = ("description", "headers", "content")


def fail(message: str) -> "SystemExit":
    print(f"ERROR: {message}", file=sys.stderr)
    return SystemExit(1)


def load_yaml(path: Path) -> dict:
    with path.open(encoding="utf-8") as fh:
        data = yaml.safe_load(fh)
    if not isinstance(data, dict):
        raise fail(f"{path} 不是合法的 YAML mapping")
    return data


def iter_operations(doc: dict):
    """Yield (path, method, operation) for every operation in the document."""
    for path, item in (doc.get("paths") or {}).items():
        if not isinstance(item, dict):
            continue
        for method, operation in item.items():
            if method in HTTP_METHODS and isinstance(operation, dict):
                yield path, method, operation


def merge_common_headers(response: dict, common_headers: dict) -> dict:
    """Return a Response object with common headers merged in.

    Existing response-specific headers always win; common headers are only
    added when absent. Key order is normalized to description/headers/content
    first so the published file is stable and review-friendly.
    """
    merged_headers = dict(common_headers)
    merged_headers.update(response.get("headers") or {})

    rewritten: dict = {}
    for key in RESPONSE_KEY_ORDER:
        if key == "headers":
            rewritten["headers"] = merged_headers
        elif key in response:
            rewritten[key] = response[key]
    for key, value in response.items():
        if key not in rewritten:
            rewritten[key] = value
    if "headers" not in rewritten:
        rewritten["headers"] = merged_headers
    return rewritten


def expand(doc: dict) -> dict:
    """Expand x-common-response-headers into every Response object."""
    doc = copy.deepcopy(doc)
    common = doc.pop("x-common-response-headers", None)
    doc.pop("x-generation-rules", None)
    if not isinstance(common, dict) or not common:
        raise fail("设计源缺少顶层 x-common-response-headers")

    # Inline response objects in operations.
    for path, method, operation in iter_operations(doc):
        responses = operation.get("responses")
        if not isinstance(responses, dict) or not responses:
            raise fail(f"{method.upper()} {path} 缺少 responses")
        for status, response in responses.items():
            if isinstance(response, dict) and "$ref" not in response:
                responses[status] = merge_common_headers(response, common)

    # Reusable component responses (targets of response $refs).
    component_responses = (doc.get("components") or {}).get("responses") or {}
    for name, response in component_responses.items():
        if isinstance(response, dict) and "$ref" not in response:
            component_responses[name] = merge_common_headers(response, common)

    return doc


def render(doc: dict) -> str:
    body = yaml.dump(
        doc,
        sort_keys=False,
        allow_unicode=True,
        default_flow_style=False,
        width=120,
    )
    return GENERATED_BANNER + body


def generate() -> int:
    doc = expand(load_yaml(DESIGN_PATH))
    output = render(doc)
    PUBLISHED_PATH.parent.mkdir(parents=True, exist_ok=True)
    PUBLISHED_PATH.write_text(output, encoding="utf-8")
    print(f"wrote {PUBLISHED_PATH.relative_to(REPO_ROOT)} from {DESIGN_PATH.relative_to(REPO_ROOT)}")
    return 0


# ---------------------------------------------------------------------------
# verify
# ---------------------------------------------------------------------------

class RefResolver:
    def __init__(self, doc: dict):
        self.doc = doc

    def resolve(self, ref: str) -> object:
        if not ref.startswith("#/"):
            raise fail(f"发布契约只允许本地引用，发现外部引用：{ref}")
        node: object = self.doc
        for segment in ref[2:].split("/"):
            segment = segment.replace("~1", "/").replace("~0", "~")
            if not isinstance(node, dict) or segment not in node:
                raise fail(f"引用无法解析：{ref}")
            node = node[segment]
        return node

    def deref(self, node: object) -> dict:
        seen = 0
        while isinstance(node, dict) and "$ref" in node:
            node = self.resolve(node["$ref"])
            seen += 1
            if seen > 10:
                raise fail("引用链过长或存在循环引用")
        if not isinstance(node, dict):
            raise fail("引用目标不是对象")
        return node


def check_refs(doc: dict) -> int:
    resolver = RefResolver(doc)
    count = 0

    def walk(node: object) -> None:
        nonlocal count
        if isinstance(node, dict):
            for key, value in node.items():
                if key == "$ref":
                    resolver.resolve(value)
                    count += 1
                else:
                    walk(value)
        elif isinstance(node, list):
            for item in node:
                walk(item)

    walk(doc)
    return count


def check_operation_ids(doc: dict) -> int:
    seen: dict[str, str] = {}
    for path, method, operation in iter_operations(doc):
        op_id = operation.get("operationId")
        if not op_id:
            raise fail(f"{method.upper()} {path} 缺少 operationId")
        if op_id in seen:
            raise fail(f"operationId 重复：{op_id}（{seen[op_id]} 与 {method.upper()} {path}）")
        seen[op_id] = f"{method.upper()} {path}"
    return len(seen)


def check_request_id_header(doc: dict) -> int:
    resolver = RefResolver(doc)
    checked = 0

    def assert_has_request_id(response_node: dict, where: str) -> None:
        response = resolver.deref(response_node)
        headers = response.get("headers") or {}
        if "X-Request-ID" not in headers:
            raise fail(f"{where} 缺少 X-Request-ID 响应头")
        resolver.deref(headers["X-Request-ID"])  # header 本身或其 $ref 必须可解析

    for path, method, operation in iter_operations(doc):
        for status, response in (operation.get("responses") or {}).items():
            assert_has_request_id(response, f"{method.upper()} {path} 响应 {status}")
            checked += 1
    for name, response in ((doc.get("components") or {}).get("responses") or {}).items():
        assert_has_request_id(response, f"components.responses.{name}")
        checked += 1
    return checked


def verify() -> int:
    if not PUBLISHED_PATH.exists():
        raise fail(f"{PUBLISHED_PATH.relative_to(REPO_ROOT)} 不存在，请先运行 make generate-openapi")

    published_text = PUBLISHED_PATH.read_text(encoding="utf-8")
    doc = load_yaml(PUBLISHED_PATH)

    version = str(doc.get("openapi", ""))
    if not version.startswith("3.1."):
        raise fail(f"发布契约 openapi 版本必须是 3.1.x，实际：{version!r}")

    for key in ("x-common-response-headers", "x-generation-rules"):
        if key in doc:
            raise fail(f"发布契约不得保留设计源扩展 {key}（应在生成时展开/移除）")

    expected = render(expand(load_yaml(DESIGN_PATH)))
    if published_text != expected:
        raise fail(
            "api/openapi.yaml 与设计源不一致（可能是手工修改或设计源已更新）。"
            "禁止手改发布契约：请修改 docs/web-rebuild/openapi.yaml 后运行 make generate-openapi"
        )

    ref_count = check_refs(doc)
    op_count = check_operation_ids(doc)
    response_count = check_request_id_header(doc)
    print(
        f"verify-openapi OK: {op_count} operations, {response_count} responses "
        f"(all declare X-Request-ID), {ref_count} refs resolved, published == generated"
    )
    return 0


# ---------------------------------------------------------------------------
# breaking
# ---------------------------------------------------------------------------

def load_baseline() -> tuple[dict | None, str]:
    explicit = os.environ.get("OPENAPI_BASELINE")
    if explicit:
        path = Path(explicit)
        if not path.exists():
            raise fail(f"OPENAPI_BASELINE 不存在：{path}")
        return load_yaml(path), str(path)

    checked_in = REPO_ROOT / "api" / "openapi.baseline.yaml"
    if checked_in.exists():
        return load_yaml(checked_in), "api/openapi.baseline.yaml"

    proc = subprocess.run(
        ["git", "show", "HEAD:api/openapi.yaml"],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if proc.returncode != 0:
        return None, "HEAD:api/openapi.yaml"
    data = yaml.safe_load(proc.stdout)
    if not isinstance(data, dict):
        raise fail("HEAD:api/openapi.yaml 不是合法 YAML mapping")
    return data, "HEAD:api/openapi.yaml"


def sorted_keys(mapping: object) -> list[str]:
    return sorted(mapping.keys()) if isinstance(mapping, dict) else []


def response_statuses(operation: dict) -> set[str]:
    return set((operation.get("responses") or {}).keys())


def parameter_key(parameter: dict) -> tuple[str, str]:
    return str(parameter.get("in", "")), str(parameter.get("name", ""))


def compare_operations(base: dict, current: dict, issues: list[str]) -> None:
    base_resolver = RefResolver(base)
    current_resolver = RefResolver(current)
    base_paths = base.get("paths") or {}
    current_paths = current.get("paths") or {}
    for path in sorted_keys(base_paths):
        if path not in current_paths:
            issues.append(f"removed path: {path}")
            continue
        base_item = base_paths[path] or {}
        current_item = current_paths[path] or {}
        for method in HTTP_METHODS:
            if method not in base_item:
                continue
            if method not in current_item:
                issues.append(f"removed operation: {method.upper()} {path}")
                continue
            base_op = base_item[method] or {}
            current_op = current_item[method] or {}
            if base_op.get("operationId") and base_op.get("operationId") != current_op.get("operationId"):
                issues.append(
                    f"changed operationId for {method.upper()} {path}: "
                    f"{base_op.get('operationId')} -> {current_op.get('operationId')}"
                )
            for status in sorted(response_statuses(base_op) - response_statuses(current_op)):
                issues.append(f"removed response {status}: {method.upper()} {path}")
            compare_request_body(
                base_op,
                current_op,
                f"{method.upper()} {path}",
                base_resolver,
                current_resolver,
                issues,
            )
            compare_operation_responses(
                base_op,
                current_op,
                f"{method.upper()} {path}",
                base_resolver,
                current_resolver,
                issues,
            )
            compare_parameters(
                base_item,
                base_op,
                current_item,
                current_op,
                f"{method.upper()} {path}",
                base_resolver,
                current_resolver,
                issues,
            )


def deref_parameter(parameter: dict, resolver: RefResolver) -> dict:
    return resolver.deref(parameter) if "$ref" in parameter else parameter


def operation_parameters(path_item: dict, operation: dict, resolver: RefResolver) -> dict[tuple[str, str], dict]:
    out: dict[tuple[str, str], dict] = {}
    for p in (path_item.get("parameters") or []) + (operation.get("parameters") or []):
        if isinstance(p, dict):
            param = deref_parameter(p, resolver)
            out[parameter_key(param)] = param
    return out


def compare_parameters(
    base_item: dict,
    base_op: dict,
    current_item: dict,
    current_op: dict,
    where: str,
    base_resolver: RefResolver,
    current_resolver: RefResolver,
    issues: list[str],
) -> None:
    base = operation_parameters(base_item, base_op, base_resolver)
    current = operation_parameters(current_item, current_op, current_resolver)
    for key, param in base.items():
        if not param.get("required"):
            continue
        if key not in current:
            issues.append(f"removed required parameter {key[0]}:{key[1]} in {where}")
    for key, param in current.items():
        if not param.get("required"):
            continue
        if key not in base or not base[key].get("required"):
            issues.append(f"added required parameter {key[0]}:{key[1]} in {where}")


def deref_optional(node: object, resolver: RefResolver) -> dict | None:
    if not isinstance(node, dict):
        return None
    return resolver.deref(node)


def media_schemas(container: dict, resolver: RefResolver) -> dict[str, dict]:
    resolved = resolver.deref(container)
    out: dict[str, dict] = {}
    for media_type, media in (resolved.get("content") or {}).items():
        if not isinstance(media, dict) or not isinstance(media.get("schema"), dict):
            continue
        out[str(media_type)] = media["schema"]
    return out


def compare_request_body(
    base_op: dict,
    current_op: dict,
    where: str,
    base_resolver: RefResolver,
    current_resolver: RefResolver,
    issues: list[str],
) -> None:
    base_body = deref_optional(base_op.get("requestBody"), base_resolver)
    current_body = deref_optional(current_op.get("requestBody"), current_resolver)
    if base_body is None:
        if current_body is not None and current_body.get("required"):
            issues.append(f"added required request body in {where}")
        return
    if current_body is None:
        issues.append(f"removed request body in {where}")
        return
    if not base_body.get("required") and current_body.get("required"):
        issues.append(f"made request body required in {where}")

    base_media = media_schemas(base_body, base_resolver)
    current_media = media_schemas(current_body, current_resolver)
    for media_type, base_schema in base_media.items():
        if media_type not in current_media:
            issues.append(f"removed request media type {media_type} in {where}")
            continue
        compare_request_schema(
            base_schema,
            current_media[media_type],
            f"{where} request {media_type}",
            base_resolver,
            current_resolver,
            issues,
        )


def compare_operation_responses(
    base_op: dict,
    current_op: dict,
    where: str,
    base_resolver: RefResolver,
    current_resolver: RefResolver,
    issues: list[str],
) -> None:
    base_responses = base_op.get("responses") or {}
    current_responses = current_op.get("responses") or {}
    for status in sorted(set(base_responses) & set(current_responses)):
        base_response = base_responses[status]
        current_response = current_responses[status]
        if not isinstance(base_response, dict) or not isinstance(current_response, dict):
            continue
        base_media = media_schemas(base_response, base_resolver)
        current_media = media_schemas(current_response, current_resolver)
        for media_type, base_schema in base_media.items():
            if media_type not in current_media:
                issues.append(f"removed response media type {media_type}: {where} {status}")
                continue
            current_schema = current_media[media_type]
            base_ref = base_schema.get("$ref")
            current_ref = current_schema.get("$ref")
            if base_ref and current_ref and base_ref != current_ref:
                issues.append(
                    f"changed response schema for {where} {status} {media_type}: "
                    f"{base_ref} -> {current_ref}"
                )
            compare_schema(
                base_schema,
                current_schema,
                f"{where} response {status} {media_type}",
                base_resolver,
                current_resolver,
                issues,
            )


def compare_request_schema(
    base_schema: dict,
    current_schema: dict,
    where: str,
    base_resolver: RefResolver,
    current_resolver: RefResolver,
    issues: list[str],
) -> None:
    base_schema = base_resolver.deref(base_schema)
    current_schema = current_resolver.deref(current_schema)
    added_required = as_set(current_schema.get("required")) - as_set(base_schema.get("required"))
    if added_required:
        issues.append(f"added required request fields at {where}: {sorted(added_required)}")
    compare_schema(base_schema, current_schema, where, base_resolver, current_resolver, issues)
    base_props = base_schema.get("properties") or {}
    current_props = current_schema.get("properties") or {}
    for prop in sorted(set(base_props) & set(current_props)):
        if isinstance(base_props[prop], dict) and isinstance(current_props[prop], dict):
            compare_request_schema(
                base_props[prop],
                current_props[prop],
                f"{where}.{prop}",
                base_resolver,
                current_resolver,
                issues,
            )


def compare_component_schemas(base: dict, current: dict, issues: list[str]) -> None:
    base_resolver = RefResolver(base)
    current_resolver = RefResolver(current)
    base_schemas = ((base.get("components") or {}).get("schemas") or {})
    current_schemas = ((current.get("components") or {}).get("schemas") or {})
    for name in sorted_keys(base_schemas):
        if name not in current_schemas:
            issues.append(f"removed schema: components.schemas.{name}")
            continue
        compare_schema(
            base_resolver.deref(base_schemas[name]),
            current_resolver.deref(current_schemas[name]),
            f"components.schemas.{name}",
            base_resolver,
            current_resolver,
            issues,
        )


def as_set(value: object) -> set[str]:
    if isinstance(value, list):
        return {str(v) for v in value}
    if isinstance(value, str):
        return {value}
    return set()


def compare_schema(
    base_schema: dict,
    current_schema: dict,
    where: str,
    base_resolver: RefResolver,
    current_resolver: RefResolver,
    issues: list[str],
) -> None:
    base_schema = base_resolver.deref(base_schema)
    current_schema = current_resolver.deref(current_schema)

    base_types = as_set(base_schema.get("type"))
    current_types = as_set(current_schema.get("type"))
    if base_types and current_types and not base_types.issubset(current_types):
        issues.append(f"narrowed/changed type at {where}: {sorted(base_types)} -> {sorted(current_types)}")

    base_enum = as_set(base_schema.get("enum"))
    current_enum = as_set(current_schema.get("enum"))
    if base_enum and current_enum and not base_enum.issubset(current_enum):
        issues.append(f"removed enum values at {where}: {sorted(base_enum - current_enum)}")

    base_required = as_set(base_schema.get("required"))
    current_required = as_set(current_schema.get("required"))
    if base_required and not base_required.issubset(current_required):
        issues.append(f"removed required fields at {where}: {sorted(base_required - current_required)}")

    base_props = base_schema.get("properties") or {}
    current_props = current_schema.get("properties") or {}
    for prop in sorted_keys(base_props):
        if prop not in current_props:
            issues.append(f"removed property at {where}.{prop}")
            continue
        if isinstance(base_props[prop], dict) and isinstance(current_props[prop], dict):
            compare_schema(
                base_props[prop],
                current_props[prop],
                f"{where}.{prop}",
                base_resolver,
                current_resolver,
                issues,
            )

    base_items = base_schema.get("items")
    current_items = current_schema.get("items")
    if isinstance(base_items, dict):
        if not isinstance(current_items, dict):
            issues.append(f"removed array item schema at {where}")
        else:
            compare_schema(
                base_items,
                current_items,
                f"{where}[]",
                base_resolver,
                current_resolver,
                issues,
            )

    for keyword in ("allOf", "anyOf", "oneOf"):
        base_branches = base_schema.get(keyword)
        current_branches = current_schema.get(keyword)
        if not isinstance(base_branches, list):
            continue
        if not isinstance(current_branches, list) or len(current_branches) < len(base_branches):
            issues.append(f"removed {keyword} branch at {where}")
            continue
        for index, base_branch in enumerate(base_branches):
            current_branch = current_branches[index]
            if isinstance(base_branch, dict) and isinstance(current_branch, dict):
                compare_schema(
                    base_branch,
                    current_branch,
                    f"{where}.{keyword}[{index}]",
                    base_resolver,
                    current_resolver,
                    issues,
                )


def breaking() -> int:
    if not PUBLISHED_PATH.exists():
        raise fail(f"{PUBLISHED_PATH.relative_to(REPO_ROOT)} 不存在，请先运行 make generate-openapi")
    current = load_yaml(PUBLISHED_PATH)
    baseline, label = load_baseline()
    if baseline is None:
        print(f"check-openapi-breaking SKIP: 未找到基线 {label}；当前阶段无法做破坏性对比")
        return 0
    issues: list[str] = []
    compare_operations(baseline, current, issues)
    compare_component_schemas(baseline, current, issues)
    if issues:
        print("ERROR: OpenAPI breaking changes detected:", file=sys.stderr)
        for issue in issues:
            print(f"  - {issue}", file=sys.stderr)
        return 1
    print(f"check-openapi-breaking OK: no removals or enum/required-field breaks vs {label}")
    return 0


def main(argv: list[str]) -> int:
    if len(argv) != 2 or argv[1] not in {"generate", "verify", "breaking"}:
        print(__doc__)
        return 2
    if argv[1] == "generate":
        return generate()
    if argv[1] == "verify":
        return verify()
    return breaking()


if __name__ == "__main__":
    sys.exit(main(sys.argv))
