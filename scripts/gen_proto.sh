#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO="${ROOT}/proto/strategy.proto"

# Go stubs
pushd "${ROOT}/backend/cmd/trading-core" >/dev/null
protoc --go_out=. --go-grpc_out=. "${PROTO}"
popd >/dev/null

# Python stubs
pushd "${ROOT}/python/worker" >/dev/null
python -m grpc_tools.protoc -I"${ROOT}" --python_out=. --grpc_python_out=. "${PROTO}"
popd >/dev/null

echo "✓ Proto generated for Go and Python"

