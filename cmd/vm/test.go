package cmd

import (
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to Proxmox server",
	Long:  `Test the connection to your Proxmox server and verify API access.`,
	Run: func(cmd *cobra.Command, args []string) {
		vm.TestConnection()
	},
}

func init() {
	vmCmd.AddCommand(testCmd)
}
