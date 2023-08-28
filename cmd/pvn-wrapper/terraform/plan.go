package terraform

import (
	"context"
	go_errors "errors"
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

var planFlags = struct {
	planOut            string
	planExplanationOut string
}{}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "terraform plan wrapper",
	Long: `terraform plan wrapper.

Takes all the same input that terraform plan would, but handles creating plan files and explanation, then exiting with 0, 1, or 2.
Meant to be used as the fetch function of the terraform-runner runtime extension.

0 - No changes detected
1 - Unknown error
2 - Changes detected

pvn-wrapper terraform plan ...

To pass flags to terraform plan, use --

pvn-wrapper terraform plan -- --refresh=false

pvn-wrapper will always pass --detailed-exitcode, --out, and --no-color.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		planArgs := []string{"plan"}
		planArgs = append(planArgs, args...)
		planArgs = append(planArgs,
			"-detailed-exitcode",
			"-no-color",
			"-out",
			planFlags.planOut,
		)
		execCmd := exec.CommandContext(ctx, terraformPath, planArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
		var exitCode int
		err := execCmd.Run()
		if err != nil {
			var exitErr *exec.ExitError
			if go_errors.As(err, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				return errors.Wrap(err, "plan command failed unexpectedly")
			}
		}
		if exitCode == 0 || exitCode == 2 {
			showCommand := exec.CommandContext(ctx, terraformPath, "show", "-no-color", planFlags.planOut)
			planExplanation, err := os.Create(planFlags.planExplanationOut)
			if err != nil {
				return errors.Wrapf(err, "failed to open %s", planFlags.planExplanationOut)
			}
			defer func() { _ = planExplanation.Close() }()
			showCommand.Stderr = os.Stderr
			showCommand.Stdout = planExplanation
			err = showCommand.Run()
			if err != nil {
				return errors.Wrap(err, "show command failed")
			}
		}
		os.Exit(exitCode)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVar(&planFlags.planOut, "plan-out", "", "Plan file out location")
	cmdutil.Must(planCmd.MarkFlagRequired("plan-out"))
	planCmd.Flags().StringVar(&planFlags.planExplanationOut, "plan-explanation-out", "", "Plan file out location")
	cmdutil.Must(planCmd.MarkFlagRequired("plan-explanation-out"))
}
