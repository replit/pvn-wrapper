package googlecloudrun

import (
	"github.com/spf13/cobra"
)

var gcloudPath string

var RootCmd = &cobra.Command{
	Use:   "google-cloud-run <subcommand>",
	Short: "Google Cloud Run wrapper commands",
}

func init() {
	RootCmd.Flags().StringVar(&gcloudPath, "gcloud-path", "gcloud", "Path to gcloud binary")
}
