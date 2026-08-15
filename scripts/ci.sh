#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

unformatted="$(gofmt -l src cmd)"
if [[ -n "$unformatted" ]]; then
  echo "Go files require formatting:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go test ./...
go vet ./...
go build -o bin/bastilledtl ./cmd/bastilledtl
npm ci
npm run fmt:check
npm run check
npm test
npm run loc
