"""The Engine's limits and rates (minimum areas, fixtures, priced items), read once at start-up."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class RateSpec:
    code: str
    package: str
    description: str
    unit: str
    quality_sensitive: bool = False


@dataclass(frozen=True)
class DesignLimits:
    min_area: dict[str, float]
    min_width: dict[str, float]
    nestable_types: list[str]
    max_nested: int
    default_fixtures: dict[str, str]  # room type -> comma-separated fixture kinds
    fixture_kinds: list[str]
    default_markups: dict[str, float]
    rate_catalog: list[RateSpec]
    priced_fixtures: list[str]
