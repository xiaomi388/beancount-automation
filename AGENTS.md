# Maintainer Notes

- Source layout: CLI commands under `cmd/`, supporting packages under `pkg/`, post-processing in `pkg/dump/postprocess.go`.
- Everyday workflow:
  1. Adjust `config.yaml` as needed (Plaid credentials + `postprocess` rules).
  2. Run `go test ./...` to sanity-check changes.
- Plaid Link flow is served via the local helper in `pkg/link/server.go`; browser POSTs the token back to the CLI.
- Keep `config.yaml` private; commit only `config.yaml.example` updates for new options.
- Keep `config.yaml` private; commit only `config.yaml.example` updates for new options.
