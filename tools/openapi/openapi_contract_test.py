# /// script
# requires-python = ">=3.11"
# dependencies = ["pyyaml==6.0.2"]
# ///
"""Focused compatibility tests for openapi_contract.py."""

from __future__ import annotations

import copy
import unittest

from openapi_contract import compare_operations


def contract() -> dict:
    return {
        "openapi": "3.1.0",
        "paths": {
            "/items": {
                "post": {
                    "operationId": "createItem",
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {
                                "schema": {"$ref": "#/components/schemas/ItemCreate"}
                            }
                        },
                    },
                    "responses": {
                        "200": {
                            "description": "item",
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/Item"}
                                }
                            },
                        }
                    },
                }
            }
        },
        "components": {
            "schemas": {
                "ItemCreate": {
                    "type": "object",
                    "required": ["title"],
                    "properties": {"title": {"type": "string"}, "note": {"type": "string"}},
                },
                "Item": {
                    "type": "object",
                    "required": ["id"],
                    "properties": {"id": {"type": "string"}},
                },
                "OtherItem": {
                    "type": "object",
                    "required": ["id"],
                    "properties": {"id": {"type": "string"}},
                },
            }
        },
    }


class BreakingContractTests(unittest.TestCase):
    def test_added_required_request_field_is_breaking(self) -> None:
        base = contract()
        current = copy.deepcopy(base)
        current["components"]["schemas"]["ItemCreate"]["required"].append("note")
        issues: list[str] = []
        compare_operations(base, current, issues)
        self.assertTrue(any("required request" in issue for issue in issues), issues)

    def test_changed_response_schema_reference_is_breaking(self) -> None:
        base = contract()
        current = copy.deepcopy(base)
        current["paths"]["/items"]["post"]["responses"]["200"]["content"][
            "application/json"
        ]["schema"] = {"$ref": "#/components/schemas/OtherItem"}
        issues: list[str] = []
        compare_operations(base, current, issues)
        self.assertTrue(any("changed response schema" in issue for issue in issues), issues)


if __name__ == "__main__":
    unittest.main()
