# Contributing

Thanks for helping improve the Hello Go SDK.

## Setup

Go 1.22 or newer. The SDK uses only the standard library; please do not add dependencies.

```sh
git clone https://github.com/lakshyapocketlab/hello-go
cd hello-go
go test -race -coverprofile=cover.out .
```

## Rules

- `gofmt -l .` must print nothing and `go vet ./...` must be clean.
- Statement coverage of the package must stay at **100.0%** (CI enforces it):
  `go tool cover -func=cover.out | tail -1`.
- Tests use `net/http/httptest`; assert method, path, headers and body for every method.
- Every new API method needs a test, a README entry and a CHANGELOG line.
- Keep one test file per source file (`foo.go` -> `foo_test.go`).

## Pull requests

Open an issue first for larger changes. Keep PRs focused, and describe the API route the
change maps to. Releases are cut by maintainers by tagging `vX.Y.Z`.
