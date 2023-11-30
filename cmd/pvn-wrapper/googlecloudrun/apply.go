package googlecloudrun

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

const (
	idAnnotation      = "prodvana.io/id"
	versionAnnotation = "prodvana.io/version"
)

func patchSpecFile(specFilePath, pvnServiceId, pvnServiceVersion string) (string, error) {
	taskDef, err := os.ReadFile(specFilePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read task definition file")
	}
	var untypedDef map[string]interface{}
	if err := yaml.Unmarshal(taskDef, &untypedDef); err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal task definition file: %s", string(taskDef))
	}
	metadata, err := cmdutil.GetOrCreateUntypedMapFromStringMap(untypedDef, "metadata")
	if err != nil {
		return "", err
	}
	annotations, err := cmdutil.GetOrCreateUntypedMap(metadata, "annotations")
	if err != nil {
		return "", err
	}
	annotations[idAnnotation] = pvnServiceId
	annotations[versionAnnotation] = pvnServiceVersion

	updatedTaskDef, err := yaml.Marshal(untypedDef)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal")
	}

	tempFile, err := os.CreateTemp("", "google-cloud-run-spec")
	if err != nil {
		return "", errors.Wrap(err, "failed to make tempfile")
	}

	if _, err := tempFile.Write(updatedTaskDef); err != nil {
		return "", errors.Wrap(err, "failed to write to tempfile")
	}
	if err := tempFile.Close(); err != nil {
		return "", errors.Wrap(err, "failed to close tempfile")
	}

	return tempFile.Name(), nil
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update a Google Cloud Run service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := gcloudAuth(); err != nil {
			return err
		}
		newSpecPath, err := patchSpecFile(commonFlags.specFile, commonFlags.pvnServiceId, commonFlags.pvnServiceVersion)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(newSpecPath) }()
		createCmd := exec.Command(
			gcloudPath,
			"--project",
			commonFlags.gcpProject,
			"run",
			"services",
			"replace",
			"--region",
			commonFlags.region,
			newSpecPath,
		)
		err = cmdutil.RunCmd(createCmd)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(applyCmd)

	registerCommonFlags(applyCmd)
}
