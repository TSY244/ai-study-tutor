"""Abstract base class for AI providers."""

from abc import ABC, abstractmethod


class BaseAI(ABC):
    """All AI adapters must implement this interface."""

    @abstractmethod
    def chat(self, prompt: str) -> str:
        """Send a prompt and return the text response."""
        ...

    @abstractmethod
    def chat_json(self, prompt: str) -> str:
        """Send a prompt requesting JSON output and return text."""
        ...
