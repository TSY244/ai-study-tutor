import copy
import unittest

from verify_compose_boundary import BoundaryViolation, validate_compose_boundary


RUNTIME_HOME = "/var/lib/ai-study-tutor"
SETTINGS_FILE = f"{RUNTIME_HOME}/.claude/settings.json"
GATEWAY_DIR = "/config/claude-gateway"
GATEWAY_FILE = f"{GATEWAY_DIR}/keys.json"


def base_compose_config() -> dict:
    shared_environment = {
        "HOME": RUNTIME_HOME,
        "APP_DATA_DIR": RUNTIME_HOME,
        "CLAUDE_CODE_SETTINGS_FILE": SETTINGS_FILE,
        "ANTHROPIC_BASE_URL": "https://gateway.example/anthropic",
        "CLAUDE_GATEWAY_CONFIG_FILE": GATEWAY_FILE,
        "URL_ALLOWLIST": "youtube.com,bilibili.com",
    }
    shared_volumes = [
        {
            "type": "volume",
            "source": "app_data",
            "target": RUNTIME_HOME,
            "volume": {},
        },
        {
            "type": "bind",
            "source": "/safe/runtime/claude-gateway",
            "target": GATEWAY_DIR,
            "bind": {},
        },
    ]
    return {
        "services": {
            "api": {
                "environment": {
                    **shared_environment,
                    "CLAUDE_GATEWAY_WRITE_ENABLED": "true",
                    "URL_API_DNS_VALIDATION": "false",
                },
                "volumes": copy.deepcopy(shared_volumes),
                "secrets": [
                    {
                        "source": "postgres_password",
                        "target": "/run/secrets/postgres_password",
                    }
                ],
            },
            "worker": {
                "environment": {
                    **shared_environment,
                    "ANTHROPIC_API_KEY_FILE": "/run/secrets/anthropic_api_key",
                },
                "volumes": copy.deepcopy(shared_volumes),
                "secrets": [
                    {
                        "source": "postgres_password",
                        "target": "/run/secrets/postgres_password",
                    },
                    {
                        "source": "anthropic_api_key",
                        "target": "/run/secrets/anthropic_api_key",
                    },
                ],
            },
            "web": {
                "environment": {"API_UPSTREAM": "api:8080"},
                "volumes": [],
                "secrets": [],
            },
            "db": {
                "environment": {"POSTGRES_DB": "ai_study_tutor"},
                "volumes": [],
                "secrets": [],
            },
        }
    }


class ComposeBoundaryTest(unittest.TestCase):
    def assert_violation(self, config: dict, code: str) -> None:
        with self.assertRaises(BoundaryViolation) as caught:
            validate_compose_boundary(config)
        message = str(caught.exception)
        self.assertIn(code, message)
        self.assertNotIn("token-do-not-print", message)
        self.assertNotIn("https://private-provider.example", message)

    def test_accepts_expected_runtime_boundary(self) -> None:
        validate_compose_boundary(base_compose_config())

    def test_accepts_unrelated_mounts_with_similar_path_prefixes(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"].append(
            {"type": "bind", "source": "/safe/config", "target": "/config/cookies"}
        )
        config["services"]["web"]["volumes"].append(
            {"type": "bind", "source": "/safe/runtime-other", "target": "/web-runtime"}
        )

        validate_compose_boundary(config)

    def test_rejects_read_only_api_app_data_mount(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"][0]["read_only"] = True

        self.assert_violation(config, "api.app_data_mount")

    def test_rejects_mismatched_app_data_dir(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["environment"]["APP_DATA_DIR"] = "/wrong/app-data"

        self.assert_violation(config, "api.runtime_environment")

    def test_rejects_mount_properties_split_across_unrelated_objects(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"][:1] = [
            {
                "type": "volume",
                "source": "app_data",
                "target": "/unrelated-target",
            },
            {
                "type": "bind",
                "source": "/safe/unrelated-source",
                "target": RUNTIME_HOME,
            },
        ]

        self.assert_violation(config, "api.app_data_mount")

    def test_rejects_web_app_data_source_mount(self) -> None:
        config = base_compose_config()
        config["services"]["web"]["volumes"] = [
            {"type": "volume", "source": "app_data", "target": "/web-data"}
        ]

        self.assert_violation(config, "web.runtime_mount")

    def test_rejects_web_runtime_home_target(self) -> None:
        config = base_compose_config()
        config["services"]["web"]["volumes"] = [
            {"type": "volume", "source": "web_data", "target": RUNTIME_HOME}
        ]

        self.assert_violation(config, "web.runtime_mount")

    def test_rejects_host_claude_directory_overlay_in_api(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"].append(
            {
                "type": "bind",
                "source": "/safe/host-home/.claude",
                "target": f"{RUNTIME_HOME}/.claude",
            }
        )

        self.assert_violation(config, "api.runtime_settings_mount")

    def test_rejects_read_only_settings_file_overlay_in_worker(self) -> None:
        config = base_compose_config()
        config["services"]["worker"]["volumes"].append(
            {
                "type": "bind",
                "source": "/safe/host-home/settings.json",
                "target": SETTINGS_FILE,
                "read_only": True,
            }
        )

        self.assert_violation(config, "worker.runtime_settings_mount")

    def test_rejects_ancestor_mount_that_shadows_runtime_settings(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"].append(
            {"type": "bind", "source": "/safe/var-lib", "target": "/var/lib"}
        )

        self.assert_violation(config, "api.runtime_settings_mount")

    def test_rejects_web_mount_of_gateway_source_at_unrelated_target(self) -> None:
        config = base_compose_config()
        gateway_source = config["services"]["api"]["volumes"][1]["source"]
        config["services"]["web"]["volumes"].append(
            {"type": "bind", "source": gateway_source, "target": "/unrelated-gateway-copy"}
        )

        self.assert_violation(config, "gateway.source_boundary")

    def test_rejects_web_mount_of_gateway_source_parent(self) -> None:
        config = base_compose_config()
        config["services"]["web"]["volumes"].append(
            {"type": "bind", "source": "/safe/runtime", "target": "/unrelated-runtime"}
        )

        self.assert_violation(config, "gateway.source_boundary")

    def test_rejects_web_mount_of_gateway_source_descendant(self) -> None:
        config = base_compose_config()
        config["services"]["web"]["volumes"].append(
            {
                "type": "bind",
                "source": "/safe/runtime/claude-gateway/keys.json",
                "target": "/unrelated-gateway-key",
            }
        )

        self.assert_violation(config, "gateway.source_boundary")

    def test_rejects_worker_gateway_file_overlay(self) -> None:
        config = base_compose_config()
        config["services"]["worker"]["volumes"].append(
            {
                "type": "bind",
                "source": "/safe/other-keys.json",
                "target": GATEWAY_FILE,
                "read_only": True,
            }
        )

        self.assert_violation(config, "worker.gateway_target_overlay")

    def test_rejects_api_gateway_ancestor_overlay(self) -> None:
        config = base_compose_config()
        config["services"]["api"]["volumes"].append(
            {"type": "bind", "source": "/safe/config", "target": "/config"}
        )

        self.assert_violation(config, "api.gateway_target_overlay")

    def test_rejects_different_gateway_sources_for_api_and_worker(self) -> None:
        config = base_compose_config()
        config["services"]["worker"]["volumes"][1]["source"] = "/safe/other-gateway"

        self.assert_violation(config, "gateway.mount_identity")

    def test_rejects_different_app_data_sources_for_api_and_worker(self) -> None:
        config = base_compose_config()
        config["services"]["worker"]["volumes"][0]["source"] = "other_app_data"

        self.assert_violation(config, "app_data.mount_identity")

    def test_rejects_app_data_source_from_other_unauthorized_service(self) -> None:
        config = base_compose_config()
        app_data_source = config["services"]["api"]["volumes"][0]["source"]
        config["services"]["db"]["volumes"].append(
            {"type": "volume", "source": app_data_source, "target": "/unrelated-app-data"}
        )

        self.assert_violation(config, "app_data.service_boundary")

    def test_rejects_other_boundary_regressions_without_echoing_values(self) -> None:
        cases = {
            "runtime HOME": (
                lambda config: config["services"]["worker"]["environment"].update(
                    {"HOME": "/wrong/home"}
                ),
                "worker.runtime_environment",
            ),
            "settings path": (
                lambda config: config["services"]["worker"]["environment"].update(
                    {"CLAUDE_CODE_SETTINGS_FILE": "/wrong/settings.json"}
                ),
                "worker.runtime_environment",
            ),
            "base URL mismatch": (
                lambda config: config["services"]["worker"]["environment"].update(
                    {"ANTHROPIC_BASE_URL": "https://private-provider.example"}
                ),
                "runtime.base_url",
            ),
            "missing API gateway env": (
                lambda config: config["services"]["api"]["environment"].pop(
                    "CLAUDE_GATEWAY_CONFIG_FILE"
                ),
                "api.gateway",
            ),
            "missing Worker gateway mount": (
                lambda config: config["services"]["worker"].update(
                    {
                        "volumes": [
                            volume
                            for volume in config["services"]["worker"]["volumes"]
                            if volume.get("target") != GATEWAY_DIR
                        ]
                    }
                ),
                "worker.gateway",
            ),
            "Worker write gate": (
                lambda config: config["services"]["worker"]["environment"].update(
                    {"CLAUDE_GATEWAY_WRITE_ENABLED": "true"}
                ),
                "gateway.write_gate",
            ),
            "legacy secret outside Worker": (
                lambda config: config["services"]["api"]["secrets"].append(
                    {"source": "anthropic_api_key"}
                ),
                "legacy_secret.boundary",
            ),
            "missing Worker legacy secret": (
                lambda config: config["services"]["worker"].update(
                    {
                        "secrets": [
                            secret
                            for secret in config["services"]["worker"]["secrets"]
                            if secret.get("source") != "anthropic_api_key"
                        ]
                    }
                ),
                "legacy_secret.boundary",
            ),
            "plain API Key": (
                lambda config: config["services"]["api"]["environment"].update(
                    {"ANTHROPIC_API_KEY": "token-do-not-print"}
                ),
                "environment.plain_api_key",
            ),
            "Web Claude env": (
                lambda config: config["services"]["web"]["environment"].update(
                    {"CLAUDE_CONFIG": "token-do-not-print"}
                ),
                "web.claude_environment",
            ),
            "legacy settings directory env": (
                lambda config: config["services"]["api"]["environment"].update(
                    {"CLAUDE_CODE_SETTINGS_DIR": "/old/settings"}
                ),
                "legacy_settings.boundary",
            ),
            "legacy settings mount": (
                lambda config: config["services"]["api"]["volumes"].append(
                    {"type": "bind", "source": "/safe/old", "target": "/claude-code"}
                ),
                "legacy_settings.boundary",
            ),
            "gateway outside API and Worker": (
                lambda config: config["services"]["web"]["volumes"].append(
                    {"type": "bind", "source": "/safe/gateway", "target": GATEWAY_DIR}
                ),
                "gateway.service_boundary",
            ),
        }

        for name, (mutate, code) in cases.items():
            with self.subTest(name=name):
                config = base_compose_config()
                mutate(config)
                self.assert_violation(config, code)


if __name__ == "__main__":
    unittest.main()
