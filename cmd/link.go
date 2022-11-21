/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"os"

	"github.com/plaid/plaid-go/plaid"
	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/link"
)

var (
	owner       *string
	institution *string
	accountType *string
)

// linkCmd represents the link command
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "link an institution",
	Run: func(_ *cobra.Command, _ []string) {
        if *accountType != string(plaid.PRODUCTS_INVESTMENTS) && *accountType != string(plaid.PRODUCTS_TRANSACTIONS) {
            fmt.Println("account type should be either investments or transactions")
            os.Exit(1)
        }

		err := link.Link(*owner, *institution, plaid.Products(*accountType))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)

	// Here you will define your flags and configuration settings.
	owner = linkCmd.PersistentFlags().String("owner", "", "")
	linkCmd.MarkPersistentFlagRequired("owner")

	institution = linkCmd.PersistentFlags().String("institution", "", "")
	linkCmd.MarkPersistentFlagRequired("institution")

	accountType = linkCmd.PersistentFlags().String("type", "transactions", "type of the linked account")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// linkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// linkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
