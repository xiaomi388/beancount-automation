package dump

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xiaomi388/beancount-automation/pkg/transaction"
)

func modify(txns []transaction.Transaction, bcTxns []BeancountTransaction) ([]BeancountTransaction, error) {
	type Message struct {
		Transactions          []transaction.Transaction `json:"transactions"`
		BeancountTransactions []BeancountTransaction    `json:"beancount_transactions"`
	}
	data, err := json.Marshal(Message{txns, bcTxns})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transactions: %w", err)
	}

	f, err := os.CreateTemp("", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	if err := os.WriteFile(f.Name(), data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	if err := filepath.Walk("./modifier", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".py" || info.Name() == "util.py" {
			return nil
		}

		data, err = exec.Command(path, f.Name()).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run modifier scripts: %w with output:\n %s", err, data)
		}

        return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk file: %w", err)
	}

	var msg Message
	data, err = os.ReadFile(f.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	mBCTxns := msg.BeancountTransactions
	return mBCTxns, nil
}
