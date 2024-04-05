package fly

import (
	"encoding/json"
	go_errors "errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/pkg/errors"
	common_config_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/common_config"
	extensions_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/runtimes/extensions"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/encoding/protojson"
)

type flyStatusOutput struct {
	ID       string        `json:"ID"`
	AppURL   string        `json:"AppURL"`
	Name     string        `json:"Name"`
	Hostname string        `json:"Hostname"`
	Deployed bool          `json:"Deployed"`
	Machines []*flyMachine `json:"Machines"`
	Version  int           `json:"Version"`
}

type flyMachine struct {
	Name   string       `json:"name"`
	State  string       `json:"state"`
	Config flyAppConfig `json:"config"`
}

type flyAppConfig struct {
	Env      map[string]string `json:"env"`
	Metadata flyMetadata       `json:"metadata"`
}

type flyMetadata struct {
	FlyReleaseVersion string `json:"fly_release_version"`
}

var errServiceNotFound = errors.New("service not found")

func flyStatus(tomlFile string) (*flyStatusOutput, error) {

	describeCmd := exec.Command(
		flyPath,
		"status",
		"--config",
		tomlFile,
		"--json",
	)
	output, err := cmdutil.RunCmdOutput(describeCmd)
	if err != nil {
		if strings.Contains(err.Error(), "Could not find") {
			return nil, errServiceNotFound
		}
		return nil, err
	}
	var statusOutput flyStatusOutput
	if err := json.Unmarshal(output, &statusOutput); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal fly status output")
	}
	return &statusOutput, nil
}

func runFetch() (*extensions_pb.FetchOutput, error) {
	cfg, err := getServiceConfig()
	if err != nil {
		return nil, err
	}
	tomlFile, err := makeTomlFile(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tomlFile) }()
	status, err := flyStatus(tomlFile)
	if err != nil {
		if go_errors.Is(err, errServiceNotFound) {
			return &extensions_pb.FetchOutput{}, nil
		}
		return nil, err
	}

	versions := map[string]*extensions_pb.ExternalObjectVersion{}
	desiredFlyVersion := fmt.Sprintf("%d", status.Version)
	for _, machine := range status.Machines {
		var versionStr string
		if machine.Config.Env["PVN_SERVICE_ID"] == commonFlags.pvnServiceId {
			versionStr = machine.Config.Env["PVN_SERVICE_VERSION"]
		}
		if _, ok := versions[versionStr]; !ok {
			versions[versionStr] = &extensions_pb.ExternalObjectVersion{
				Version: versionStr,
				Active:  machine.Config.Metadata.FlyReleaseVersion == desiredFlyVersion,
			}
		}
		versions[versionStr].Replicas++
	}

	versionsList := maps.Values(versions)
	sort.Slice(versionsList, func(i, j int) bool {
		return versionsList[i].Version < versionsList[j].Version
	})

	cloudRunObj := &extensions_pb.ExternalObject{
		Name:       status.Name,
		ObjectType: "FlyApp",
		Versions:   versionsList,
		ExternalLinks: []*common_config_pb.ExternalLink{
			{
				Type: common_config_pb.ExternalLink_DETAIL,
				Name: "Fly Console",
				Url: fmt.Sprintf(
					"https://fly.io/apps/%[1]s",
					status.ID, // 1
				),
			},
			{
				Type: common_config_pb.ExternalLink_APP,
				Name: "App",
				Url:  status.AppURL,
			},
		},
	}
	if status.Deployed {
		// TODO(naphat) failure?
		cloudRunObj.Status = extensions_pb.ExternalObject_SUCCEEDED
	}

	return &extensions_pb.FetchOutput{
		Objects: []*extensions_pb.ExternalObject{
			cloudRunObj,
		},
	}, nil
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch current state of an Cloud Run service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fetchOutput, err := runFetch()
		if err != nil {
			return err
		}
		output, err := protojson.Marshal(fetchOutput)
		if err != nil {
			return errors.Wrap(err, "failed to marshal")
		}
		_, err = os.Stdout.Write(output)
		if err != nil {
			return errors.Wrap(err, "failed to write to stdout")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(fetchCmd)

	registerCommonFlags(fetchCmd)
}
