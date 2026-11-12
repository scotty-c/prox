package container

import (
	"context"
	"fmt"
	"strings"
	"time"

	c "github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/output"
)

// finishTask prints the task ID and, if wait, waits for it and prints done
func finishTask(ctx context.Context, client c.ProxmoxClientInterface, node, taskID string, wait bool, done string) error {
	output.Result("Task started: %s\n", taskID)
	if !wait {
		output.Info("Tip: Follow progress with 'prox logs --follow %s'\n", taskID)
		return nil
	}
	start := time.Now()
	if err := waitForTask(ctx, client, node, taskID); err != nil {
		return err
	}
	output.Result("%s in %s\n", done, time.Since(start).Round(time.Millisecond))
	return nil
}

// checkIDFree returns an error if a VM or container already uses id
// (they share one ID space)
func checkIDFree(ctx context.Context, client c.ProxmoxClientInterface, id int) error {
	resources, err := client.GetClusterResources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster resources: %w", err)
	}
	for _, r := range resources {
		if (r.Type == "qemu" || r.Type == "lxc") && r.VMID != nil && *r.VMID == id {
			kind := "VM"
			if r.Type == "lxc" {
				kind = "container"
			}
			return fmt.Errorf("ID %d is already in use by %s '%s' on node %s", id, kind, r.Name, r.Node)
		}
	}
	return nil
}

// CloneContainer clones a container. newID 0 uses the next free ID. full
// makes a full copy; a linked clone is only possible from a template.
func CloneContainer(sourceNameOrID string, newID int, name string, full, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	source, err := FindByNameOrID(ctx, client, sourceNameOrID)
	if err != nil {
		return fmt.Errorf("failed to find source container: %w", err)
	}

	if newID == 0 {
		if newID, err = client.GetNextVMID(ctx); err != nil {
			return fmt.Errorf("failed to get next ID: %w", err)
		}
	} else if err := checkIDFree(ctx, client, newID); err != nil {
		return err
	}

	cloneType := "linked"
	if full {
		cloneType = "full"
	}
	output.Info("Cloning container %s (ID: %d) to %s (ID: %d) on node %s using a %s clone...\n",
		source.Name, source.ID, name, newID, source.Node, cloneType)

	taskID, err := client.CloneContainer(ctx, source.Node, source.ID, newID, name, full)
	if err != nil {
		if !full && strings.Contains(strings.ToLower(err.Error()), "linked clone") {
			return fmt.Errorf("%w\n\nTip: linked clones need a template as the source; use --full to copy the container", err)
		}
		return fmt.Errorf("failed to clone container: %w", err)
	}
	return finishTask(ctx, client, source.Node, taskID, wait, fmt.Sprintf("Container %s (ID: %d) created", name, newID))
}

// MigrateContainer moves a container to targetNode. A running container is
// only moved with restart (stopped, moved and started again).
func MigrateContainer(nameOrID, targetNode string, restart, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}

	ct, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}
	if ct.Node == targetNode {
		return fmt.Errorf("container %s is already on node %s", ct.Name, targetNode)
	}
	if ct.Status == "running" && !restart {
		return fmt.Errorf("container %s is running; use --restart to stop it, move it and start it again on %s", ct.Name, targetNode)
	}

	output.Info("Migrating container %s (ID: %d) from %s to %s...\n", ct.Name, ct.ID, ct.Node, targetNode)
	taskID, err := client.MigrateContainer(ctx, ct.Node, ct.ID, targetNode, restart)
	if err != nil {
		return fmt.Errorf("failed to start migration: %w", err)
	}
	return finishTask(ctx, client, ct.Node, taskID, wait, fmt.Sprintf("Container %s is now on node %s", ct.Name, targetNode))
}

// EditContainer changes a container's hostname, cores or memory, and grows
// its root disk by diskGB. Zero values are left unchanged.
func EditContainer(nameOrID, hostname string, cores, memoryMB, diskGB int) error {
	if hostname == "" && cores <= 0 && memoryMB <= 0 && diskGB <= 0 {
		return fmt.Errorf("no changes specified: use --name, --cpu, --memory or --disk")
	}

	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}
	ct, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}

	config := map[string]interface{}{}
	if hostname != "" {
		config["hostname"] = hostname
		output.Info("   • Hostname: %s\n", hostname)
	}
	if cores > 0 {
		config["cores"] = cores
		output.Info("   • CPU cores: %d\n", cores)
	}
	if memoryMB > 0 {
		config["memory"] = memoryMB
		output.Info("   • Memory: %d MB\n", memoryMB)
	}

	if len(config) > 0 {
		if err := client.UpdateContainer(ctx, ct.Node, ct.ID, config); err != nil {
			return fmt.Errorf("failed to update container %d: %w", ct.ID, err)
		}
		output.Result("Container %s (ID: %d) configuration updated\n", ct.Name, ct.ID)
	}

	if diskGB > 0 {
		output.Info("Growing root disk of container %s by %d GB...\n", ct.Name, diskGB)
		taskID, err := client.ResizeContainerDisk(ctx, ct.Node, ct.ID, "rootfs", fmt.Sprintf("+%dG", diskGB))
		if err != nil {
			return fmt.Errorf("failed to resize container %d disk: %w", ct.ID, err)
		}
		return finishTask(ctx, client, ct.Node, taskID, true, fmt.Sprintf("Root disk of container %s grown by %d GB", ct.Name, diskGB))
	}
	return nil
}

// RebootContainer reboots a running container
func RebootContainer(nameOrID string, wait bool) error {
	ctx := context.Background()
	client, err := c.CreateClient()
	if err != nil {
		return fmt.Errorf("error creating client: %w", err)
	}
	ct, err := FindByNameOrID(ctx, client, nameOrID)
	if err != nil {
		return fmt.Errorf("failed to find container: %w", err)
	}
	if ct.Status != "running" {
		return fmt.Errorf("container %s is not running (status: %s); use 'prox ct start %s'", ct.Name, ct.Status, ct.Name)
	}

	output.Info("Rebooting container %s (ID: %d)...\n", ct.Name, ct.ID)
	taskID, err := client.RebootGuest(ctx, ct.Node, c.GuestContainer, ct.ID)
	if err != nil {
		return fmt.Errorf("failed to reboot container: %w", err)
	}
	return finishTask(ctx, client, ct.Node, taskID, wait, fmt.Sprintf("Container %s rebooted", ct.Name))
}
