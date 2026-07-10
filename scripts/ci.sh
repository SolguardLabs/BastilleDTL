#!/usr/bin/env bash
set -euo pipefail

gofmt -w src cmd
go test ./...
go vet ./...
npm test
npm run loc
