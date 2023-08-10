package terraform

import (
	"github.com/spf13/cobra"
)

var terraformPath string

var RootCmd = &cobra.Command{
	Use:     "terraform <subcommand>",
	Short:   "Terraform wrapper commands",
	Aliases: []string{"tf"},
	Long: `Terraform wrapper commands.

pvn-wrapper terraform plan ...
`,
}

func init() {
	RootCmd.Flags().StringVar(&terraformPath, "terraform-path", "terraform", "Path to terraform binary")
}
