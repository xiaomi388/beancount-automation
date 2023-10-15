/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package dump

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/dump"
)

// DumpCmd represents the dump command
var DumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "generate beancount file",
	Run: func(cmd *cobra.Command, args []string) {
		if err := dump.Dump(); err != nil {
			fmt.Println(err)
		}
	},
}
