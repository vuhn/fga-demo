"""Engine estimate types: the facts takeoff and pricing read (value and source), takeoff lines, priced estimate and
rate types. The conversation state (slots, questions, history) is the Agent's. Units are metres; money is VND."""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field


class _Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


Source = Literal["user", "plan", "assumed"]
Quality = Literal["typical", "normal", "good", "luxury"]
WorkType = Literal["new_build", "fit_out", "renovation"]
RoomState = Literal["empty", "existing", "to_replace", "new"]
Confidence = Literal["measured", "derived", "assumed"]

FACT_NAMES: tuple[str, ...] = (
    "kind", "work_type", "city", "district", "quality", "floor_height", "storeys",
    "structure_in_scope", "mep_in_scope", "fixtures",
    "vat_percent", "overhead_percent", "profit_percent", "design_percent", "contingency_percent",
)


class Fact(_Strict):
    value: str | float | bool
    source: Source


class RoomScope(_Strict):
    state: RoomState
    source: Source


class RateSource(_Strict):
    url: str
    quote: str
    retrieved_at: str  # YYYY-MM-DD


class RateResult(_Strict):
    code: str
    low: float | None = None
    base: float | None = None
    high: float | None = None
    unit: str
    currency: str = "VND"
    confidence: Literal["ok", "low_confidence", "user", "none"] = "none"
    region: str = ""
    quality: str = ""
    sources: list[RateSource] = Field(default_factory=list)
    note: str = ""


class RateRequest(_Strict):
    code: str
    description: str
    unit: str
    region: str
    quality: str


class TakeoffLine(_Strict):
    code: str  # rate code
    package: str
    description: str
    quantity: float
    unit: str
    basis: str
    confidence: Confidence
    deps: list[str] = Field(default_factory=list)  # fact names the quantity depends on
    room_id: str | None = None


class Takeoff(_Strict):
    status: Literal["ok", "unsupported"] = "ok"
    lines: list[TakeoffLine] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)

    def codes(self) -> list[str]:
        seen: list[str] = []
        for ln in self.lines:
            if ln.code not in seen:
                seen.append(ln.code)
        return seen


class PricedLine(_Strict):
    code: str
    package: str
    description: str
    quantity: float
    unit: str
    basis: str
    confidence: Confidence
    rate_low: float | None = None
    rate_base: float | None = None
    rate_high: float | None = None
    low: float = 0
    base: float = 0
    high: float = 0
    priced: bool = True
    rate_confidence: str = "none"


class Markup(_Strict):
    name: str
    percent: float
    base_label: str
    low: float
    base: float
    high: float


class Estimate(_Strict):
    currency: str = "VND"
    lines: list[PricedLine] = Field(default_factory=list)
    packages: dict[str, dict[str, float]] = Field(default_factory=dict)  # package -> low/base/high
    direct: dict[str, float] = Field(default_factory=dict)
    markups: list[Markup] = Field(default_factory=list)
    total: dict[str, float] = Field(default_factory=dict)  # low / base / high, VAT included
    assumed_share: float = 0.0
    assumptions: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)
    rates: list[RateResult] = Field(default_factory=list)  # every rate used, with its sources


class EstimateFacts(_Strict):
    """Everything takeoff and pricing read from an interview. The caller owns the conversation."""

    facts: dict[str, Fact] = Field(default_factory=dict)
    scope: dict[str, RoomScope] = Field(default_factory=dict)
    # plan fixture ids the person says are already installed; every other priced fixture is assumed NOT there
    fixtures_existing: list[str] = Field(default_factory=list)
    # rate codes the person took out of the estimate ("the doors already exist"); the takeoff leaves them out
    excluded: list[str] = Field(default_factory=list)
