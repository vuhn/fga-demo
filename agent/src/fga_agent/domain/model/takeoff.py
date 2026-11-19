from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class TakeoffLine:
    code: str  # rate code
    package: str
    description: str
    quantity: float
    unit: str
    basis: str
    confidence: str  # measured | derived | assumed
    deps: list[str] = field(default_factory=list)  # fact names the quantity depends on
    room_id: str = ""


@dataclass(frozen=True)
class Takeoff:
    """What the plan needs, measured by the Engine."""

    status: str  # ok | unsupported
    lines: list[TakeoffLine] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
