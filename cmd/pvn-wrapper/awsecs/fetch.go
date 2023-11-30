package awsecs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/pkg/errors"
	common_config_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/common_config"
	extensions_pb "github.com/prodvana/prodvana-public/go/prodvana-sdk/proto/prodvana/runtimes/extensions"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func runFetch() (*extensions_pb.FetchOutput, error) {
	serviceOutput, err := describeService(commonFlags.ecsClusterName, commonFlags.ecsServiceName)
	if err != nil {
		return nil, err
	}
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		return nil, errors.Errorf("AWS_DEFAULT_REGION not set")
	}
	ecsServiceObj := &extensions_pb.ExternalObject{
		Name:       commonFlags.ecsServiceName,
		ObjectType: "ECS",
		ExternalLinks: []*common_config_pb.ExternalLink{
			{
				Type: common_config_pb.ExternalLink_DETAIL,
				Name: "ECS Console",
				Url: fmt.Sprintf(
					"https://%[1]s.console.aws.amazon.com/ecs/v2/clusters/%[2]s/services/%[3]s?region=%[1]s",
					region,
					commonFlags.ecsClusterName,
					commonFlags.ecsServiceName,
				),
			},
		},
	}
	if serviceMissing(serviceOutput) {
		ecsServiceObj.Status = extensions_pb.ExternalObject_PENDING
		return &extensions_pb.FetchOutput{
			Objects: []*extensions_pb.ExternalObject{
				ecsServiceObj,
			},
		}, nil
	}
	versionChan := make(chan *extensions_pb.ExternalObjectVersion)
	errg := errgroup.Group{}
	for _, depl := range serviceOutput.Services[0].Deployments {
		depl := depl
		errg.Go(func() error {
			def, err := describeTaskDefinition(depl.TaskDefinition)
			if err != nil {
				return err
			}
			tags := tagsToMap(def.Tags)
			version := &extensions_pb.ExternalObjectVersion{
				Replicas:          int32(depl.PendingCount) + int32(depl.RunningCount),
				Active:            depl.Status == "PRIMARY",
				AvailableReplicas: int32(depl.RunningCount),
				TargetReplicas:    int32(depl.DesiredCount),
				// TODO(naphat) today we use the service version string to detect drift.
				// It is currently not possible to change ECS-service-level settings like desired count
				// without also creating a new version string, so this works.
			}
			if version.Replicas == 0 {
				// skip, this deployment is no longer active and has no replicas left
				return nil
			}
			if tags[serviceIdTagKey] == commonFlags.pvnServiceId {
				// if the service ID doesn't match, we leave the version unset, essentially treating it as unknown
				version.Version = tags[serviceVersionTagKey]
			}
			versionChan <- version
			return nil
		})
	}
	var versions []*extensions_pb.ExternalObjectVersion
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ver := range versionChan {
			versions = append(versions, ver)
		}
	}()
	err = errg.Wait()
	close(versionChan)
	if err != nil {
		return nil, err
	}
	<-done
	ecsServiceObj.Versions = versions
	foundCount := 0
	for _, depl := range serviceOutput.Services[0].Deployments {
		if depl.Status == "PRIMARY" {
			switch depl.RolloutState {
			case "COMPLETED":
				ecsServiceObj.Status = extensions_pb.ExternalObject_SUCCEEDED
			case "FAILED":
				ecsServiceObj.Status = extensions_pb.ExternalObject_FAILED
			}
			foundCount++
		}
	}
	if foundCount != 1 {
		log.Printf("Found multiple PRIMARY deployments for service %s, marking it as PENDING", commonFlags.ecsServiceName)
		ecsServiceObj.Status = extensions_pb.ExternalObject_PENDING
	}
	return &extensions_pb.FetchOutput{
		Objects: []*extensions_pb.ExternalObject{
			ecsServiceObj,
		},
	}, nil
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Fetch current state of an ECS service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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
