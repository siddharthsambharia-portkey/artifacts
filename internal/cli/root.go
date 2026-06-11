package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	ver     string
)

func Execute(version string) error {
	ver = version
	root := &cobra.Command{
		Use:   "artifact",
		Short: "Artifact — internal hosting platform",
		Long:  "Drop a folder of HTML into your company's trust bubble and get a live internal URL with database, AI, files, websockets, and more.",
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "path to artifact.yaml (default: ARTIFACT_CONFIG env or dev defaults)")

	root.AddCommand(serveCmd())
	root.AddCommand(devCmd())
	root.AddCommand(deployCmd())
	root.AddCommand(initCmd())
	root.AddCommand(loginCmd())
	root.AddCommand(listCmd())
	root.AddCommand(openCmd())
	root.AddCommand(logsCmd())
	root.AddCommand(mcpCmd())
	root.AddCommand(versionCmd())

	return root.Execute()
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("artifact %s\n", ver)
		},
	}
}

func exitErr(msg string) {
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	os.Exit(1)
}
