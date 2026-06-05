"""Material parsing package."""

from .local_parse import parse_local_file
from .url_parse import parse_url

__all__ = ["parse_local_file", "parse_url"]
