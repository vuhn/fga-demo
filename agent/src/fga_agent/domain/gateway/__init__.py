"""Interfaces the services need from the outside world; infrastructure implements them.

One file per collaborator (`<kind>_gateway.py`), like `api/internal/domain/gateway/<kind>_gateway.go`. A gateway is
named for what the domain asks of the collaborator, not for the technology behind it. This file only re-exports:
a new gateway is a new file plus one line here."""

from .blueprint_engine_gateway import BlueprintEngine
from .language_model_gateway import LanguageModel
from .progress_reporter_gateway import ProgressReporter

__all__ = ["BlueprintEngine", "LanguageModel", "ProgressReporter"]
