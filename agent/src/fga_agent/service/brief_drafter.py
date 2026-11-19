from __future__ import annotations

import json
import re
from collections.abc import Callable

from ..domain.errors import InvalidEdit
from ..domain.gateway import BlueprintEngine, LanguageModel
from ..domain.model.brief import BriefDraft
from ..domain.model.call_context import CallContext
from ..domain.model.progress import Progress
from ..usecase.helper.design_catalog_loader import DesignCatalogLoader
from ..usecase.helper.feedback_asker import FeedbackAsker
from .prompts import architect, examples

MAX_PROMPT = 4000
SIDES = ("north", "south", "east", "west")


class BriefDrafterService:
    def __init__(self, engine: BlueprintEngine, llm: LanguageModel, catalog: DesignCatalogLoader,
                 asker: FeedbackAsker) -> None:
        self._engine, self._llm, self._catalog, self._asker = engine, llm, catalog, asker

    def draft(self, ctx: CallContext, prompt: str, enhance: bool, on_progress: Callable[[Progress], None]) -> BriefDraft:
        if not prompt.strip():
            raise InvalidEdit("describe the house first")
        prompt = prompt[:MAX_PROMPT]
        limits, draft_schema = self._catalog.limits(ctx), self._catalog.operations(ctx).draft_schema
        user, notes = f"Description: {prompt}", []
        if enhance:  # off by default: it costs a model call and, on small models, drifts from the description
            answer = self._llm.answer_json(self._enhancer_system(limits), f"Description: {prompt}\nLet's think step by step.",
                                           examples.requirements_schema(), on_progress=on_progress)
            user += "\nRequirements from the architect:\n" + "\n".join(f"- {r}" for r in answer.get("requirements", []))
            notes += [f"assumed: {a}" for a in answer.get("assumptions", [])[:5]]
        system = ("You turn a house description and its requirements into a JSON brief.\n"
                  + architect.conventions(limits) + "\n" + examples.draft_example())
        said = {d for d in SIDES if re.search(rf"\b{d}", prompt, re.I)}

        def normalize(draft: dict) -> BriefDraft:
            return self._engine.normalize_draft(ctx, json.dumps(_keep_named_sides(draft, said)).encode())

        _, drafted, _ = self._asker.ask(ctx, system, user, draft_schema, normalize, on_progress)
        return BriefDraft(brief=drafted.brief, notes=notes + drafted.notes)

    @staticmethod
    def _enhancer_system(limits) -> str:
        return ("You are an architect. Turn a short house description into explicit requirements: every room "
                "with an approximate area, rooms inside other rooms, requested fixtures, the lot and the "
                "entrance. List as assumptions only decisions about this house that the user did not state "
                "(at most 5); do not restate general rules, and do not mention rooms you did not list.\n"
                + architect.architect_rules(limits) + "\n" + architect.conventions(limits) + "\n" + examples.enhance_examples())


def _keep_named_sides(draft: dict, said: set[str]) -> dict:
    """Small models over-assign sides; keep one only if the person named that direction."""
    rooms = [{**r, "side": None} if r.get("side") is not None and r["side"] not in said else r
             for r in draft.get("rooms", [])]
    return {**draft, "rooms": rooms}
