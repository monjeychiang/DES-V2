"""
Python gRPC worker that serves StrategyService.

It auto-generates strategy_pb2*.py on startup if missing (requires grpcio-tools).
"""
import logging
import subprocess
import sys
from concurrent import futures
from pathlib import Path

import grpc

# Ensure parent directory (which contains `strategies/`) is importable
HERE = Path(__file__).resolve().parent
PROJECT_ROOT = HERE.parent
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from strategies.example_grid import ExampleGrid


def ensure_proto_generated():
    """Generate gRPC stubs if they do not exist."""
    here = Path(__file__).resolve().parent
    out_dir = here / "proto"
    out_dir.mkdir(exist_ok=True)
    init_file = out_dir / "__init__.py"
    if not init_file.exists():
        init_file.write_text("", encoding="utf-8")
    pb = out_dir / "strategy_pb2.py"
    pb_grpc = out_dir / "strategy_pb2_grpc.py"
    if pb.exists() and pb_grpc.exists():
        return

    proto_root = (here / "../..").resolve()
    proto_file = proto_root / "proto" / "strategy.proto"
    cmd = [
        sys.executable,
        "-m",
        "grpc_tools.protoc",
        f"-I{proto_root}",
        f"--python_out={out_dir}",
        f"--grpc_python_out={out_dir}",
        str(proto_file),
    ]
    logging.info("Generating protobuf stubs: %s", " ".join(cmd))
    subprocess.check_call(cmd, cwd=here)


def import_stubs():
    try:
        here = Path(__file__).resolve().parent
        # Ensure local 'proto' package shadows any installed 'proto'
        if str(here) not in sys.path:
            sys.path.insert(0, str(here))

        from proto import strategy_pb2  # type: ignore
        from proto import strategy_pb2_grpc  # type: ignore
    except Exception as exc:  # broaden to see real error
        logging.exception("Failed to import generated stubs")
        raise SystemExit(
            f"Failed to import generated stubs: {exc!r}"
        ) from exc
    return strategy_pb2, strategy_pb2_grpc


class StrategyService:
    def __init__(self, pb):
        self.pb = pb
        self.strategy = ExampleGrid(symbol="BTCUSDT", lower=100.0, upper=200.0, size=0.001)

    def OnTick(self, request, context):
        decision = self.strategy.on_tick(request.symbol, request.price, dict(request.indicators))
        if not decision:
            return self.pb.Signal(action="HOLD", symbol=request.symbol, size=0, note="no-op")
        return self.pb.Signal(
            action=decision["action"],
            symbol=decision["symbol"],
            size=decision["size"],
            note=decision.get("note", ""),
        )


def serve(port: int = 50051):
    ensure_proto_generated()
    pb, pb_grpc = import_stubs()

    class Servicer(StrategyService, pb_grpc.StrategyServiceServicer):  # type: ignore
        pass

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    pb_grpc.add_StrategyServiceServicer_to_server(Servicer(pb), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    logging.info("Python worker started on %d", port)
    server.wait_for_termination()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    serve()
