package cmd

import (
	"os"

	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit <name|id> [flags]",
	Short: "Edit container configuration",
	Long: `Change a container's hostname, CPU cores or memory, or grow its root disk.
CPU and memory changes apply to a running container immediately.

Examples:
  prox ct edit web --cpu 2 --memory 2048
  prox ct edit web --name web-old
  prox ct edit web --disk 10            # Grow the root disk by 10 GB`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetContainerNames),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		cpu, _ := cmd.Flags().GetInt("cpu")
		memory, _ := cmd.Flags().GetInt("memory")
		disk, _ := cmd.Flags().GetInt("disk")
		if err := container.EditContainer(args[0], name, cpu, memory, disk); err != nil {
			output.UserError("editing container", err)
			os.Exit(1)
		}
	},
}

func init() {
	editCmd.Flags().StringP("name", "N", "", "Container hostname")
	editCmd.Flags().IntP("cpu", "c", 0, "Number of CPU cores")
	editCmd.Flags().IntP("memory", "m", 0, "Memory size in MB")
	editCmd.Flags().IntP("disk", "d", 0, "Increase root disk size by this many GB")
	ctCmd.AddCommand(editCmd)
}
