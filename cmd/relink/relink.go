/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package relink

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/link"
	"github.com/xiaomi388/beancount-automation/pkg/types"
)

var (
	owner           *string
	institution     *string
	institutionType *string
)

// linkCmd represents the link command
var RelinkCmd = &cobra.Command{
	Use:   "relink",
	Short: "relink an institution",
	Run: func(_ *cobra.Command, _ []string) {
		err := link.Relink(*owner, *institution, types.InstitutionType(*institutionType))
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	owner = RelinkCmd.PersistentFlags().String("owner", "", "")
	_ = RelinkCmd.MarkPersistentFlagRequired("owner")

	institution = RelinkCmd.PersistentFlags().String("institution", "", "")
	_ = RelinkCmd.MarkPersistentFlagRequired("institution")

	institutionType = RelinkCmd.PersistentFlags().String("type", "transactions", "type of the linked account")
	_ = RelinkCmd.MarkPersistentFlagRequired("type")
}
