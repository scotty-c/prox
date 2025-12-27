package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test the configuration by connecting to the Proxmox server",
	Long:  `Validates the current configuration by attempting to connect to the Proxmox server, authenticate, and retrieve basic information.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Testing Proxmox configuration...")

		// Step 1: Load Configuration
		fmt.Print("1. Loading configuration... ")
		user, pass, url, err := client.ReadConfig()
		if err != nil {
			fmt.Println("✗ Failed")
			fmt.Printf("   Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Success")
		fmt.Printf("   URL: %s\n", url)
		fmt.Printf("   User: %s\n", user)

		// Step 2: Initialize Client
		fmt.Print("2. Initializing client... ")
		// We use NewClient directly instead of CreateClient to avoid caching side effects for a test command
		// and to ensure we are testing the exact credentials we just read
		c := client.NewClient(url, user, pass)
		fmt.Println("✓ Success")

		// Step 3: Authenticate
		fmt.Print("3. Authenticating... ")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = c.Authenticate(ctx)
		if err != nil {
			fmt.Println("✗ Failed")
			fmt.Printf("   Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Success")

		// Step 4: Check API Version
		fmt.Print("4. Checking API version... ")
		version, err := c.GetVersion(ctx)
		if err != nil {
			fmt.Println("✗ Failed")
			fmt.Printf("   Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Success")
		fmt.Printf("   Proxmox VE %s (Release %s)\n", version.Version, version.Release)

		// Step 5: Check Node Access
		fmt.Print("5. Checking node access... ")
		nodes, err := c.GetNodes(ctx)
		if err != nil {
			fmt.Println("✗ Failed")
			fmt.Printf("   Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Success")
		fmt.Printf("   Found %d nodes: ", len(nodes))
		for i, node := range nodes {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%s)", node.Node, node.Status)
		}
		fmt.Println()

		fmt.Println("\nConfiguration is valid and working correctly!")
	},
}

func init() {
	configCmd.AddCommand(testCmd)
}
