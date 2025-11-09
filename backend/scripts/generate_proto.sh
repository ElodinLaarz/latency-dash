#!/bin/bash

set -e

# Ensure the output directory exists
mkdir -p proto

# Generate Go code from the proto file
protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/messages.proto

echo "Protocol Buffers generated successfully!"
