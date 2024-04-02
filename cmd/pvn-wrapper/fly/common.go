package fly

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/fly"
	service_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/service"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var commonFlags = struct {
	pvnCfgFile        string
	pvnServiceId      string
	pvnServiceVersion string
}{}

func registerCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&commonFlags.pvnCfgFile, "pvn-cfg-file", "", "Path to Prodvana Service Instance config file")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-cfg-file"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceId, "pvn-service-id", "", "Prodvana Service ID")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-id"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceVersion, "pvn-service-version", "", "Prodvana Service Version")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-version"))
}

func getServiceConfig() (*service_pb.CompiledServiceInstanceConfig, error) {
	bytes, err := os.ReadFile(commonFlags.pvnCfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read Prodvana config file")
	}
	var cfg service_pb.CompiledServiceInstanceConfig
	if err := proto.Unmarshal(bytes, &cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal Prodvana config file")
	}
	return &cfg, nil
}

func makeTomlFile(cfg *service_pb.CompiledServiceInstanceConfig) (string, error) {
	tempFile, err := os.CreateTemp("", "*.fly.toml")
	if err != nil {
		return "", errors.Wrap(err, "failed to make tempfile")
	}
	var tomlBytes []byte
	switch inner := cfg.GetFly().GetTomlOneof().(type) {
	case *fly.FlyConfig_Inlined:
		tomlBytes = []byte(inner.Inlined)
	default:
		return "", errors.Errorf("unrecognized toml type: %T", inner)
	}
	if _, err := tempFile.Write(tomlBytes); err != nil {
		return "", errors.Wrap(err, "failed to write to tempfile")
	}
	if err := tempFile.Close(); err != nil {
		return "", errors.Wrap(err, "failed to close tempfile")
	}
	log.Printf("fly toml is:\n%s", string(tomlBytes))
	return tempFile.Name(), nil
}
