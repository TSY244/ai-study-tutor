"""OpenAI-compatible API adapter.

Supports any OpenAI-compatible endpoint (OpenAI, DeepSeek, Claude via proxy, etc.)
by setting base_url in config.json. The model name is also configurable.
"""

from __future__ import annotations

from typing import Any

try:
    from openai import OpenAI
except ImportError:  # pragma: no cover
    OpenAI = None  # type: ignore


class AIAPIError(RuntimeError):
    """Raised when the AI API call fails."""


class ClaudeAPI:
    """OpenAI-compatible chat completions API adapter.

    Named ClaudeAPI for backward compatibility, but works with any
    OpenAI-compatible endpoint via base_url configuration.
    """

    def __init__(self, cfg: dict[str, Any]) -> None:
        self.api_key: str = cfg.get("api_key", "")
        self.model: str = cfg.get("model", "gpt-4.1-mini")
        self.base_url: str = cfg.get("base_url", "")
        self.max_tokens: int = cfg.get("max_token", 4096)

    # ------------------------------------------------------------------
    # Public interface (implements BaseAI implicitly)
    # ------------------------------------------------------------------

    def chat(
        self,
        prompt: str,
        system: str = "",
        max_tokens: int | None = None,
        temperature: float = 0.2,
    ) -> str:
        """Send a single user prompt and return the text response."""
        messages: list[dict[str, str]] = []
        if system:
            messages.append({"role": "system", "content": system})
        messages.append({"role": "user", "content": prompt})
        return self._create_message(messages, max_tokens=max_tokens, temperature=temperature)

    def chat_json(
        self,
        prompt: str,
        system: str = "",
        max_tokens: int | None = None,
        temperature: float = 0.2,
    ) -> str:
        """Send a prompt expecting JSON output."""
        json_system = "你必须返回严格 JSON，不要 Markdown 代码块，不要额外解释。"
        if system:
            json_system = system + "\n" + json_system
        return self.chat(prompt, system=json_system, max_tokens=max_tokens, temperature=temperature)

    # ------------------------------------------------------------------
    # Internal
    # ------------------------------------------------------------------

    def _create_message(
        self,
        messages: list[dict[str, str]],
        max_tokens: int | None = None,
        temperature: float = 0.2,
    ) -> str:
        if not self.api_key or self.api_key == "替换为你的API Key":
            raise AIAPIError(
                "请先设置 API Key（config.json 或 OPENAI_API_KEY 环境变量）。\n"
                "编辑 config.json 中的 api_key 字段，或设置环境变量：\n"
                "  export OPENAI_API_KEY='your-key'"
            )

        if OpenAI is None:
            raise AIAPIError("缺少 openai 依赖。请运行：pip install openai")

        kwargs: dict[str, Any] = {"api_key": self.api_key}
        if self.base_url:
            kwargs["base_url"] = self.base_url
        client = OpenAI(**kwargs)

        try:
            response = client.chat.completions.create(
                model=self.model,
                messages=messages,
                max_tokens=max_tokens or self.max_tokens,
                temperature=temperature,
            )
        except Exception as exc:
            raise AIAPIError(f"AI API 调用失败：{exc}") from exc

        content = response.choices[0].message.content
        if not content:
            raise AIAPIError("AI 返回内容为空。")
        return content
