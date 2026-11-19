"""Brief endpoints (NormalizeDraft, Generate, ApplyBriefOps). Decode, call one use case, encode through mapper/."""

from __future__ import annotations

from dataclasses import dataclass

from ...usecase.brief_ops_applier import BriefOpsApplier
from ...usecase.draft_normalizer import DraftNormalizer
from ...usecase.generator import Generator
from .common.error_helper import handled
from .mapper import brief_mapper


@dataclass(frozen=True)
class BriefUsecases:
    draft_normalizer: DraftNormalizer
    generator: Generator
    brief_ops_applier: BriefOpsApplier


class BriefHandler:
    def __init__(self, usecases: BriefUsecases) -> None:
        self._uc = usecases

    @handled
    def NormalizeDraft(self, request, context):
        brief, notes = self._uc.draft_normalizer.normalize_draft(brief_mapper.normalize_to_domain(request))
        return brief_mapper.normalized_to_pb(brief, notes)

    @handled
    def Generate(self, request, context):
        brief, previous, repair = brief_mapper.generate_to_domain(request)
        return brief_mapper.result_to_pb(self._uc.generator.generate_brief(brief, previous, repair=repair))

    @handled
    def ApplyBriefOps(self, request, context):
        brief, ops, previous = brief_mapper.apply_ops_to_domain(request)
        return brief_mapper.result_to_pb(self._uc.brief_ops_applier.apply_brief_ops(brief, ops, previous))
