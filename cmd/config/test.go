package cmd

import (
	"context"
	"os"
	"time"

	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test the configuration by connecting to the Proxmox server",
	Long:  `Validates the current configuration by attempting to connect to the Proxmox server, authenticate, and retrieve basic information.`,
	Run: func(cmd *cobra.Command, args []string) {
		output.Resultln("Testing Proxmox configuration...")

		// Step 1: Load Configuration
		output.Info("1. Loading configuration... ")
		user, pass, url, err := client.ReadConfig()
		if err != nil {
			output.Errorln("FAILED")
			output.Error("   Error: %v\n", err)
			os.Exit(1)
		}
		output.Resultln("OK")
		output.Result("   URL: %s\n", url)
		output.Result("   User: %s\n", user)

		// Step 2: Initialize Client
		output.Info("2. Initializing client... ")
		// We use NewClient directly instead of CreateClient to avoid caching side effects for a test command
		// and to ensure we are testing the exact credentials we just read
		c := client.NewClient(url, user, pass)
		output.Resultln("OK")

		// Step 3: Authenticate
		output.Info("3. Authenticating... ")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = c.Authenticate(ctx)
		if err != nil {
			output.Errorln("FAILED")
			output.Error("   Error: %v\n", err)
			os.Exit(1)
		}
		output.Resultln("OK")

		// Step 4: Check API Version
		output.Info("4. Checking API version... ")
		version, err := c.GetVersion(ctx)
		if err != nil {
			output.Errorln("FAILED")
			output.Error("   Error: %v\n", err)
			os.Exit(1)
		}
		output.Resultln("OK")
		output.Result("   Proxmox VE %s (Release %s)\n", version.Version, version.Release)

		// Step 5: Check Node Access
		output.Info("5. Checking node access... ")
		nodes, err := c.GetNodes(ctx)
		if err != nil {
			output.Errorln("FAILED")
			output.Error("   Error: %v\n", err)
			os.Exit(1)
		}
		output.Resultln("OK")
		output.Result("   Found %d nodes: ", len(nodes))
		for i, node := range nodes {
			if i > 0 {
				output.Result(", ")
			}
			output.Result("%s (%s)", node.Node, node.Status)
		}
		output.Resultln("")

		output.Resultln("\nConfiguration is valid and working correctly!")
	},
}

func init() {
	configCmd.AddCommand(testCmd)
}
