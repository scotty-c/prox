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
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		user, pass, url, err := c.Read()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Username: ", user)
		fmt.Println("Password: ", pass)
		fmt.Println("URL: ", url)
	},
}

func init() {
	configCmd.AddCommand(readCmd)
}
