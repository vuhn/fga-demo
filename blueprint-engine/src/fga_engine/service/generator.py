from __future__ import annotations

from ..domain.results import BriefResult, StepResult, brief_result
from ..domain.schemas import Brief, BuildingModel
from ..pipeline import generate, generate_fitted


def fitted_step(brief: Brief, previous: BuildingModel | None = None, notes: list[str] | None = None) -> StepResult:
    """Layout with the deterministic repair (rooms shrink toward their minimums); shared with the LLM flows."""
    result, fit_notes, runs = generate_fitted(brief, previous)
    return StepResult(result=result, notes=list(notes or []) + fit_notes, attempts=runs)


class GeneratorService:
    def generate(self, brief: Brief, previous: BuildingModel | None = None, *, repair: bool = False) -> StepResult:
        if repair:
            return fitted_step(brief, previous)
        return StepResult(result=generate(brief, previous))

    def generate_brief(self, brief: Brief, previous: BuildingModel | None = None, *, repair: bool = False) -> BriefResult:
        step = self.generate(brief, previous, repair=repair)
        return brief_result(step.result, notes=step.notes, attempts=step.attempts)
