"""Engine DTOs <-> domain for plans: the ApplyPlanOps request out, the revision in."""

from __future__ import annotations

from ....domain.model.plan import PlanRevision
from fga.demo.engine.v1 import engine_pb2 as e
from . import plan_check_mapper


def apply_ops_request(model: bytes, ops: bytes) -> e.ApplyPlanOpsRequest:
    return e.ApplyPlanOpsRequest(model_json=model, ops_json=ops)


def to_domain(r: e.PlanStep) -> PlanRevision:
    return PlanRevision(model=r.model_json, checks=[plan_check_mapper.to_domain(x) for x in r.checks], ok=r.ok,
                        notes=list(r.notes), summary=r.summary, descriptions=list(r.descriptions), changed=r.changed)
