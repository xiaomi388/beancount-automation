package holding

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/plaid/plaid-go/plaid"
)

type Holding struct {
	Holding     plaid.Holding     `json:"holding"`
	Security    plaid.Security    `json:"security"`
	Account     plaid.AccountBase `json:"account"`
	Owner       string            `json:"owner"`
	Institution string            `json:"institution"`
}

func New(holding plaid.Holding, security plaid.Security, owner string, institution string, account plaid.AccountBase) Holding {
	return Holding{
		Holding:     holding,
		Security:    security,
		Account:     account,
		Owner:       owner,
		Institution: institution,
	}
}

func Load(dbPath string) ([]Holding, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return []Holding{}, nil
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read holding db file: %w", err)
	}

	holdings := []Holding{}
	if err := json.Unmarshal(data, &holdings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return holdings, nil
}

func Dump(dbPath string, holdings []Holding) error {
	data, err := json.Marshal(holdings)
	if err != nil {
		return fmt.Errorf("failed to marshal holdings: %w", err)
	}

	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
