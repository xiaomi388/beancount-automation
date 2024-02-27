/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package link

import (
	"fmt"
	"os"

	"github.com/plaid/plaid-go/plaid"
	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/link"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

var (
	owner       *string
	institution *string
	accountType *string
)

// LinkCmd represents the link command
var LinkCmd = &cobra.Command{
	Use:   "link",
	Short: "link an institution",
	Run: func(_ *cobra.Command, _ []string) {
		if *accountType != string(plaid.PRODUCTS_INVESTMENTS) && *accountType != string(plaid.PRODUCTS_TRANSACTIONS) {
			fmt.Println("account type should be either investments or transactions")
			os.Exit(1)
		}

		err := link.Link(*owner, *institution, types.InstitutionType(*accountType))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	// Here you will define your flags and configuration settings.
	owner = LinkCmd.PersistentFlags().String("owner", "", "")
	LinkCmd.MarkPersistentFlagRequired("owner")

	institution = LinkCmd.PersistentFlags().String("institution", "", "")
	LinkCmd.MarkPersistentFlagRequired("institution")

	accountType = LinkCmd.PersistentFlags().String("type", "transactions", "type of the linked account")
}
