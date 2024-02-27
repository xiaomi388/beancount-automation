# Beancount-Automation

Beancount-Automation auto generates a beancount file by getting the transaction records from Plaid.

## How to Run

### Installation

Get the latest binary from [here](https://github.com/xiaomi388/beancount-automation/releases/tag/latest).

### Prerequisite

1. register a plaid account and get the clientID and secret.
2. rename `config.yaml.tpl` to `config.yaml`, and populate the clientID and secret from step 1.
3. download [fava](https://github.com/beancount/fava) and [beancount](https://github.com/beancount/beancount/)


### Link a New Account

```
./bean-auto link --owner <OwnerName> --institution <InstitutionName> --type <AccountType>
```

Owner name and institution name can be arbitrary values. They are only used to identify your accounts in beancount.

For credit/debit account, type should be `transactions`. For investment account such as Schwab/Vanguard, type should be `investments`

Note: only plaid production account can be able to use `investments` type.

### Sync Data from Plaid

Then, run this command to download all transaction data from Plaid:

```
./bean-auto sync
```

### Convert to Beancount Format

Finally, run this command to convert the plaid transactions to beancount format, and run fava:

```
./bean-auto dump
fava ./plaid_gen.beancount
```

