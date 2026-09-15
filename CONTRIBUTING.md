# Contributing

Use Go 1.24 or later. Clone the repository and run `go mod download`, then `make check` and `make smoke`.

## Changes

1. Open an issue for substantial API changes so the use case and tradeoffs can be discussed.
2. Keep changes focused. Put reusable behavior in `pkg/walker`; keep CLI parsing in `cmd`.
3. Add regression tests for bugs and tests of observable behavior for new features.
4. Run `gofmt` on changed Go files and `make check` before submitting a pull request.
5. Update the README, reference docs, and generated examples when CLI or library behavior changes. See [development instructions](docs/development.md).

Avoid adding mutable package-level state. Exports should remain deterministic, and errors should include enough context to identify the failing route or field.

Do not commit binaries, editor configuration, credentials, or real customer payloads.
