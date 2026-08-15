$ErrorActionPreference = "Stop"
Set-Location (Resolve-Path "$PSScriptRoot\..")

$unformatted = gofmt -l src cmd
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
if ($unformatted) {
    Write-Error "Go files require formatting:`n$($unformatted -join "`n")"
}

go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go build -o bin/bastilledtl.exe ./cmd/bastilledtl
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm ci
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm run fmt:check
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm run check
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm test
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
npm run loc
exit $LASTEXITCODE
