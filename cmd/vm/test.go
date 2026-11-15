package cmd

import (
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:    "test",
	Hidden: true, // a connection check, not a VM operation; use 'prox config test' or 'prox status'
	Short:  "Test connection to Proxmox server",
	Long:   `Test the connection to your Proxmox server and verify API access.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := vm.TestConnection(cmd.Context()); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	vmCmd.AddCommand(testCmd)
}
