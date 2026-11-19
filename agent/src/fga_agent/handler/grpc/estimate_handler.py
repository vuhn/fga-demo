"""Estimate endpoint of the Agent (EstimateTurn): decode, call one use case, encode through mapper/. A refusal before the first stream
message is a gRPC status; after it, an in-band `Error` event."""

from __future__ import annotations

from collections.abc import Iterator
from dataclasses import dataclass

from ...usecase.estimate_turner import EstimateTurner
from .common.error_helper import call_context, refuse_before_stream
from .common.streaming_helper import stream_run
from .mapper import error_mapper, estimate_event_mapper, estimate_turn_mapper


@dataclass(frozen=True)
class EstimateUsecases:
    estimate_turner: EstimateTurner


class EstimateHandler:
    def __init__(self, usecases: EstimateUsecases) -> None:
        self._uc = usecases

    def EstimateTurn(self, request, context) -> Iterator:
        ctx = call_context(context)
        return stream_run(
            context,
            lambda on_event: estimate_turn_mapper.to_done_event(
                self._uc.estimate_turner.turn(ctx, estimate_turn_mapper.to_domain(request), on_event)),
            estimate_event_mapper.to_event, error_mapper.to_estimate_error_event,
            lambda err: refuse_before_stream(context, err))
