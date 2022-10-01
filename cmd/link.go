/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/pkg/link"
)

var (
	owner       *string
	institution *string
)

// linkCmd represents the link command
var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "link an institution",
	Run: func(_ *cobra.Command, _ []string) {
		err := link.Link(*owner, *institution)
		if err != nil {
			fmt.Println(err)
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

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// linkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// linkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
