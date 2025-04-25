package dump

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/xiaomi388/beancount-automation/pkg/types"
)

type Message struct {
	Owners                []types.Owner          `json:"owners"`
	BeancountTransactions []BeancountTransaction `json:"beancount_transactions"`
}

func modify(owners []types.Owner, bcTxns []BeancountTransaction) ([]BeancountTransaction, error) {
	err := filepath.Walk("./modifier", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".py" || info.Name() == "util.py" {
			return nil
		}

		tempFile, err := createTempFile(Message{owners, bcTxns})
		if err != nil {
			return err
		}
		defer os.Remove(tempFile.Name())

		if err := runModifierScript(path, tempFile.Name()); err != nil {
			return err
		}

		modifiedTxns, err := readModifiedTransactions(tempFile.Name())
		if err != nil {
			return err
		}

		bcTxns = modifiedTxns
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk file: %w", err)
	}

	return bcTxns, nil
}

func createTempFile(message Message) (*os.File, error) {
	data, err := json.Marshal(message)
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

	return f, nil
}

func runModifierScript(scriptPath, tempFilePath string) error {
	data, err := exec.Command(scriptPath, tempFilePath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run modifier scripts: %w with output:\n %s", err, data)
	}
	return nil
}

func readModifiedTransactions(filePath string) ([]BeancountTransaction, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return msg.BeancountTransactions, nil
}
