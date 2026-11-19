"""What every call carries besides its arguments: the opaque request id (`x-request-id`), like Go's context.
Passed explicitly as the first argument of use cases and gateways; no tenant, person or actor ever appears."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class CallContext:
    request_id: str = ""
