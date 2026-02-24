package migrate

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/persistence"
)

var (
	fromBackend string
	toBackend   string
	sourcePath  string
	destPath    string
)

var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate owner data between storage backends",
	Long:  `Migrate owner data from one storage backend to another (e.g. json to sqlite).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrate()
	},
}

func init() {
	MigrateCmd.Flags().StringVar(&fromBackend, "from", "json", "source backend (json or sqlite)")
	MigrateCmd.Flags().StringVar(&toBackend, "to", "sqlite", "destination backend (json or sqlite)")
	MigrateCmd.Flags().StringVar(&sourcePath, "source", "", "source file path (defaults based on backend)")
	MigrateCmd.Flags().StringVar(&destPath, "dest", "", "destination file path (defaults based on backend)")
}

func runMigrate() error {
	if fromBackend == toBackend {
		return fmt.Errorf("source and destination backends are the same: %s", fromBackend)
	}

	src, err := persistence.NewStoreWithBackend(fromBackend, sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source store: %w", err)
	}
	defer src.Close()

	dst, err := persistence.NewStoreWithBackend(toBackend, destPath)
	if err != nil {
		return fmt.Errorf("failed to open destination store: %w", err)
	}
	defer dst.Close()

	owners, err := src.LoadOwners()
	if err != nil {
		return fmt.Errorf("failed to load from source: %w", err)
	}

	if err := dst.DumpOwners(owners); err != nil {
		return fmt.Errorf("failed to write to destination: %w", err)
	}

	fmt.Printf("Successfully migrated %d owner(s) from %s to %s.\n", len(owners), fromBackend, toBackend)
	fmt.Println("Update your config.yaml to use the new backend:")
	fmt.Println("  storage:")
	fmt.Printf("    backend: %s\n", toBackend)
	return nil
}
