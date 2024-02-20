/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/xiaomi388/beancount-automation/cmd/dump"
	"github.com/xiaomi388/beancount-automation/cmd/link"
	"github.com/xiaomi388/beancount-automation/cmd/relink"
	"github.com/xiaomi388/beancount-automation/cmd/sync"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "beancountautomation",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	//p, err := os.Getwd()
	//if err != nil {
	//	panic(err)
	//}
	//rootCmd.PersistentFlags().StringVar(&config.ConfigPath, "config", filepath.Join(p, "config.yaml"), "config file (default is config.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	logrus.SetLevel(logrus.DebugLevel)

	rootCmd.AddCommand(dump.DumpCmd)
	rootCmd.AddCommand(sync.SyncCmd)
	rootCmd.AddCommand(link.LinkCmd)
	rootCmd.AddCommand(relink.RelinkCmd)

}
