package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/prodvana/pvn-wrapper/result"
	"github.com/spf13/cobra"
)

var execFlags = struct {
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
		result.RunWrapper(func(ctx context.Context) (*result.ResultType, []result.OutputFileUpload, error) {
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
	execCmd.Flags().StringArrayVar(&execFlags.out, "out", nil, "List of output files to capture, in the format of output-name=output-file. These files will be uploaded to Prodvana.")
}
