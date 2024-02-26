package dump

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xiaomi388/beancount-automation/pkg/types"
)

func modify(owners []types.Owner, bcTxns []BeancountTransaction) ([]BeancountTransaction, error) {
	type Message struct {
		Owners                []types.Owner          `json:"owners"`
		BeancountTransactions []BeancountTransaction `json:"beancount_transactions"`
	}
	if err := filepath.Walk("./modifier", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		data, err := json.Marshal(Message{owners, bcTxns})
		if err != nil {
			return fmt.Errorf("failed to marshal transactions: %w", err)
		}

		f, err := os.CreateTemp("", "")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}

		if err := os.WriteFile(f.Name(), data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

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

		var msg Message
		data, err = os.ReadFile(f.Name())
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal json: %w", err)
		}

		bcTxns = msg.BeancountTransactions
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk file: %w", err)
	}

	return bcTxns, nil
}
