package awsecs

import (
	"github.com/spf13/cobra"
)

var awsPath string

var RootCmd = &cobra.Command{
	Use:   "aws-ecs <subcommand>",
	Short: "AWS ECS wrapper commands",
	Long: `AWS ECS wrapper commands.

pvn-wrapper awsecs apply ...
`,
}

func init() {
	RootCmd.Flags().StringVar(&awsPath, "aws-path", "aws", "Path to aws binary")
}
