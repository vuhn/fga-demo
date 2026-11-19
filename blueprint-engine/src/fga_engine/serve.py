"""Entrypoint: `python -m fga_engine.serve` runs the Engine's gRPC server on GRPC_ADDR (default :8092)."""

from __future__ import annotations

import logging
import os
import signal

from .handler.grpc.server import build_server
from .wire import build_engine


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    addr = os.environ.get("GRPC_ADDR", "0.0.0.0:8092")
    server = build_server(build_engine(), workers=int(os.environ.get("GRPC_WORKERS", "8")))
    if server.add_insecure_port(addr) == 0:
        raise SystemExit(f"cannot listen on {addr}")
    server.start()
    logging.getLogger("fga_engine.serve").info("engine gRPC listening on %s", addr)
    stop = server.wait_for_termination
    signal.signal(signal.SIGTERM, lambda *_: server.stop(5))
    try:
        stop()
    except KeyboardInterrupt:
        server.stop(5).wait()


if __name__ == "__main__":
    main()
