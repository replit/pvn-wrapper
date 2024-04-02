package fly

import (
	"github.com/spf13/cobra"
)

var flyPath string

var RootCmd = &cobra.Command{
	Use:   "fly <subcommand>",
	Short: "Fly wrapper commands",
}

func init() {
	RootCmd.Flags().StringVar(&flyPath, "fly-path", "fly", "Path to fly binary")
}
