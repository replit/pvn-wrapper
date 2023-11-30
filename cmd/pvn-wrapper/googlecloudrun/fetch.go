package googlecloudrun

import (
	"encoding/json"
	go_errors "errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	common_config_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/common_config"
	extensions_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/runtimes/extensions"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	knative_serving "knative.dev/serving/pkg/apis/serving/v1"
	k8s_yaml "sigs.k8s.io/yaml"
)

var errServiceNotFound = fmt.Errorf("service not found")

func unmarshalKnativeService(bytes []byte) (*knative_serving.Service, error) {
	var serviceSpec knative_serving.Service
	err := k8s_yaml.Unmarshal(bytes, &serviceSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal yaml")
	}
	return &serviceSpec, nil
}

func describeService(service string) (*knative_serving.Service, error) {
	describeCmd := exec.Command(
		gcloudPath,
		"--project",
		commonFlags.gcpProject,
		"run",
		"services",
		"describe",
		"--region",
		commonFlags.region,
		service,
		"--format",
		"yaml",
	)
	output, err := cmdutil.RunCmdOutput(describeCmd)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot find service") {
			return nil, errServiceNotFound
		}
		return nil, err
	}
	return unmarshalKnativeService(output)
}

func getConditionStatus(conditions apis.Conditions, condType apis.ConditionType) corev1.ConditionStatus {
	for _, cond := range conditions {
		if cond.Type == condType {
			return cond.Status
		}
	}
	return corev1.ConditionUnknown
}

func runFetch() (*extensions_pb.FetchOutput, error) {
	specBytes, err := os.ReadFile(commonFlags.specFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read spec file")
	}
	inputSpec, err := unmarshalKnativeService(specBytes)
	if err != nil {
		return nil, err
	}
	name := inputSpec.GetObjectMeta().GetName()
	cloudRunObj := &extensions_pb.ExternalObject{
		Name:       name,
		ObjectType: "CloudRun",
		ExternalLinks: []*common_config_pb.ExternalLink{
			{
				Type: common_config_pb.ExternalLink_DETAIL,
				Name: "Cloud Run Console",
				Url: fmt.Sprintf(
					"https://console.cloud.google.com/run/detail/%[1]s/%[3]s/metrics?project=%[2]s",
					commonFlags.region,     // 1
					commonFlags.gcpProject, // 2
					name,                   // 3
				),
			},
		},
	}
	currentState, err := describeService(name)
	if err != nil {
		if go_errors.Is(err, errServiceNotFound) {
			return &extensions_pb.FetchOutput{
				Objects: []*extensions_pb.ExternalObject{
					cloudRunObj,
				},
			}, nil
		}
		return nil, err
	}
	version := &extensions_pb.ExternalObjectVersion{
		Active: true,
	}
	annotations := currentState.GetAnnotations()
	serviceId := annotations[idAnnotation]
	serviceVersion := annotations[versionAnnotation]
	if serviceId == commonFlags.pvnServiceId {
		version.Version = serviceVersion
	} // otherwise treat as unknown version
	cloudRunObj.Versions = []*extensions_pb.ExternalObjectVersion{version}
	if getConditionStatus(apis.Conditions(currentState.Status.Conditions), apis.ConditionReady) == corev1.ConditionTrue {
		cloudRunObj.Status = extensions_pb.ExternalObject_SUCCEEDED
	}
	// TODO(naphat) how to handle failures?
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
		if err := gcloudAuth(); err != nil {
			return err
		}
		fetchOutput, err := runFetch()
		if err != nil {
			return err
		}
		err = json.NewEncoder(os.Stdout).Encode(fetchOutput)
		if err != nil {
			return errors.Wrap(err, "failed to marshal")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(fetchCmd)

	registerCommonFlags(fetchCmd)
}
