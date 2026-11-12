package cmdutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
)

// NodeSSHArgs returns the ssh arguments (without "ssh") to run remote
// interactively on a Proxmox node
func NodeSSHArgs(host, user string, port int, remote ...string) []string {
	args := []string{"-t"}
	if port != 0 && port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	dest := host
	if user != "" {
		dest = user + "@" + host
	}
	return append(append(args, dest), remote...)
}

// RunSSH runs ssh with args attached to the terminal and exits with ssh's
// exit code if it fails
func RunSSH(args []string) error {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh client not found in PATH: %w", err)
	}
	cmd := exec.Command(sshPath, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Let ssh handle Ctrl-C itself
	signal.Ignore(os.Interrupt)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run ssh: %w", err)
	}
	return nil
}
