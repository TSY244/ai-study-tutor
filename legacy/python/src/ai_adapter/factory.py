"""Factory function for creating AI adapter instances."""

from __future__ import annotations

from typing import Any

from .base_ai import BaseAI
from .claude_api import ClaudeAPI


def get_ai(provider: str, cfg: dict[str, Any]) -> BaseAI | ClaudeAPI:
    """Return an AI adapter for the given provider.

    Any OpenAI-compatible provider works — just configure it in config.json.
    The adapter is always ClaudeAPI (OpenAI-compatible chat completions).

    Args:
        provider: The provider name from config.json (e.g. "deepseek", "openai_compat").
        cfg: The full ai_config dict from config.json.

    Raises:
        ValueError: If the provider is not configured.
    """
    providers = cfg.get("providers", {})
    provider_cfg = providers.get(provider)
    if provider_cfg is None:
        raise ValueError(
            f"未配置该 AI 服务商：{provider}。请在 config.json 的 ai_config.providers 中添加。"
        )
    return ClaudeAPI(provider_cfg)
