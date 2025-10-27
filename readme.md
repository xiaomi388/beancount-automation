# Beancount-Automation

Beancount-Automation fetches transactions from Plaid and produces a Beancount ledger (plus optional holdings) ready for Fava.

## Workflow

1. **Configure Plaid**
   - Copy `config.yaml.example` to `config.yaml`.
   - Fill in `clientID`, `secret`, and `environment` (`Development`, `Sandbox`, or `Production`).
   - Optionally configure post-processing (see below).

2. **Link a new account**

   ```bash
   ./bean-auto link --owner <OwnerName> --institution <InstitutionName> --type <transactions|investments>
   ```

   Owner/institution values are free-form labels used inside Beancount. Use `transactions` for checking/credit, `investments` for brokerage.

3. **Sync transactions**

   ```bash
   ./bean-auto sync
   ```

   Pulls the latest transactions (and investment data if enabled) from Plaid into `owners.yaml`.

4. **Dump to Beancount**

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

## Notes

- `config.yaml` contains Plaid credentials—keep it out of version control and share carefully.
