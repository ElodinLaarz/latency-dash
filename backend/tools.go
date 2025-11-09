//go:build tools
// +build tools

// Package tools tracks dependencies for tools used in the build process.
// This file is not meant to be compiled; it is only used to track tool dependencies.
package tools

import (
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)
