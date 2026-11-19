from __future__ import annotations

from typing import Protocol

from ..model.brief import BriefBuild, BriefDraft
from ..model.call_context import CallContext
from ..model.design_limits import DesignLimits
from ..model.operation_catalog import OperationCatalog
from ..model.plan import PlanRevision
from ..model.plan_check import PlanCheckReport


class BlueprintEngine(Protocol):
    """The deterministic Engine: layout, checks, edit operations. No LLM. Every method is one Engine call.
    InvalidEdit / UnsupportedEdit when it refuses the input (the model's output included); EngineUnavailable
    when it cannot serve."""

    def operations(self, ctx: CallContext) -> OperationCatalog: ...

    def design_limits(self, ctx: CallContext) -> DesignLimits: ...

    def normalize_draft(self, ctx: CallContext, draft: bytes) -> BriefDraft: ...

    def generate(self, ctx: CallContext, brief: bytes, previous_model: bytes = b"", repair: bool = False) -> BriefBuild: ...

    def apply_brief_ops(self, ctx: CallContext, brief: bytes, ops: bytes, previous_model: bytes = b"") -> BriefBuild: ...

    def apply_plan_ops(self, ctx: CallContext, model: bytes, ops: bytes) -> PlanRevision: ...

    def run_checks(self, ctx: CallContext, model: bytes, brief: bytes = b"") -> PlanCheckReport: ...
