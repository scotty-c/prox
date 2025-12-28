package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	versionInfo = struct {
		Version   string
		Commit    string
		BuildDate string
		GoVersion string
	}{
		Version:   "dev",
		Commit:    "unknown",
		BuildDate: "unknown",
		GoVersion: "unknown",
	}
)

// SetVersionInfo sets the version information from build-time variables
func SetVersionInfo(version, commit, buildDate, goVersion string) {
	versionInfo.Version = version
	versionInfo.Commit = commit
	versionInfo.BuildDate = buildDate
	if goVersion == "unknown" {
		versionInfo.GoVersion = runtime.Version()
	} else {
		versionInfo.GoVersion = goVersion
	}
}

var versionCmd = &cobra.Command{
	Use:     "version",
	GroupID: "utilities",
	Short:   "Display version information",
	Long:    `Display detailed version information including version number, git commit, build date, and Go version.`,
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")

		if verbose {
			fmt.Printf("prox version information:\n")
			fmt.Printf("  Version:    %s\n", versionInfo.Version)
			fmt.Printf("  Git Commit: %s\n", versionInfo.Commit)
			fmt.Printf("  Build Date: %s\n", versionInfo.BuildDate)
			fmt.Printf("  Go Version: %s\n", versionInfo.GoVersion)
		} else {
			fmt.Printf("prox %s\n", versionInfo.Version)
		}
	},
}

func init() {
	versionCmd.Flags().BoolP("verbose", "v", false, "Show detailed version information")
	RootCmd.AddCommand(versionCmd)
}
