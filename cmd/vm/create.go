package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/scotty-c/prox/pkg/cmdutil"
	"github.com/scotty-c/prox/pkg/completion"
	"github.com/scotty-c/prox/pkg/container"
	"github.com/scotty-c/prox/pkg/output"
	"github.com/scotty-c/prox/pkg/vm"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name> --from <template>",
	Short: "Create a VM from a cloud-init template",
	Long: `Create a VM by cloning a cloud-init template, then set its resources, network,
user and SSH keys, optionally grow its disk, and optionally start it.

Settings you don't give keep the template's values. Without --id the next free
ID is used. A linked clone is made unless --full is given (or the storage
doesn't support linked clones).

Examples:
  prox vm create web --from ubuntu-24-template --start
  prox vm create web --from 9000 --cpu 2 --memory 4096 --disk 20 \
    --ip 10.0.0.20/24 --gateway 10.0.0.1 --ci-user admin \
    --ssh-keys-file ~/.ssh/id_ed25519.pub --start --wait`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := vm.CreateOptions{Name: args[0]}
		opts.Template, _ = cmd.Flags().GetString("from")
		opts.Node, _ = cmd.Flags().GetString("node")
		opts.NewID, _ = cmd.Flags().GetInt("id")
		opts.Full, _ = cmd.Flags().GetBool("full")
		opts.Cores, _ = cmd.Flags().GetInt("cpu")
		opts.MemoryMB, _ = cmd.Flags().GetInt("memory")
		opts.DiskGB, _ = cmd.Flags().GetInt("disk")
		opts.IP, _ = cmd.Flags().GetString("ip")
		opts.Gateway, _ = cmd.Flags().GetString("gateway")
		opts.CIUser, _ = cmd.Flags().GetString("ci-user")
		opts.Start, _ = cmd.Flags().GetBool("start")
		opts.Wait = cmdutil.ResolveWait(cmd, false)

		if keysFile, _ := cmd.Flags().GetString("ssh-keys-file"); keysFile != "" {
			keys, err := readKeysFile(keysFile)
			if err != nil {
				output.UserError("reading SSH keys", err)
				os.Exit(1)
			}
			opts.SSHKeys = keys
		}

		if err := vm.CreateFromTemplate(opts); err != nil {
			output.VMError("create", err)
			os.Exit(1)
		}
	},
}

// readKeysFile reads and validates public keys from a file, or stdin for "-"
func readKeysFile(path string) (string, error) {
	f := os.Stdin
	if path != "-" {
		var err error
		if f, err = os.Open(path); err != nil {
			return "", err
		}
		defer f.Close()
	}
	keys, _, err := container.ReadSSHKeys(f)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(keys) == "" {
		return "", fmt.Errorf("no keys found in %s", path)
	}
	if _, err := container.ValidateSSHKeys(keys); err != nil {
		return "", err
	}
	return keys, nil
}

func init() {
	createCmd.Flags().String("from", "", "Template VM to clone (name or ID)")
	_ = createCmd.MarkFlagRequired("from")
	_ = createCmd.RegisterFlagCompletionFunc("from", completion.GetVMNames)
	createCmd.Flags().StringP("node", "n", "", "Only match the template on this node")
	createCmd.Flags().Int("id", 0, "ID for the new VM (default: next free ID)")
	createCmd.Flags().BoolP("full", "f", false, "Make a full clone instead of a linked clone")
	createCmd.Flags().IntP("cpu", "c", 0, "Number of CPU cores (default: template's)")
	createCmd.Flags().IntP("memory", "m", 0, "Memory in MB (default: template's)")
	createCmd.Flags().IntP("disk", "d", 0, "Grow the primary disk by this many GB")
	createCmd.Flags().String("ip", "", "IPv4 for the first interface: dhcp, or an address with prefix length (default: template's)")
	createCmd.Flags().String("gateway", "", "IPv4 gateway (with a static --ip)")
	createCmd.Flags().String("ci-user", "", "Cloud-init user name (default: template's)")
	createCmd.Flags().String("ssh-keys-file", "", "Public SSH key file for cloud-init, or '-' for stdin")
	createCmd.Flags().Bool("start", false, "Start the VM after creating it")
	cmdutil.AddWaitFlags(createCmd, false)
	vmCmd.AddCommand(createCmd)
	completion.RegisterNodeFlag(createCmd, "node")
}
