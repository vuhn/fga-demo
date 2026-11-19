"""Delivery layer of the Agent (like the Go API's handler/http): `<domain>_handler.py` endpoint classes and the plumbing
around them (servicer, server and health, errors, mapper/, and common/ for the helpers the handlers share, such as streaming). It decodes a request, calls one use case and
encodes the answer; business logic lives in services."""
