"""Engine `PlanStep` -> domain."""

from __future__ import annotations

from ....domain.model.plan import PlanRevision
from ....gen.fga.demo.engine.v1 import engine_pb2 as e
from .plan_check_mapper import plan_check


def plan_revision(r: e.PlanStep) -> PlanRevision:
    return PlanRevision(model=r.model_json, checks=[plan_check(x) for x in r.checks], ok=r.ok, notes=list(r.notes),
                        summary=r.summary, descriptions=list(r.descriptions), changed=r.changed)
