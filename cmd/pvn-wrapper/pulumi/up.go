package pulumi

import (
	"bytes"
	"context"
	go_errors "errors"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var upFlags = struct {
	retryableExitCode int
}{}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "pulumi up wrapper",
	Long: `pulumi up wrapper.

Takes all the same input that pulumi up would, but handles detecting retryable errors and exiting
with a special exit code (default: 2). Retryable errors include:

- lock acquisition errors

Otherwise, pvn-wrapper will exit with the original exit code of pulumi up. Note that this command will buffer
stderr to look at potential error messages, therefore, the order of stderr/stdout will be different
than pulumi up's.

pvn-wrapper pulumi up ...

To pass flags to pulumi up, use --

pvn-wrapper terraform apply -- --show-sames

pvn-wrapper will always pass --non-interactive.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		applyArgs := []string{"up"}
		applyArgs = append(applyArgs, args...)
		applyArgs = append(applyArgs,
			"--non-interactive",
		)
		execCmd := exec.CommandContext(ctx, pulumiPath, applyArgs...)
		var stderr bytes.Buffer
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = &stderr
		execErr := execCmd.Run()
		// write out stderr before doing any processing to be transparent
		_, err := os.Stderr.Write(stderr.Bytes())
		if err != nil {
			return errors.Wrap(err, "failed to write to stderr")
		}
		if execErr != nil {
			stderrString := stderr.String()
			if strings.Contains(stderrString, "the stack is currently locked") {
				os.Exit(upFlags.retryableExitCode)
			}
			// other errors, try to match original exit code
			var exitErr *exec.ExitError
			if go_errors.As(execErr, &exitErr) {
				os.Exit(exitErr.ExitCode())
			} else {
				return errors.Wrap(execErr, "up command failed unexpectedly")
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(upCmd)

	upCmd.Flags().IntVar(&upFlags.retryableExitCode, "retryable-exit-code", 2, "Special exit code to use for retryable errors.")
}
