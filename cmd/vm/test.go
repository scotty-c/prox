package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to Proxmox server",
	Long:  `Test the connection to your Proxmox server and verify API access.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := vm.TestConnection(); err != nil {
			os.Exit(1)
		}
	},
}

func init() {
	vmCmd.AddCommand(testCmd)
}
