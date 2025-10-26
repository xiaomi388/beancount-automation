# Repository Guidelines

## Project Structure & Module Organization
The CLI entry point lives in `main.go` and wires into Cobra commands under `cmd/` (e.g. `cmd/sync`, `cmd/dump`). Reusable Go logic is grouped by feature under `pkg/` (`pkg/plaidclient`, `pkg/sync`, `pkg/link`, etc.) with fixtures in sibling `testdata/` folders. Python post-processing helpers for category merging reside in `modifier/`. Runtime artifacts and sample configs stay outside the tracked tree; keep local credentials there and copy `config.yaml.example` as a starting point before running commands.

## Build, Test, and Development Commands
- `go build ./...` compiles all Go packages; use the workspace helper script to rebuild the CLI binary when needed.
- `go run ./main.go link --owner Alice --institution "Mock Bank" --type transactions` runs the CLI without building, handy for local experiments.
- `go fmt ./...` and `go mod tidy` keep formatting and dependencies in sync; run them before submitting changes.

## Coding Style & Naming Conventions
Follow standard Go 1.17 conventions: tabs for indentation, exported identifiers in PascalCase, and package-private helpers in lowerCamelCase. Maintain the existing file layout (commands in `cmd`, feature code in `pkg`). Python utilities in `modifier/` use snake_case functions and should stay PEP 8 compliant; prefer docstrings when adding new scripts.

## Testing Guidelines
Primary coverage comes from integration tests in `pkg/link` and `pkg/sync`. Run `go test ./...` before pushing; add focused unit tests when touching core packages. Tests rely on golden files in `pkg/**/testdata`; set `UPDATE_SYNC_GOLDEN=1 go test ./pkg/sync -run Sync` to refresh expected data when behavior changes. Keep fixtures deterministic and anonymized.

## Commit & Pull Request Guidelines
Write imperative, scoped commit messages (`Add integration test harness for sync`) and use conventional prefixes only when they add clarity (`refactor:`). Each PR should describe the change, list manual test steps, and link related issues. Include screenshots or sample CLI output when modifying user-visible flows. Confirm CI status or local test runs before requesting review.

## Configuration & Security Tips
Store Plaid credentials only in local `config.yaml` copies; never commit secrets. When testing against the Plaid sandbox, ensure `plaidclient` is pointed at the sandbox environment as shown in the integration harness. Document any new environment variables or configuration flags in both the PR and `config.yaml.example`.
