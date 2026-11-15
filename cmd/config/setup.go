package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/scotty-c/prox/pkg/apierr"
	"github.com/scotty-c/prox/pkg/client"
	"github.com/scotty-c/prox/pkg/completion"
	c "github.com/scotty-c/prox/pkg/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// setupCmd creates a profile or changes a saved one
var setupCmd = &cobra.Command{
	Use:   "setup [profile]",
	Short: "Create a connection profile, or change a saved one",
	Long: `Save the connection details for a profile, creating it if it does not exist
and changing it if it does. Without a profile name the current profile is used.

Only what you give is changed: for an existing profile, anything you leave out
keeps its saved value. On a terminal, prox asks for whatever a new profile is
still missing.

Before anything is saved, prox connects with the details to check that they
work, and saves nothing if they do not. Use --no-verify to save without
checking, for example before the server exists.

Certificates are verified. Proxmox's default certificate is self-signed, so
the first time you set up a server that uses it, prox shows the certificate's
SHA-256 fingerprint and asks you to compare it with the one in the Proxmox web
interface (node > System > Certificates). If you type 'yes', the fingerprint is
saved in the profile and only that certificate is trusted for it from then on,
the way ssh remembers a host key. Without a terminal, pass the fingerprint
yourself with --fingerprint. The alternatives are --ca-cert, to trust a CA file
(a copy of /etc/pve/pve-root-ca.pem), and --insecure, to skip verification.

Examples:
  prox config setup                               # prompts for what is missing
  prox config setup homelab -u root@pam -l https://pve.example.com:8006
  prox config setup homelab --use                 # and make it the current profile
  prox config setup --url https://pve2.example.com:8006
  echo "$TOKEN_SECRET" | prox config setup ci -u 'ci@pve!deploy' -l https://pve:8006 --password-stdin
  prox config setup --fingerprint AA:BB:CC:...    # trust one certificate, no prompt`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completion.FirstArgOnly(completion.GetProfileNames),
	RunE:              runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	named := len(args) == 1
	profile := c.GetCurrentProfile()
	if named {
		profile = args[0]
	}
	if _, err := c.GetProfilePath(profile); err != nil { // rejects names that are not valid
		return err
	}
	existed := c.ProfileExists(profile)
	saved, _ := c.ListProfiles()
	first := len(saved) == 0
	wasCurrent := profile == c.GetCurrentProfile()

	username, password, url, err := readCredentialFlags(cmd)
	if err != nil {
		return err
	}

	opts := c.TLSOptions{}
	if existed {
		if opts, err = c.ReadTLSOptions(profile); err != nil {
			return err
		}
	}
	opts, chosenTrust, err := applyTLSFlags(cmd, opts)
	if err != nil {
		return err
	}

	// Anything not given keeps its saved value
	if existed && (username == "" || password == "" || url == "") {
		u, p, l, err := c.ReadProfile(profile)
		if err != nil {
			return fmt.Errorf("reading the saved profile '%s': %w\n(to replace it instead, give --username, --password and --url)", profile, err)
		}
		username, password, url = firstNonEmpty(username, u), firstNonEmpty(password, p), firstNonEmpty(url, l)
	}

	in := bufio.NewReader(os.Stdin)
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	if interactive {
		if username, password, url, err = promptMissing(in, username, password, url); err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		if url, _, err = c.NormalizeURL(url); err != nil {
			return err
		}
	}
	// A profile without these cannot connect, so don't save one
	var missing []string
	for flag, value := range map[string]string{"--url": url, "--username": username, "--password": password} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, flag)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("a profile needs a URL, username and password; missing %s (run on a terminal to be prompted)", strings.Join(missing, ", "))
	}

	version := ""
	if noVerify, _ := cmd.Flags().GetBool("no-verify"); !noVerify {
		check := checkConnection
		if opts, version, err = verifyAndTrust(cmd.Context(), check, in, os.Stderr, interactive, profile, url, username, password, opts, chosenTrust); err != nil {
			return err
		}
	}

	method, err := c.SaveProfile(profile, username, password, url, opts)
	if err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}
	fmt.Printf("Profile '%s' saved (credentials stored %s)\n", profile, c.DescribeMethod(method))
	if version != "" {
		fmt.Printf("Connected to Proxmox VE %s as %s\n", version, username)
	}
	switch {
	case opts.Insecure:
		fmt.Println("TLS: certificate verification is turned off for this profile")
	case opts.Fingerprint != "":
		fmt.Printf("TLS: only the certificate with SHA-256 fingerprint %s is trusted\n", opts.Fingerprint)
	case opts.CACert != "":
		fmt.Printf("TLS: certificates are verified against %s\n", opts.CACert)
	}

	// The current profile follows the command unless a new one is named: the
	// first profile is always usable at once, any other needs --use
	if useIt, _ := cmd.Flags().GetBool("use"); (useIt || !named || first) && !wasCurrent {
		if err := c.SetCurrentProfile(profile); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: profile saved but failed to switch to it: %v\n", err)
		} else {
			fmt.Printf("Switched to profile: %s\n", profile)
		}
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// applyTLSFlags applies --insecure, --ca-cert and --fingerprint to the TLS
// settings already saved. A CA file and a pinned fingerprint are alternative
// ways to say who to trust, so giving one replaces the other. It also reports
// whether this run chose a way to trust the server at all.
func applyTLSFlags(cmd *cobra.Command, opts c.TLSOptions) (c.TLSOptions, bool, error) {
	insecureSet := cmd.Flags().Changed("insecure")
	caCert, _ := cmd.Flags().GetString("ca-cert")
	fingerprint, _ := cmd.Flags().GetString("fingerprint")
	if caCert != "" && fingerprint != "" {
		return opts, false, errors.New("use either --ca-cert or --fingerprint, not both")
	}
	if insecureSet {
		opts.Insecure, _ = cmd.Flags().GetBool("insecure")
	}
	if caCert != "" {
		abs, err := filepath.Abs(caCert)
		if err != nil {
			return opts, false, fmt.Errorf("invalid CA certificate path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return opts, false, fmt.Errorf("CA certificate file: %w", err)
		}
		opts.CACert, opts.Fingerprint = abs, ""
	}
	if fingerprint != "" {
		fp, err := c.NormalizeFingerprint(fingerprint)
		if err != nil {
			return opts, false, fmt.Errorf("invalid --fingerprint: %w", err)
		}
		opts.Fingerprint, opts.CACert = fp, ""
	}
	return opts, insecureSet || caCert != "" || fingerprint != "", nil
}

// checker connects with the given details and returns the Proxmox version
type checker func(ctx context.Context, url, username, password string, opts c.TLSOptions) (string, error)

// checkConnection logs in and asks the server for its version
func checkConnection(ctx context.Context, url, username, password string, opts c.TLSOptions) (string, error) {
	cl, err := client.NewClientWithTLS(url, username, password, opts)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := cl.Authenticate(ctx); err != nil {
		return "", err
	}
	v, err := cl.GetVersion(ctx)
	if err != nil {
		return "", err
	}
	return v.Version, nil
}

// verifyAndTrust checks that the details work before they are saved. If the
// only problem is a certificate that is not trusted and nothing was chosen to
// say otherwise, it offers to pin that certificate. It returns the TLS settings
// to save and the server's version; on any error nothing should be saved.
func verifyAndTrust(ctx context.Context, check checker, in *bufio.Reader, w io.Writer, interactive bool,
	profile, url, username, password string, opts c.TLSOptions, chosenTrust bool) (c.TLSOptions, string, error) {

	version, err := check(ctx, url, username, password, opts)
	if err == nil {
		return opts, version, nil
	}

	if errors.Is(err, apierr.ErrCertificate) && !chosenTrust && !opts.Insecure {
		cert, probeErr := client.ProbeCertificate(ctx, url)
		if probeErr != nil {
			return opts, "", err
		}
		fmt.Fprint(w, describeCertificate(url, cert, opts.Fingerprint))
		if !interactive {
			return opts, "", fmt.Errorf("%w: the certificate presented by %s is not trusted, so nothing was saved.\n"+
				"Compare its fingerprint (above) with the Proxmox web interface (node > System > Certificates), then run again with\n"+
				"  --fingerprint %s\n(or use --ca-cert or --insecure)", apierr.ErrCertificate, url, cert.Fingerprint)
		}
		if !askYes(in, w, fmt.Sprintf("Type 'yes' to trust this certificate and save its fingerprint in profile '%s': ", profile), true) {
			return opts, "", fmt.Errorf("%w: certificate not trusted, so nothing was saved", apierr.ErrCertificate)
		}
		opts.Fingerprint, opts.CACert = cert.Fingerprint, ""
		// the trusted certificate must be the one that now logs in
		if version, err = check(ctx, url, username, password, opts); err != nil {
			return opts, "", fmt.Errorf("connecting with the trusted certificate failed, so nothing was saved: %w", err)
		}
		return opts, version, nil
	}

	if interactive && !errors.Is(err, apierr.ErrCertificate) {
		fmt.Fprintf(w, "Could not connect with these details: %v\n", err)
		if askYes(in, w, "Save the profile anyway? [y/N]: ", false) {
			return opts, "", nil
		}
	}
	return opts, "", fmt.Errorf("could not connect with these details, so nothing was saved (--no-verify saves without checking): %w", err)
}

// askYes asks a question and reports whether the answer was yes. With strict,
// only the whole word counts, as ssh requires for a new host key.
func askYes(in *bufio.Reader, w io.Writer, question string, strict bool) bool {
	fmt.Fprint(w, question)
	line, _ := in.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "yes" || (!strict && answer == "y")
}

// describeCertificate shows what a person needs to decide whether to trust a
// server's certificate. previous is the fingerprint the profile pinned before,
// if any.
func describeCertificate(url string, cert *client.ServerCertificate, previous string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s presented a certificate that is not trusted:\n", url)
	fmt.Fprintf(&b, "  Subject:      %s\n", cert.Subject)
	if cert.SelfSigned {
		fmt.Fprintf(&b, "  Issuer:       (self-signed)\n")
	} else {
		fmt.Fprintf(&b, "  Issuer:       %s\n", cert.Issuer)
	}
	if len(cert.DNSNames) > 0 {
		fmt.Fprintf(&b, "  Names:        %s\n", strings.Join(cert.DNSNames, ", "))
	}
	fmt.Fprintf(&b, "  Valid:        %s to %s\n", cert.NotBefore.Format("2006-01-02"), cert.NotAfter.Format("2006-01-02"))
	fmt.Fprintf(&b, "  SHA-256:      %s\n", cert.Fingerprint)
	if previous != "" {
		fmt.Fprintf(&b, "\nThis profile pins a different certificate (%s).\nIf you did not expect the certificate to change, do not trust this one.\n", previous)
	}
	b.WriteString("\nCompare the SHA-256 fingerprint with the one in the Proxmox web interface\n(node > System > Certificates), not with anything this server sends you.\n\n")
	return b.String()
}

// promptMissing asks for any connection details that are empty. Prompts go to
// stderr, so stdout stays for results.
func promptMissing(in *bufio.Reader, username, password, url string) (string, string, string, error) {
	ask := func(label, current string) (string, error) {
		if current != "" {
			return current, nil
		}
		fmt.Fprintf(os.Stderr, "%s: ", label)
		line, err := in.ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}

	var err error
	if url, err = ask("Proxmox URL (e.g. https://pve.example.com:8006)", url); err != nil {
		return "", "", "", err
	}
	if username, err = ask("Username (e.g. root@pam, or token ID user@realm!name)", username); err != nil {
		return "", "", "", err
	}
	if password == "" {
		fmt.Fprint(os.Stderr, "Password (or token secret): ")
		bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", "", "", err
		}
		password = string(bytes)
	}
	return username, password, url, nil
}

func init() {
	addCredentialFlags(setupCmd)
	setupCmd.Flags().Bool("use", false, "Make this the current profile (the profile you set up without naming one, or your first, is current anyway)")
	setupCmd.Flags().Bool("no-verify", false, "Save without connecting to check the details first")
	configCmd.AddCommand(setupCmd)
}
