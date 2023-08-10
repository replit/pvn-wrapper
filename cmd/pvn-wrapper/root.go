package main

import (
	"log"

	"github.com/prodvana/pvn-wrapper/cmd/pvn-wrapper/terraform"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "pvn-wrapper",
	Short:            "pvn-wrapper is used to facilitate executions of jobs in Prodvana.",
	Long:             `pvn-wrapper is used to facilitate executions of jobs in Prodvana.`,
	TraverseChildren: true,
}

func init() {
	rootCmd.AddCommand(terraform.RootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
