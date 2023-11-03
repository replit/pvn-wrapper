package pulumi

import (
	"github.com/spf13/cobra"
)

var pulumiPath string

var RootCmd = &cobra.Command{
	Use:     "pulumi <subcommand>",
	Short:   "Pulumi wrapper commands",
	Long: `Pulumi wrapper commands.

pvn-wrapper pulumi preview ...
`,
}

func init() {
	RootCmd.Flags().StringVar(&pulumiPath, "pulumi-path", "pulumi", "Path to the pulumi binary")
}
