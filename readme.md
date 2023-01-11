## prerequisite

1. register a plaid account and get the clientID and secret.
2. rename `config.yaml.tpl` to `config.yaml`, and populate the clientID and secret from step 1.
3. download [fava](https://github.com/beancount/fava) and [beancount](https://github.com/beancount/beancount/)

## How to Run

### link a new account

```
go run main.go link --owner <OwnerName> --institution <InstitutionName> --type <AccountType>
```

Owner name and institution name can be arbitrary values. They are only used to identify your accounts in beancount.

For credit/debit account, type should be `transactions`. For investment account such as Schwab/Vanguard, type should be `investments`

Note: only plaid production account can be able to use `investments` type.

### sync data from plaid

Then, run this command to download all transaction data from Plaid:

```
go run main.go sync
```

### convert to beancount format

Finally, run this command to convert the plaid transactions to beancount format, and run fava:

```
go run main.go dump
fava ./plaid_gen/main.beancount
```

