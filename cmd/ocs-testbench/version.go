package main

// Version is injected at build time via:
//
//	go build -ldflags "-X main.Version=v1.2.3"
//
// The Makefile targets set this from `git describe --tags --always`.
// Falls back to "dev" for plain `go build` or `go run`.
var Version = "dev"
