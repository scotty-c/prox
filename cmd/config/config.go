/*
Copyright © 2024 prox contributors
*/
package cmd

import (
	"github.com/scotty-c/prox/cmd"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:     "config",
	GroupID: "core",
	Short:   "Set up, read, and delete the configuration file for prox",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	cmd.RootCmd.AddCommand(configCmd)

}
