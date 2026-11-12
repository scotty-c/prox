package cmd

import (
	"fmt"
	"os"

	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
)

// readCmd represents the read command to get the contents of the config file

var readCmd = &cobra.Command{
	Use:   "read",
	Short: "This command reads the configuration file for prox",
	Long: `Show the current profile's saved configuration.

The password is masked unless --show-password is given.`,
	Run: func(cmd *cobra.Command, args []string) {
		user, pass, url, err := c.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Username: ", user)
		showPassword, _ := cmd.Flags().GetBool("show-password")
		if showPassword {
			fmt.Println("Password: ", pass)
		} else {
			fmt.Println("Password: ", maskPassword(pass))
		}
		fmt.Println("URL: ", url)
	},
}

// maskPassword hides a password without revealing its length
func maskPassword(pass string) string {
	if pass == "" {
		return "(not set)"
	}
	return "******** (use --show-password to reveal)"
}

func init() {
	readCmd.Flags().Bool("show-password", false, "Show the saved password in clear text")
	configCmd.AddCommand(readCmd)
}
