"""Errors the Agent's services raise; handlers map them to gRPC statuses."""

from __future__ import annotations


class EditError(ValueError):
    """The input or the model's output is wrong in a way a retry with this feedback can fix."""

    code = "edit_error"


class UnsupportedEdit(EditError):
    """A valid request the Engine does not support; retrying cannot help."""

    code = "unsupported"


class EngineUnavailable(RuntimeError):
    """The Engine cannot serve (down, deadline)."""


class LLMError(RuntimeError):
    """The language model cannot serve (Ollama down, invalid output twice)."""
