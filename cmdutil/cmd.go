package cmdutil

import (
	go_errors "errors"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

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
