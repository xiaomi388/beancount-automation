package transaction

import (
    "os"
	"encoding/json"
	"fmt"

	"github.com/plaid/plaid-go/plaid"
)

// TODO: make db path configurable.
const DBPath = "./transaction.db"

type Transaction struct {
    Transaction plaid.Transaction `json:"transaction"`
    Account plaid.AccountBase `json:"account"`
    Owner string `json:"owner"`
    Institution string `json:"institution"`
}


func New(txn plaid.Transaction, owner string, institution string, account plaid.AccountBase) Transaction {
    return Transaction{
        Transaction: txn,
        Account: account,
        Owner: owner,
        Institution: institution,
    }
}



func Load(dbPath string) (map[string]Transaction, error) {
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        return map[string]Transaction{}, nil
    }

    data, err := os.ReadFile(dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read txn db file: %w", err)
    }

    txns := map[string]Transaction{}
    if err := json.Unmarshal(data, &txns); err != nil {
        return nil, fmt.Errorf("failed to unmarshal data: %w", err)
    }

    return txns, nil
}

func Dump(dbPath string, txns map[string]Transaction) error {
    data, err := json.Marshal(txns)
    if err != nil {
        return fmt.Errorf("failed to marshal txns: %w", err)
    }

    if err := os.WriteFile(dbPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }

    return nil
}
