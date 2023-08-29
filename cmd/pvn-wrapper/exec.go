package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/pvn_wrapper"
	"github.com/prodvana/pvn-wrapper/result"
	"github.com/spf13/cobra"
)

var execFlags = struct {
	in               []string
	out              []string
	successExitCodes []int32
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
		successExitCodes := execFlags.successExitCodes
		if len(successExitCodes) == 0 {
			successExitCodes = []int32{0}
		}
		result.RunWrapper(inputFiles, successExitCodes, func(ctx context.Context) (*pvn_wrapper.Output, []result.OutputFileUpload, error) {
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

			res, cmdOutputs, err := result.RunCmd(execCmd)
			outputs = append(outputs, cmdOutputs...)
			return res, outputs, err
		})
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
	execCmd.Flags().StringArrayVar(&execFlags.in, "in", nil, "List of input files that should be created, in the format input-file-path=input-blob-id. These files will be downloaded from Prodvana and saved to the specified paths before the binary executes.")
	execCmd.Flags().StringArrayVar(&execFlags.out, "out", nil, "List of output files to capture, in the format of output-name=output-file-path. These files will be uploaded to Prodvana.")
	execCmd.Flags().Int32SliceVar(&execFlags.successExitCodes, "success-exit-codes", nil, "List of successful exit codes, used in the event that the program exited but an output file is missing. If the output file is missing and the exit code is a successful exit code as defined here, then the script will fail with an error (and no json output). Defaults to 0.")
}
