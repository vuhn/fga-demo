"""An in-memory BlueprintEngine for service tests: each method returns the next scripted answer or raises it."""

from __future__ import annotations

from fga_agent.domain.model.brief import BriefBuild, BriefDraft
from fga_agent.domain.model.design_limits import DesignLimits
from fga_agent.domain.model.operation_catalog import EditOperation, OperationCatalog
from fga_agent.domain.model.plan import PlanRevision
from fga_agent.domain.model.plan_check import PlanCheck

BRIEF = (b'{"title":"House","lot":{"width":20,"depth":30},"entrance_side":"south","open_plan":false,"rooms":['
         b'{"id":"living","name":"Living","type":"living","target_area":22,"adjacent_to":["kitchen"],"fixtures":[]},'
         b'{"id":"kitchen","name":"Kitchen","type":"kitchen","target_area":10,"adjacent_to":[],"fixtures":["sink"]},'
         b'{"id":"ensuite","name":"Ensuite","type":"bathroom","target_area":4.5,"parent":"kitchen","fixtures":[]}]}')

CATALOG = OperationCatalog(
    operations=[EditOperation("set_area", "brief_op", "set a room's target area", {}, "set_area {room, area}"),
                EditOperation("remove_room", "brief_op", "remove a room", {}, "remove_room {room}"),
                EditOperation("move_fixture", "plan_op", "move a fixture", {}, "move_fixture {id, x, y}")],
    brief_ops_schema={"type": "object"}, plan_ops_schema={"type": "object"}, draft_schema={"type": "object"})

LIMITS = DesignLimits(min_area={"bedroom": 9.0, "corridor": 1.0, "bathroom": 3.0}, min_width={"bedroom": 2.4},
                      nestable_types=["bathroom"], max_nested=2, default_fixtures={"bathroom": "toilet,basin"},
                      fixture_kinds=["sink", "toilet"], default_markups={}, rate_catalog=[], priced_fixtures=[])


def build(**over) -> BriefBuild:
    base = dict(brief=BRIEF, model=b"{}", checks=[], issues=[], ok=True, notes=["fit: ok"], attempts=1, issue_score=0,
                error_rules=[], checks_line="0 errors", total_target_area=36.5, room_targets={}, changed=True)
    return BriefBuild(**{**base, **over})


PLAN = (b'{"rooms":[{"id":"r1","name":"Kitchen","type":"kitchen","net_area":11.2},{"id":"r2","name":"Bed","type":"bedroom",'
        b'"net_area":12}],"fixtures":[{"kind":"sink","room_id":"r1"}],"openings":[{"id":"d1","kind":"door","width":0.9,'
        b'"rooms":["r1","r2"]}]}')


def check(rule="side", ok=False, severity="error", hint="set_side {room, side}") -> PlanCheck:
    return PlanCheck(rule=rule, severity=severity, ok=ok, message=f"{rule} problem", issue_id=f"{rule}:kitchen", hint=hint)


def revision(**over) -> PlanRevision:
    base = dict(model=PLAN, checks=[], ok=True, notes=[], summary="s", descriptions=["rename room Kitchen"], changed=True)
    return PlanRevision(**{**base, **over})


class ScriptedEngine:
    def __init__(self, drafts=(), builds=(), generates=(), plans=()) -> None:
        self.drafts, self.builds, self.generates, self.plans = list(drafts), list(builds), list(generates), list(plans)
        self.normalized: list[bytes] = []
        self.generated: list[tuple[bytes, bytes]] = []
        self.plan_ops: list[tuple[bytes, bytes]] = []
        self.applied: list[tuple[bytes, bytes, bytes]] = []
        self.down = False

    def _next(self, queue: list):
        item = queue.pop(0)
        if isinstance(item, Exception):
            raise item
        return item

    def operations(self, ctx):
        return self._guard(CATALOG)

    def design_limits(self, ctx):
        return self._guard(LIMITS)

    def _guard(self, value):
        if self.down:
            from fga_agent.domain.errors import EngineUnavailable
            raise EngineUnavailable("engine is down")
        return value

    def normalize_draft(self, ctx, draft: bytes) -> BriefDraft:
        self.normalized.append(draft)
        return self._next(self.drafts)

    def apply_brief_ops(self, ctx, brief: bytes, ops: bytes, previous_model: bytes = b"") -> BriefBuild:
        self.applied.append((brief, ops, previous_model))
        return self._next(self.builds)

    def generate(self, ctx, brief: bytes, previous_model: bytes = b"", repair: bool = False) -> BriefBuild:
        self.generated.append((brief, previous_model))
        return self._next(self.generates)

    def apply_plan_ops(self, ctx, model: bytes, ops: bytes) -> PlanRevision:
        self.plan_ops.append((model, ops))
        return self._next(self.plans)
