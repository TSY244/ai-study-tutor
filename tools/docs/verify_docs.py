# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml==6.0.2"]
# ///
"""Verify the web-rebuild documentation set.

Checks (no network, no writes):
  1. docs/web-rebuild/README.md exists and its 文档导航 links resolve.
  2. Every relative markdown link in docs/web-rebuild/*.md resolves.
  3. The design source docs/web-rebuild/openapi.yaml parses and still
     declares x-common-response-headers + x-generation-rules (the inputs
     of the published-contract generation).
  4. The published contract api/openapi.yaml and its rule book
     api/README.md exist (content is verified by verify-openapi).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
DOCS_DIR = REPO_ROOT / "docs" / "web-rebuild"

LINK_PATTERN = re.compile(r"\[[^\]]*\]\(([^)]+)\)")

errors: list[str] = []


def fail(message: str) -> None:
    errors.append(message)


def check_nav_links() -> None:
    readme = DOCS_DIR / "README.md"
    if not readme.exists():
        fail("缺少 docs/web-rebuild/README.md")
        return
    nav_entries = [
        match.group(1)
        for line in readme.read_text(encoding="utf-8").splitlines()
        if (match := LINK_PATTERN.search(line)) and match.group(1).startswith("./")
    ]
    if len(nav_entries) < 10:
        fail("README.md 文档导航条目异常少，可能结构被破坏")
    for entry in nav_entries:
        target = entry.split("#", 1)[0]
        if target and not (DOCS_DIR / target).resolve().exists():
            fail(f"README.md 导航链接失效：{entry}")


def check_relative_links() -> None:
    for md in sorted(DOCS_DIR.glob("*.md")):
        for lineno, line in enumerate(md.read_text(encoding="utf-8").splitlines(), start=1):
            for match in LINK_PATTERN.finditer(line):
                target = match.group(1).split("#", 1)[0].strip()
                if not target or re.match(r"^[a-zA-Z][a-zA-Z0-9+.-]*:", target):
                    continue  # external URL or pure anchor
                if not (md.parent / target).resolve().exists():
                    fail(f"{md.name}:{lineno} 相对链接失效：{match.group(1)}")


def check_design_source() -> None:
    design = DOCS_DIR / "openapi.yaml"
    if not design.exists():
        fail("缺少设计源 docs/web-rebuild/openapi.yaml")
        return
    try:
        doc = yaml.safe_load(design.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        fail(f"设计源 openapi.yaml 无法解析：{exc}")
        return
    if not isinstance(doc, dict) or not str(doc.get("openapi", "")).startswith("3.1."):
        fail("设计源不是 OpenAPI 3.1.x 文档")
        return
    common = doc.get("x-common-response-headers")
    if not isinstance(common, dict) or "X-Request-ID" not in common:
        fail("设计源缺少 x-common-response-headers.X-Request-ID（展开输入）")
    rules = doc.get("x-generation-rules", {})
    if "common-response-headers" not in rules:
        fail("设计源缺少 x-generation-rules.common-response-headers 说明")


def check_published_artifacts_exist() -> None:
    for rel in ("api/openapi.yaml", "api/README.md"):
        if not (REPO_ROOT / rel).exists():
            fail(f"缺少 {rel}（运行 make generate-openapi 生成发布契约）")


def main() -> int:
    check_nav_links()
    check_relative_links()
    check_design_source()
    check_published_artifacts_exist()
    if errors:
        for message in errors:
            print(f"ERROR: {message}", file=sys.stderr)
        return 1
    print("verify-docs OK: web-rebuild 文档导航与相对链接有效，设计源声明完整，发布产物存在")
    return 0


if __name__ == "__main__":
    sys.exit(main())
