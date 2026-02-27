# CLAUDE.md — AI Assistant Guide for beancount-automation

## Project Overview

**beancount-automation** is a Go CLI tool that fetches financial transactions and investment data from [Plaid](https://plaid.com/) and generates a [Beancount](https://beancount.github.io/) ledger file (`plaid_gen.beancount`) suitable for viewing in [Fava](https://fava.pythonanywhere.com/).

The compiled binary is named `bean-auto`.

---

## Repository Layout

```
.
├── main.go                         # Entry point — calls cmd.Execute()
├── config.yaml.example             # Template config (commit only this, never config.yaml)
├── go.mod / go.sum                 # Go module files
├── cmd/                            # Cobra CLI subcommands (thin wrappers)
│   ├── root.go                     # Root command, registers all subcommands
│   ├── vars.go                     # Shared CLI flag variables
│   ├── link/link.go                # `bean-auto link` — link a new institution
│   ├── relink/relink.go            # `bean-auto relink` — refresh an expired token
│   ├── sync/sync.go                # `bean-auto sync` — pull transactions from Plaid
│   ├── dump/dump.go                # `bean-auto dump` — write plaid_gen.beancount
│   └── migrate/migrate.go          # `bean-auto migrate` — migrate between storage backends
└── pkg/                            # Business logic packages
    ├── types/types.go              # All shared data types (Config, Owner, Institution, …)
    ├── plaidclient/client.go       # Plaid API client factory + environment registry
    ├── link/
    │   ├── link.go                 # Link/relink flow (creates access tokens)
    │   ├── relink.go               # Re-link expired institution tokens
    │   ├── server.go               # Local HTTP server that captures the Plaid public token
    │   ├── util.go                 # Link helper utilities
    │   ├── link.html.tpl           # Template source for the Plaid Link page
    │   └── static/link.html        # Embedded HTML served during the Link flow
    ├── sync/
    │   ├── sync.go                 # Transaction & investment sync logic
    │   ├── sync_integration_test.go # Integration test with mock Plaid server
    │   └── testdata/               # Fixtures: config, owners, mock API request/response JSON
    ├── dump/
    │   ├── dump.go                 # Converts owner data → BeancountTransaction structs → file
    │   ├── postprocess.go          # Merge & categorise post-processing pipeline
    │   └── tpl.go                  # Beancount output templates (transaction, open-account, holding)
    └── persistence/
        ├── store.go                # Store interface + factory (NewStore / NewStoreWithBackend)
        ├── consts.go               # Default file paths
        ├── json_store.go           # JSONStore — stores owners in owners.yaml (JSON encoding)
        ├── yaml.go                 # Low-level YAML load/dump helpers
        └── sqlite_store.go         # SQLiteStore — normalised relational schema in owners.db
```

---

## Key Data Flow

```
config.yaml
    │
    ▼
bean-auto link          → Plaid Link browser flow → access token saved to store
    │
    ▼
bean-auto sync          → Plaid API (TransactionsSync / InvestmentsHoldingsGet /
    │                      InvestmentsTransactionsGet) → updates store
    ▼
bean-auto dump          → loads store → converts to BeancountTransaction →
                          post-processing (merge + categorise) →
                          writes plaid_gen.beancount
```

---

## CLI Commands

| Command | Flags | Description |
|---------|-------|-------------|
| `bean-auto link` | `--owner`, `--institution`, `--type` (transactions\|investments) | Link a new bank/brokerage to Plaid. Opens a browser for the Plaid Link UI; the public token is POSTed back automatically to the local server. |
| `bean-auto relink` | `--owner`, `--institution`, `--type` (required) | Re-authenticate an institution whose token has expired. |
| `bean-auto sync` | — | Pull the latest transactions and investment data from Plaid and persist to the store. Uses cursor-based pagination for transactions. |
| `bean-auto dump` | — | Read all owner data from the store and write `./plaid_gen.beancount`. |
| `bean-auto migrate` | `--from`, `--to`, `--source`, `--dest` | Migrate owner data between storage backends (json ↔ sqlite). |

All commands read `./config.yaml` and use the storage backend configured there.

---

## Configuration (`config.yaml`)

```yaml
clientID: ""          # Plaid client ID
secret: ""            # Plaid secret (environment-specific)
environment: "Development"   # Development | Sandbox | Production

storage:
  backend: json        # "json" (owners.yaml) or "sqlite" (owners.db)
  # path: ./owners.yaml  # optional override

postprocess:
  merge:
    enabled: true
    same_owner: true        # collapse intra-owner transfers into one entry
    cross_owner: true       # collapse inter-owner transfers (by amount + currency + date window)
    max_days_apart: 10      # days window for cross-owner matching
  categorise:
    enabled: false
    keyword_rules:          # ordered; first match wins
      - match:
          description:
            contains: ["Starbucks"]
        set:
          to_account:
            category: ["Food", "Coffee"]
```

**Important:** `config.yaml` contains secrets — never commit it. Only update `config.yaml.example` when adding new options.

---

## Storage Backends

### JSON (default)
- Data stored in `./owners.yaml` (JSON-encoded despite the `.yaml` extension).
- Simple; suitable for small datasets.
- `JSONStore` is a thin wrapper around `LoadOwners`/`DumpOwners` in `pkg/persistence/yaml.go`.

### SQLite
- Data stored in `./owners.db`.
- Schema: `owners → institutions → accounts → transactions / holdings / securities / investment_transactions`.
- Uses WAL mode and foreign-key enforcement.
- Type aliases (e.g. `rawAccountBase`) bypass plaid-go's custom `UnmarshalJSON` which silently drops unknown enum values.
- Switch backends with `bean-auto migrate --from json --to sqlite`, then update `config.yaml`.

---

## Core Types (`pkg/types/types.go`)

| Type | Purpose |
|------|---------|
| `Config` | Top-level config struct (YAML) |
| `StorageConfig` | Backend selector + optional path |
| `PostprocessConfig` | Merge + categorise pipeline config |
| `Owner` | A person; holds `[]TransactionInstitution` and `[]InvestmentInstitution` |
| `InstitutionBase` | Shared fields: name, Plaid access token, cursor |
| `TransactionInstitution` | Institution + `[]TransactionAccount` |
| `InvestmentInstitution` | Institution + `[]InvestmentAccount` |
| `TransactionAccount` | `plaid.AccountBase` + `map[transactionID]plaid.Transaction` |
| `InvestmentAccount` | `plaid.AccountBase` + holdings/securities/transactions maps |

All `CreateOrUpdate*` methods follow an upsert pattern (match by name/ID; append if not found). Types are value receivers returning modified copies — mutation is explicit.

---

## Beancount Output Format

The `dump` package converts Plaid data to `BeancountTransaction` structs and renders them via Go `text/template`.

**Account naming scheme:**

- Balance accounts: `{Assets|Liabilities}:{Owner}:{Currency}:{Institution}:{PlaidAccountType}:{AccountName}`
- Expense/Income accounts: `{Expenses|Income}:{Currency}:{Category...}`

Special characters (non-alphanumeric) are stripped from all name components via regex.

**Post-processing pipeline** (`pkg/dump/postprocess.go`):
1. **Merge** — detects transfer pairs (same amount + currency; `Assets`→`Assets`/`Liabilities`) and collapses them:
   - Same-owner transfers: both legs replaced by a single entry.
   - Cross-owner transfers: matched within `max_days_apart` days.
2. **Categorise** — applies ordered `keyword_rules` to override `ToAccount`/`FromAccount` categories or add tags.

---

## Plaid Link Flow (`pkg/link/`)

1. CLI requests a Plaid link token via the API.
2. A local HTTP server starts on a random port (`pkg/link/server.go`).
3. The browser is opened to `http://127.0.0.1:<port>/` which serves the embedded `static/link.html`.
4. The user completes Plaid Link in the browser; the page POSTs the public token back to `/token`.
5. The CLI exchanges the public token for a permanent access token via `PlaidApi.ItemPublicTokenExchange`.
6. The access token is saved to the store.

The server handles three routes:
- `GET /` — serves the Plaid Link HTML page
- `GET /link-token` — returns the link token as JSON (consumed by the page's JS)
- `POST /token` — receives the public token from the browser

---

## Development Workflow

### Prerequisites
- Go 1.24+
- A valid `config.yaml` with Plaid credentials

### Build
```bash
go build -o bean-auto .
```

### Test
```bash
go test ./...
```

Integration tests in `pkg/sync/` spin up an `httptest.Server` that mocks the Plaid API using JSON fixtures in `pkg/sync/testdata/`. No real Plaid credentials are needed.

To update golden files after intentional output changes:
```bash
UPDATE_SYNC_GOLDEN=1 go test ./pkg/sync/...
```

### CI (GitHub Actions — `.github/workflows/go.yml`)
- Triggers on push/PR to `master`.
- Cross-compiles for linux/386, linux/amd64, linux/arm64, windows/386, windows/amd64, darwin/amd64, darwin/arm64.
- Publishes binaries to the `latest` GitHub release, bundled with `config.yaml`.

---

## Feature Development Flow

**Never push directly to `master`.** All changes must go through a feature branch and pull request.

### Steps for every change

1. **Create a branch** off `master` with a short descriptive name:
   ```bash
   git checkout master
   git pull origin master
   git checkout -b feat/<short-description>
   ```
   Use prefixes to signal intent:
   | Prefix | Use for |
   |--------|---------|
   | `feat/` | new functionality |
   | `fix/` | bug fixes |
   | `refactor/` | code restructuring without behaviour change |
   | `docs/` | documentation-only changes |
   | `chore/` | tooling, CI, dependency updates |

2. **Develop and commit** with clear, descriptive messages:
   ```bash
   git add <files>
   git commit -m "feat: short summary of what and why"
   ```

3. **Run tests** before pushing:
   ```bash
   go test ./...
   ```

4. **Push the branch** and open a PR against `master`:
   ```bash
   git push -u origin feat/<short-description>
   gh pr create --base master --title "feat: ..." --body "..."
   ```

5. **Merge via PR** only — squash or merge commit, never force-push to `master`.

### Rules
- `master` must always be in a releasable state.
- PRs require passing CI (the `release` workflow builds all targets).
- Delete the feature branch after the PR is merged.

---

## Code Conventions

- **Error handling:** always wrap errors with `fmt.Errorf("context: %w", err)`. Propagate errors up; only `os.Exit(1)` at the `cmd/` layer.
- **No global state in packages:** configuration and stores are passed as parameters, not global variables. The exception is `plaidclient.environments` (used for test overrides only).
- **Store lifecycle:** always `defer store.Close()` immediately after `NewStore(...)`.
- **Upsert pattern:** `CreateOrUpdate*` helpers take value receivers and return modified copies; the caller must reassign.
- **Config privacy:** `config.yaml` is gitignored. Only update `config.yaml.example` for new options.
- **Beancount name sanitisation:** strip all non-alphanumeric characters using `regexp.MustCompile(`[^a-zA-Z0-9]`)` before embedding strings in account names or descriptions.
- **Template rendering:** all Beancount output uses `text/template` defined in `pkg/dump/tpl.go`.

---

## Known Issues / Notes

- `dumpHoldings()` in `pkg/dump/dump.go` is intentionally commented out (holdings output is incomplete).
- `strings.Title` is used in the dump package but is deprecated in newer Go versions (no functional issue currently).
- The `AccoutBase` field name is a typo (`Accout` not `Account`) that is preserved across the codebase for backwards compatibility with existing JSON data.
- Investment transaction syncing fetches a fixed date range (`2020-01-01` to `2999-01-01`) — cursor-based pagination is not yet used for investments.
- The `cmd/vars.go` shared flag variables (`owner`, `institution`, `accountType`) are currently unused; each subcommand defines its own local flags.
