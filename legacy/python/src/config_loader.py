"""Load and provide access to config.json with env-var overrides."""

from __future__ import annotations

import json
import os
from pathlib import Path
from typing import Any

CODEX_AGENT_COMMAND = "codex exec --sandbox workspace-write --ask-for-approval never"
CLAUDE_AGENT_COMMAND = "claude -p --permission-mode acceptEdits --output-format text"
DEFAULT_AGENT_COMMAND = CODEX_AGENT_COMMAND

AGENT_COMMAND_PRESETS = {
    "codex": CODEX_AGENT_COMMAND,
    "claude": CLAUDE_AGENT_COMMAND,
}


def resolve_agent_command(command: str | None) -> str:
    """Resolve an agent preset name or raw command into an executable command."""
    if not isinstance(command, str) or not command.strip():
        return DEFAULT_AGENT_COMMAND
    command = command.strip()
    return AGENT_COMMAND_PRESETS.get(command.lower(), command)


class ConfigError(RuntimeError):
    """Raised when config is missing or invalid."""


class Config:
    """Wraps config.json with env-var overrides.

    Env vars take precedence over config.json:
      OPENAI_API_KEY   → api_key
      OPENAI_BASE_URL  → base_url
      OPENAI_MODEL     → model
      TUTOR_COURSE_PATH → default_course_path
      TUTOR_CONFIG     → config file path
    """

    def __init__(self, config_path: Path | str | None = None) -> None:
        if config_path is None:
            config_path = Path(os.getenv("TUTOR_CONFIG", "config.json"))
        self.config_path = Path(config_path)
        self._data = self._load()

    def _load(self) -> dict[str, Any]:
        if not self.config_path.exists():
            raise ConfigError(f"配置文件不存在：{self.config_path}")
        with open(self.config_path, "r", encoding="utf-8") as f:
            return json.load(f)

    @property
    def ai_config(self) -> dict[str, Any]:
        return self._data.get("ai_config", {})

    @property
    def default_provider(self) -> str:
        return self.ai_config.get("default_provider", "openai_compat")

    def provider_config(self, provider: str | None = None, model_override: str | None = None) -> dict[str, Any]:
        """Return provider config with env-var and CLI overrides applied.

        Precedence: CLI --model > env var > config.json
        """
        provider = provider or self.default_provider
        providers = self.ai_config.get("providers", {})
        cfg = dict(providers.get(provider, providers.get("openai_compat", {})))

        # Env var overrides
        if os.getenv("OPENAI_API_KEY"):
            cfg["api_key"] = os.getenv("OPENAI_API_KEY")
        if os.getenv("OPENAI_BASE_URL"):
            cfg["base_url"] = os.getenv("OPENAI_BASE_URL")
        if os.getenv("OPENAI_MODEL"):
            cfg["model"] = os.getenv("OPENAI_MODEL")

        # CLI --model override (highest priority)
        if model_override:
            cfg["model"] = model_override

        return cfg

    @property
    def max_token(self) -> int:
        return self.ai_config.get("max_token", 4096)

    @property
    def agent_command(self) -> str:
        command = self._data.get("agent_command", self.ai_config.get("agent_command", ""))
        return resolve_agent_command(command)

    @property
    def course_path(self) -> Path:
        env_path = os.getenv("TUTOR_COURSE_PATH")
        if env_path:
            return Path(env_path)
        return Path(self._data.get("default_course_path", "./course"))

    @property
    def score_rule(self) -> dict[str, int]:
        return self._data.get("score_rule", {"choice": 2, "judge": 1, "short_answer": 5})

    @property
    def download_sub_only(self) -> bool:
        return self._data.get("download_sub_only", True)


# Module-level singleton
_config: Config | None = None


def get_config(config_path: Path | str | None = None) -> Config:
    global _config
    if _config is None:
        _config = Config(config_path)
    return _config
