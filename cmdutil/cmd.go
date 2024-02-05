package cmdutil

import (
	go_errors "errors"
	"log"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func RunCmd(cmd *exec.Cmd) error {
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	log.Printf("Running command: %s", cmd.String())
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "Command failed:\n%s", cmd.String())
	}
	return nil
}

func RunCmdOutput(cmd *exec.Cmd) ([]byte, error) {
	log.Printf("Running command: %s", cmd.String())
	output, err := cmd.Output()
	log.Printf("Command output (%s):\n%s", cmd.String(), output)
	if err != nil {
		var exitErr *exec.ExitError
		if go_errors.As(err, &exitErr) {
			return nil, errors.Wrapf(err, "Command failed:\n%s\n%s", cmd.String(), string(exitErr.Stderr))
		}
		return nil, errors.Wrapf(err, "Command failed for unknown reasons:\n%s", cmd.String())
	}
	return output, nil
}
