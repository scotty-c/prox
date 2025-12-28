package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/scotty-c/prox/pkg/client"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:     "logs <upid>",
	GroupID: "utilities",
	Short:   "View Proxmox task logs",
	Long: `View logs for a Proxmox task using its UPID (Unique Process ID).

The UPID format is typically returned by operations like VM clone, migrate, 
container creation, etc. It looks like:
  UPID:node:00000000:00000000:00000000:vzcreate:100:user@pam:

You can find UPIDs from task output or by monitoring operations that return Task IDs.

Examples:
  prox logs UPID:node1:00012345:001A2B3C:65A1B2C3:vzcreate:100:root@pam:
  prox logs UPID:pve:00012345:001A2B3C:65A1B2C3:qmclone:100:root@pam: --follow
  prox logs <upid> --tail 50`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		upid := args[0]

		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")

		if err := viewTaskLogs(upid, follow, tail); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (stream new lines)")
	logsCmd.Flags().IntP("tail", "n", 0, "Number of lines to show from the end (0 = all)")
	RootCmd.AddCommand(logsCmd)
}

func viewTaskLogs(upid string, follow bool, tail int) error {
	ctx := context.Background()

	// Parse UPID to extract node
	node, err := parseUPIDNode(upid)
	if err != nil {
		return fmt.Errorf("failed to parse UPID: %w", err)
	}

	c, err := client.CreateClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if follow {
		return followTaskLogs(ctx, c, node, upid)
	}

	// Get all logs
	lines, err := c.GetTaskLog(ctx, node, upid, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to get task log: %w", err)
	}

	// Apply tail if specified
	if tail > 0 && len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}

	// Print logs
	for _, line := range lines {
		fmt.Println(line)
	}

	return nil
}

func followTaskLogs(ctx context.Context, c client.ProxmoxClientInterface, node, upid string) error {
	start := 0
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Check if task is still running
	checkTaskStatus := func() (bool, error) {
		task, err := c.GetTaskStatus(ctx, node, upid)
		if err != nil {
			return false, err
		}
		return task.Status != "stopped", nil
	}

	for {
		// Get new log lines
		lines, err := c.GetTaskLog(ctx, node, upid, start, 0)
		if err != nil {
			return fmt.Errorf("failed to get task log: %w", err)
		}

		// Print new lines
		for _, line := range lines {
			fmt.Println(line)
		}

		// Update start position for next fetch
		if len(lines) > 0 {
			start += len(lines)
		}

		// Check if task is still running
		running, err := checkTaskStatus()
		if err != nil {
			return fmt.Errorf("failed to check task status: %w", err)
		}

		if !running {
			// Task finished, get any remaining logs and exit
			lines, _ = c.GetTaskLog(ctx, node, upid, start, 0)
			for _, line := range lines {
				fmt.Println(line)
			}
			break
		}

		// Wait before next poll
		<-ticker.C
	}

	return nil
}

func parseUPIDNode(upid string) (string, error) {
	// UPID format: UPID:node:pid:pstart:starttime:type:id:user:
	// We need to extract the node part
	parts := []rune{}
	colonCount := 0

	for _, char := range upid {
		if char == ':' {
			colonCount++
			if colonCount == 2 {
				// We've collected the node part
				break
			}
			continue
		}

		if colonCount == 1 {
			parts = append(parts, char)
		}
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("invalid UPID format: expected UPID:node:... got %s", upid)
	}

	return string(parts), nil
}
