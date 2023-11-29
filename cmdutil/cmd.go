package cmdutil

import (
	go_errors "errors"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

func RunCmd(cmd *exec.Cmd) error {
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Command failed:\n%s", strings.Join(cmd.Args, " "))
	}
	return nil
}

func RunCmdOutput(cmd *exec.Cmd) ([]byte, error) {
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if go_errors.As(err, &exitErr) {
			return nil, errors.Wrapf(err, "Command failed:\n%s\n%s", strings.Join(cmd.Args, " "), string(exitErr.Stderr))
		}
		return nil, errors.Wrapf(err, "Command failed for unknown reasons:\n%s", strings.Join(cmd.Args, " "))
	}
	return output, nil
}
