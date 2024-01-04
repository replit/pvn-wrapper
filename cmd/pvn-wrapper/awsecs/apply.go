package awsecs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

const (
	serviceIdTagKey      = "pvn:id"
	serviceVersionTagKey = "pvn:version"
)

type tagPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type describeTaskDefinitionOutput struct {
	Tags []tagPair `json:"tags"`
}

func describeTaskDefinition(definition string) (*describeTaskDefinitionOutput, error) {
	describeCmd := exec.Command(
		awsPath,
		"ecs",
		"describe-task-definition",
		"--include=TAGS",
		"--task-definition",
		definition,
	)
	output, err := cmdutil.RunCmdOutput(describeCmd)
	if err != nil {
		return nil, err
	}
	var describeOutput describeTaskDefinitionOutput
	if err := json.Unmarshal(output, &describeOutput); err != nil {
		return nil, errors.Wrap(err, "failed to unmarsal describe-task-definition output")
	}
	return &describeOutput, nil
}

func tagsToMap(tags []tagPair) map[string]string {
	tagMap := make(map[string]string)
	for _, tag := range tags {
		tagMap[tag.Key] = tag.Value
	}
	return tagMap
}

type getResourcesOutput struct {
	ResourceTagMappingList []struct {
		ResourceARN string `json:"ResourceARN"`
		// this endpoint uses capital CamelCase so it cannot use the same struct as the other endpoints
		Tags []struct {
			Key   string `json:"Key"`
			Value string `json:"Value"`
		} `json:"Tags"`
	} `json:"ResourceTagMappingList"`
}

func getValidTaskDefinitionArns(pvnServiceId, pvnServiceVersion string) ([]string, error) {
	output, err := cmdutil.RunCmdOutput(exec.Command(
		awsPath,
		"resourcegroupstaggingapi",
		"get-resources",
		"--resource-type-filters", "ecs:task-definition",
		"--tag-filters",
		fmt.Sprintf("Key=%s,Values=%s", serviceIdTagKey, pvnServiceId),
		fmt.Sprintf("Key=%s,Values=%s", serviceVersionTagKey, pvnServiceVersion),
	))
	if err != nil {
		return nil, err
	}
	var outputParsed getResourcesOutput
	if err := json.Unmarshal(output, &outputParsed); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal get-resources output")
	}
	var validArns []string
	for _, resource := range outputParsed.ResourceTagMappingList {
		validArns = append(validArns, resource.ResourceARN)
	}
	return validArns, nil
}

type registerTaskDefinitionOutput struct {
	TaskDefinition struct {
		TaskDefinitionArn string `json:"taskDefinitionArn"`
	} `json:"taskDefinition"`
}

func registerTaskDefinitionIfNeeded(taskDefPath, pvnServiceId, pvnServiceVersion string, serviceOutput *describeServicesOutput) (string, error) {
	validArns, err := getValidTaskDefinitionArns(pvnServiceId, pvnServiceVersion)
	if err != nil {
		return "", err
	}
	if len(serviceOutput.Services) > 0 {
		service := serviceOutput.Services[0]
		for _, arn := range validArns {
			if arn == service.TaskDefinition {
				// prioritize returning the service's existing task arn
				log.Printf("Using existing task definition %s already present in service definition", arn)
				return arn, nil
			}
		}
	}
	if len(validArns) > 0 {
		log.Printf("Using existing task definition %s", validArns[0])
		return validArns[0], nil
	}
	log.Printf("Registering new task definition for %s:%s", pvnServiceId, pvnServiceVersion)
	taskDefPath, err = filepath.Abs(taskDefPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to make abs path")
	}
	registerCmd := exec.Command(
		awsPath,
		"ecs",
		"register-task-definition",
		"--cli-input-json",
		fmt.Sprintf("file://%s", taskDefPath),
	)
	output, err := cmdutil.RunCmdOutput(registerCmd)
	if err != nil {
		return "", err
	}
	var registerOutput registerTaskDefinitionOutput
	if err := json.Unmarshal(output, &registerOutput); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal register-task-definition output")
	}
	taskArn := registerOutput.TaskDefinition.TaskDefinitionArn
	if taskArn == "" {
		return "", errors.Errorf("got empty task definition arn. Register output: %s", string(output))
	}
	return taskArn, nil
}

type networkConfiguration struct {
	AwsvpcConfiguration *struct {
		Subnets        []string `json:"subnets"`
		SecurityGroups []string `json:"securityGroups"`
		AssignPublicIp string   `json:"assignPublicIp"`
	} `json:"awsvpcConfiguration"`
}

type describeServicesOutput struct {
	Services []struct {
		Status         string `json:"status"`
		TaskDefinition string `json:"taskDefinition"`
		Deployments    []struct {
			Status               string               `json:"status"`
			TaskDefinition       string               `json:"taskDefinition"`
			DesiredCount         int                  `json:"desiredCount"`
			PendingCount         int                  `json:"pendingCount"`
			RunningCount         int                  `json:"runningCount"`
			NetworkConfiguration networkConfiguration `json:"networkConfiguration"`
			RolloutState         string               `json:"rolloutState"`
			RolloutStateReason   string               `json:"rolloutStateReason"`
		} `json:"deployments"`
	} `json:"services"`
	Failures []struct {
		Reason string `json:"reason"`
	} `json:"failures"`
}

func describeService(clusterName, serviceName string) (*describeServicesOutput, error) {
	describeCmd := exec.Command(
		awsPath,
		"ecs",
		"describe-services",
		"--cluster",
		clusterName,
		"--services",
		serviceName,
	)
	output, err := cmdutil.RunCmdOutput(describeCmd)
	if err != nil {
		return nil, err
	}
	var describeOutput describeServicesOutput
	if err := json.Unmarshal(output, &describeOutput); err != nil {
		return nil, errors.Wrap(err, "failed to unmarsal describe-services output")
	}
	if len(describeOutput.Failures) > 0 {
		if describeOutput.Failures[0].Reason != "MISSING" {
			return nil, errors.Errorf("unexpected failure reason: %s", describeOutput.Failures[0].Reason)
		}
	} else {
		if len(describeOutput.Services) != 1 {
			return nil, errors.Errorf("unexpected number of services: %d", len(describeOutput.Services))
		}
	}
	return &describeOutput, nil
}

func patchTaskDefinition(taskDefPath, pvnServiceId, pvnServiceVersion string) (string, error) {
	taskDef, err := os.ReadFile(taskDefPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read task definition file")
	}
	var untypedDef map[string]interface{}
	if err := json.Unmarshal(taskDef, &untypedDef); err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal task definition file: %s", string(taskDef))
	}
	var tagsList []interface{}
	tags, hasTags := untypedDef["tags"]
	if hasTags {
		var ok bool
		tagsList, ok = tags.([]interface{})
		if !ok {
			return "", errors.Wrapf(err, "unexpected type for tags: %T", tags)
		}
	}
	tagsList = append(tagsList, map[string]string{
		"key":   serviceIdTagKey,
		"value": pvnServiceId,
	}, map[string]string{
		"key":   serviceVersionTagKey,
		"value": pvnServiceVersion,
	})
	untypedDef["tags"] = tagsList

	updatedTaskDef, err := json.Marshal(untypedDef)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal")
	}

	tempFile, err := os.CreateTemp("", "ecs-task-definition")
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

func patchServiceSpec(serviceSpecPath string, ecsServiceName, ecsCluster, taskArn string, forUpdate bool) (string, error) {
	serviceSpec, err := os.ReadFile(serviceSpecPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read service spec file")
	}
	var untypedDef map[string]interface{}
	if err := json.Unmarshal(serviceSpec, &untypedDef); err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal service spec file: %s", string(serviceSpec))
	}

	serviceName, hasServiceName := untypedDef["serviceName"]
	if hasServiceName {
		serviceNameString, ok := serviceName.(string)
		if !ok {
			return "", errors.Wrapf(err, "unexpected type for serviceName: %T", serviceName)
		}
		if serviceNameString != ecsServiceName {
			return "", errors.Errorf("serviceName in service spec file does not match ECS service name from Prodvana Service config. Got %s, want %s", serviceNameString, ecsServiceName)
		}
	}
	// delete the service field, as it's passed differently on update vs. create and handled on cli
	delete(untypedDef, "serviceName")
	delete(untypedDef, "service")
	cluster, hasCluster := untypedDef["cluster"]
	if hasCluster {
		clusterString, ok := cluster.(string)
		if !ok {
			return "", errors.Wrapf(err, "unexpected type for cluster: %T", cluster)
		}
		if clusterString != ecsCluster {
			return "", errors.Errorf("cluster in service spec file does not match ECS cluster name from Prodvana Runtime config. Got %s, want %s", clusterString, ecsCluster)
		}
	} else {
		untypedDef["cluster"] = ecsCluster
	}

	untypedDef["taskDefinition"] = taskArn

	if forUpdate {
		delete(untypedDef, "launchType")
	}

	updatedTaskDef, err := json.Marshal(untypedDef)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal")
	}

	tempFile, err := os.CreateTemp("", "ecs-task-definition")
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

func serviceMissing(output *describeServicesOutput) bool {
	if len(output.Failures) > 0 {
		return output.Failures[0].Reason == "MISSING"
	}
	return output.Services[0].Status == "INACTIVE"
}

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update an ECS service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		newTaskDefPath, err := patchTaskDefinition(commonFlags.taskDefinitionFile, commonFlags.pvnServiceId, commonFlags.pvnServiceVersion)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(newTaskDefPath) }()
		serviceOutput, err := describeService(commonFlags.ecsClusterName, commonFlags.ecsServiceName)
		if err != nil {
			return err
		}
		taskArn, err := registerTaskDefinitionIfNeeded(newTaskDefPath, commonFlags.pvnServiceId, commonFlags.pvnServiceVersion, serviceOutput)
		if err != nil {
			return err
		}
		commonArgs := []string{
			"--propagate-tags=TASK_DEFINITION",
			"--cluster", // must be set regardless of serviceSpec, in case updateTaskDefinitionOnly is set
			commonFlags.ecsClusterName,
		}
		if commonFlags.updateTaskDefinitionOnly {
			commonArgs = append(commonArgs,
				"--task-definition",
				taskArn,
			)
		} else {
			newServiceSpecPath, err := patchServiceSpec(
				commonFlags.serviceSpecFile,
				commonFlags.ecsServiceName,
				commonFlags.ecsClusterName,
				taskArn,
				!serviceMissing(serviceOutput),
			)
			if err != nil {
				return err
			}
			defer func() { _ = os.Remove(newServiceSpecPath) }()
			commonArgs = append(commonArgs,
				"--cli-input-json",
				fmt.Sprintf("file://%s", newServiceSpecPath),
			)
		}
		if serviceMissing(serviceOutput) {
			if commonFlags.updateTaskDefinitionOnly {
				return errors.Errorf("cannot update task definition only when ECS service does not exist. ECS service: %s", commonFlags.ecsServiceName)
			}
			log.Printf("Creating service %s on cluster %s with task ARN %s\n", commonFlags.ecsServiceName, commonFlags.ecsClusterName, taskArn)
			// create service
			createCmd := exec.Command(awsPath, append([]string{
				"ecs",
				"create-service",
				"--service-name",
				commonFlags.ecsServiceName,
			}, commonArgs...)...)
			err := cmdutil.RunCmd(createCmd)
			if err != nil {
				return err
			}
		} else {
			log.Printf("Updating service %s on cluster %s with task ARN %s\n", commonFlags.ecsServiceName, commonFlags.ecsClusterName, taskArn)
			// update service
			updateCmd := exec.Command(awsPath, append([]string{
				"ecs",
				"update-service",
				"--service",
				commonFlags.ecsServiceName, // must be set regardless of serviceSpec, in case updateTaskDefinitionOnly is set
			}, commonArgs...)...)
			err := cmdutil.RunCmd(updateCmd)
			if err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(applyCmd)

	registerCommonFlags(applyCmd)
}
