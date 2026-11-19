"""EngineService: routes each RPC to the handler of its domain (like the Go router registers each handler's routes).
Business logic is in the services, conversion in mapper/, status mapping in common/error_helper."""

from __future__ import annotations

from fga.demo.engine.v1 import engine_pb2_grpc as rpc
from .brief_handler import BriefHandler
from .catalog_handler import CatalogHandler
from .estimate_handler import EstimateHandler
from .file_handler import FileHandler
from .plan_handler import PlanHandler


class EngineServicer(rpc.EngineServiceServicer):
    def __init__(self, catalog: CatalogHandler, brief: BriefHandler, plan: PlanHandler, estimate: EstimateHandler,
                 files: FileHandler) -> None:
        self._catalog, self._brief, self._plan, self._estimate, self._files = catalog, brief, plan, estimate, files

    def ListTools(self, request, context):
        return self._catalog.ListTools(request, context)

    def GetCatalog(self, request, context):
        return self._catalog.GetCatalog(request, context)

    def NormalizeDraft(self, request, context):
        return self._brief.NormalizeDraft(request, context)

    def Generate(self, request, context):
        return self._brief.Generate(request, context)

    def ApplyBriefOps(self, request, context):
        return self._brief.ApplyBriefOps(request, context)

    def CreatePlan(self, request, context):
        return self._plan.CreatePlan(request, context)

    def DrawPlan(self, request, context):
        return self._plan.DrawPlan(request, context)

    def ApplyPlanOps(self, request, context):
        return self._plan.ApplyPlanOps(request, context)

    def RunChecks(self, request, context):
        return self._plan.RunChecks(request, context)

    def SummarizePlan(self, request, context):
        return self._plan.SummarizePlan(request, context)

    def Takeoff(self, request, context):
        return self._estimate.Takeoff(request, context)

    def PriceEstimate(self, request, context):
        return self._estimate.PriceEstimate(request, context)

    def ConvertToDXF(self, request, context):
        return self._files.ConvertToDXF(request, context)

    def ImportDXF(self, request, context):
        return self._files.ImportDXF(request, context)

    def PatchDXF(self, request, context):
        return self._files.PatchDXF(request, context)

    def Render(self, request, context):
        return self._files.Render(request, context)
