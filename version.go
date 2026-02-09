package main

// Build-time variables, set via ldflags:
//
//	go build -ldflags "-X main.version=2.0.1 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	version = "2.0.1"
	commit  = "dev"
	date    = "unknown"
)
