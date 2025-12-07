$ErrorActionPreference = "Stop"

$root = Resolve-Path "$PSScriptRoot\.."
$proto = Join-Path $root "proto\strategy.proto"

# Go stubs (source_relative keeps outputs beside proto files)
Push-Location "$root\backend\cmd\trading-core"
protoc -I"$root" --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative --go_out=. --go-grpc_out=. "$proto"
Pop-Location

# Python stubs
Push-Location "$root\python\worker"
python -m grpc_tools.protoc -I"$root" --python_out=. --grpc_python_out=. "$proto"
Pop-Location

Write-Host "Proto generated for Go and Python"
