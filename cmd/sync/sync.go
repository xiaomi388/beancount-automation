/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package sync

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/sync"
)

// syncCmd represents the sync command
var SyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "sync transactions from plaid",
	Run: func(cmd *cobra.Command, args []string) {
		if err := sync.Sync(); err != nil {
			fmt.Println(err)
		}
	},
}
