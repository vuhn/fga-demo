"""Handler helper: plans, briefs and ops cross the wire as JSON bytes (the Engine's schemas are the contract)."""

from __future__ import annotations

from typing import TypeVar

from pydantic import BaseModel

M = TypeVar("M", bound=BaseModel)


def load_json(model: type[M], data: bytes, what: str) -> M:
    if not data:
        raise ValueError(f"{what} is empty")
    return model.model_validate_json(data)


def load_optional_json(model: type[M], data: bytes, what: str) -> M | None:
    return load_json(model, data, what) if data else None


def dump_json(obj: BaseModel) -> bytes:
    return obj.model_dump_json(exclude_none=False).encode()
