#!/bin/bash

# Ensure the output directory exists
mkdir -p proto

# Use the full path to protoc and specify plugin paths
/home/elodin/anaconda3/bin/protoc \
    --plugin=protoc-gen-go=$GOPATH/bin/protoc-gen-go \
    --plugin=protoc-gen-go-grpc=$GOPATH/bin/protoc-gen-go-grpc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/messages.proto
