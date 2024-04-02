package fly

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
	common_config_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/common_config"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

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
