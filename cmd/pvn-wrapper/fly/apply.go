package fly

import (
	go_errors "errors"
	"os"
	"os/exec"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	common_config_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/common_config"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

type flyCfg struct {
	App string `toml:"app"`
}

func createIfNeeded(tomlFile string) error {
	bytes, err := os.ReadFile(tomlFile)
	if err != nil {
		return errors.Wrap(err, "failed to read toml file")
	}
	var cfg flyCfg
	if err := toml.Unmarshal(bytes, &cfg); err != nil {
		return errors.Wrap(err, "failed to unmarshal toml file")
	}
	_, err = flyStatus(tomlFile)
	if err == nil {
		return nil
	}
	if !go_errors.Is(err, errServiceNotFound) {
		return err
	}
	createCmd := exec.Command(
		flyPath,
		"app",
		"create",
		cfg.App,
	)
	return cmdutil.RunCmd(createCmd)
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update a Fly service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getServiceConfig()
		if err != nil {
			return err
		}
		tomlFile, err := makeTomlFile(cfg)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(tomlFile) }()
		if err := createIfNeeded(tomlFile); err != nil {
			return err
		}
		envToInject := cfg.Env
		envToInject["PVN_SERVICE_ID"] = &common_config_pb.EnvValue{
			ValueOneof: &common_config_pb.EnvValue_Value{Value: commonFlags.pvnServiceId},
		}
		envToInject["PVN_SERVICE_VERSION"] = &common_config_pb.EnvValue{
			ValueOneof: &common_config_pb.EnvValue_Value{Value: commonFlags.pvnServiceVersion},
		}
		flyArgs := []string{
			"deploy",
			"--config",
			tomlFile,
		}
		for k, v := range envToInject {
			switch v.GetValueOneof().(type) {
			case *common_config_pb.EnvValue_Value:
				flyArgs = append(flyArgs, "--env", k+"="+v.GetValue())
			default:
				return errors.Errorf("unrecognized env value type for %s: %T", k, v)
			}
		}
		// TODO(naphat) disable waits
		createCmd := exec.Command(
			flyPath,
			flyArgs...,
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
