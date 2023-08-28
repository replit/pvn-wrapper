package terraform

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

var applyFlags = struct {
	retryableExitCode int
}{}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "terraform apply wrapper",
	Long: `terraform apply wrapper.

Takes all the same input that terraform apply would, but handles detecting retryable errors and exiting
with a special exit code (default: 2). Retryable errors include:

- stale plan errors
- lock acquisition errors

Otherwise, pvn-wrapper will exit with the original exit code of terraform apply. Note that this command will buffer
stderr to look at potential error messages, therefore, the order of stderr/stdout will be different
than terraform apply's.

pvn-wrapper terraform apply ...

To pass flags to terraform apply, use --

pvn-wrapper terraform apply -- --lock=false plan.tfplan

pvn-wrapper will always pass --no-color.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		applyArgs := []string{"apply"}
		applyArgs = append(applyArgs, args...)
		applyArgs = append(applyArgs,
			"-no-color",
		)
		execCmd := exec.CommandContext(ctx, terraformPath, applyArgs...)
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
			if strings.Contains(stderrString, "Saved plan is stale") || strings.Contains(stderrString, "Error acquiring the state lock") {
				os.Exit(applyFlags.retryableExitCode)
			}
			// other errors, try to match original exit code
			var exitErr *exec.ExitError
			if go_errors.As(execErr, &exitErr) {
				os.Exit(exitErr.ExitCode())
			} else {
				return errors.Wrap(execErr, "apply command failed unexpectedly")
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(applyCmd)

	applyCmd.Flags().IntVar(&applyFlags.retryableExitCode, "retryable-exit-code", 2, "Special exit code to use for retryable errors.")
}
