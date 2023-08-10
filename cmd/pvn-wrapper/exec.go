package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/prodvana/pvn-wrapper/result"
	"github.com/spf13/cobra"
)

var execFlags = struct {
	in  []string
	out []string
}{}

var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a command then wrap its output in a format that Prodvana understands.",
	Long: `Execute a command then wrap its output in a format that Prodvana understands.
The exit code matches the exit code of the underlying binary being executed.

pvn-wrapper exec my-binary --my-flag=value my-args ...
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputFiles := make([]result.InputFile, 0, len(execFlags.in))
		for _, in := range execFlags.in {
			components := strings.SplitN(in, "=", 2)
			if len(components) != 2 {
				log.Fatal("--in must be in the format input-file-path=input-blob-id")
			}
			inputFiles = append(inputFiles, result.InputFile{
				Path:   components[0],
				BlobId: components[1],
			})
		}
		result.RunWrapper(inputFiles, func(ctx context.Context) (*result.ResultType, []result.OutputFileUpload, error) {
			execCmd := exec.CommandContext(ctx, args[0], args[1:]...)
			execCmd.Env = os.Environ()

			outputs := make([]result.OutputFileUpload, 0, len(execFlags.out))
			for _, out := range execFlags.out {
				components := strings.SplitN(out, "=", 2)
				if len(components) != 2 {
					return nil, nil, fmt.Errorf("--out must be in the format output-name=output-file")
				}
				outputs = append(outputs, result.OutputFileUpload{
					Name: components[0],
					Path: components[1],
				})
			}

			res, err := result.RunCmd(execCmd)
			return res, outputs, err
		})
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVar(&execFlags.in, "in", nil, "List of input files that should be created, in the format input-file-path=input-blob-id. These files will be downloaded from Prodvana and saved to the specified paths before the binary executes.")
	execCmd.Flags().StringArrayVar(&execFlags.out, "out", nil, "List of output files to capture, in the format of output-name=output-file-path. These files will be uploaded to Prodvana.")
}
