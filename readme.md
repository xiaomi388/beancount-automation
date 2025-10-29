# Beancount-Automation

Beancount-Automation fetches transactions from Plaid and produces a Beancount ledger (plus optional holdings) ready for Fava.

## Quick Start

1. **Download the binary**
   - Grab the archive for your operating system from the [latest release](https://github.com/xiaomi388/beancount-automation/releases/tag/latest).

2. **Review `config.yaml`**
   - The release archive already includes a `config.yaml` copied from `config.yaml.example`.
   - Update the Plaid credentials and preferred environment (`Development`, `Sandbox`, or `Production`) before running the CLI.

3. **Get your Plaid client ID & secret**
   - Sign in to the [Plaid Dashboard](https://dashboard.plaid.com/).
   - Navigate to **Team Settings → Keys** (direct link: `https://dashboard.plaid.com/team/keys`).
   - Copy the `client_id` and the environment-specific `secret` (Sandbox for testing, Development/Production for live data) into `config.yaml`.

4. **Optional:** Update the `postprocess` section in `config.yaml` to tweak transfer merging or categorisation rules (see below).

## Everyday Workflow

1. **Link a new account**

   ```bash
   ./bean-auto link --owner <OwnerName> --institution <InstitutionName> --type <transactions|investments>
   ```

   Owner/institution values are free-form labels used inside Beancount. Use `transactions` for checking/credit, `investments` for brokerage. The CLI launches Plaid Link in your browser; once you complete the flow the browser auto-sends the token back to the CLI (no manual copy/paste required). Leave the browser tab open until you see a success message.

2. **Sync transactions**

   ```bash
   ./bean-auto sync
   ```

   Pulls the latest transactions (and investment data if enabled) from Plaid into `owners.yaml`.

3. **Dump to Beancount**

   ```bash
   ./bean-auto dump
   ```

   Generates `plaid_gen.beancount`. Open it with Fava if desired: `fava ./plaid_gen.beancount`.

## Post-Processing Configuration

After Plaid data is converted, the Go pipeline applies optional merge and categorisation rules configured in `config.yaml`.

```yaml
postprocess:
  merge:
    enabled: true            # disable to skip all merge heuristics
    same_owner: true         # collapse transfers within the same owner
    cross_owner: true        # collapse transfers across owners when they match
    max_days_apart: 10       # window (days) for cross-owner matching (0 = exact same day)
  categorise:
    enabled: true            # master switch for keyword rules
    keyword_rules:           # ordered rules (first match wins)
      - match:
          description:
            contains: ["SampleMerchant", "SampleKeyword"]
        set:
          to_account:
            category: ["Example", "Category"]
      - match:
          metadata:
            id:
              equals: "E9MgQeyRmxugjevz31KysQ3Pkkp9a8HnqoPV4"
        set:
          to_account:
            category: ["Recreation", "ArtsandEntertainment"]
      # ... additional rules as needed
```
