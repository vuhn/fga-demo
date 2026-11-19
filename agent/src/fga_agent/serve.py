"""Entrypoint: `python -m fga_agent.serve` runs the Agent's gRPC server on GRPC_ADDR (default :8091)."""

from __future__ import annotations

import logging
import os
import signal

from .handler.grpc.server import build_server
from .wire import build_agent


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    addr = os.environ.get("GRPC_ADDR", "0.0.0.0:8091")
    built = build_server(build_agent(), workers=int(os.environ.get("GRPC_WORKERS", "8")))
    if built.server.add_insecure_port(addr) == 0:
        raise SystemExit(f"cannot listen on {addr}")
    built.server.start()
    built.watch_catalog()
    logging.getLogger("fga_agent.serve").info("agent gRPC listening on %s (engine %s)", addr,
                                              os.environ.get("ENGINE_ADDR", "localhost:8092"))
    signal.signal(signal.SIGTERM, lambda *_: built.stop())
    try:
        built.server.wait_for_termination()
    except KeyboardInterrupt:
        built.stop()


if __name__ == "__main__":
    main()
