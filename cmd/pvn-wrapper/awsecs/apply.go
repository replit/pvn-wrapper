package awsecs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

const (
	serviceIdTagKey      = "pvn:id"
	serviceVersionTagKey = "pvn:version"
)

var applyFlags = struct {
	taskDefinitionFile string
	ecsClusterName     string
	ecsServiceName     string
	pvnServiceId       string
	pvnServiceVersion  string
	desiredCount       int
	subnets            []string
	securityGroups     []string
	assignPublicIp     bool
}{}

type getResourcesOutput struct {
	ResourceTagMappingList []struct {
		ResourceARN string `json:"ResourceARN"`
		Tags        []struct {
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

type describeServicesOutput struct {
	Services []struct {
		Status         string `json:"status"`
		TaskDefinition string `json:"taskDefinition"`
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

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Create or update an ECS service",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		newTaskDefPath, err := patchTaskDefinition(applyFlags.taskDefinitionFile, applyFlags.pvnServiceId, applyFlags.pvnServiceVersion)
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(newTaskDefPath) }()
		serviceOutput, err := describeService(applyFlags.ecsClusterName, applyFlags.ecsServiceName)
		if err != nil {
			return err
		}
		taskArn, err := registerTaskDefinitionIfNeeded(newTaskDefPath, applyFlags.pvnServiceId, applyFlags.pvnServiceVersion, serviceOutput)
		if err != nil {
			return err
		}
		if len(serviceOutput.Failures) > 0 {
			if serviceOutput.Failures[0].Reason != "MISSING" {
				return errors.Errorf("unexpected failure reason: %s", serviceOutput.Failures[0].Reason)
			}
		} else {
			if len(serviceOutput.Services) != 1 {
				return errors.Errorf("unexpected number of services: %d", len(serviceOutput.Services))
			}
		}
		networkConfigurations := []string{
			fmt.Sprintf("subnets=[%s]", strings.Join(applyFlags.subnets, ",")),
			fmt.Sprintf("securityGroups=[%s]", strings.Join(applyFlags.securityGroups, ",")),
		}
		if applyFlags.assignPublicIp {
			networkConfigurations = append(networkConfigurations, "assignPublicIp=ENABLED")
		}
		commonArgs := []string{
			"--task-definition",
			taskArn,
			"--desired-count",
			fmt.Sprintf("%d", applyFlags.desiredCount),
			"--network-configuration",
			fmt.Sprintf("awsvpcConfiguration={%s}", strings.Join(networkConfigurations, ",")),
			"--propagate-tags=TASK_DEFINITION",
			"--cluster",
			applyFlags.ecsClusterName,
		}
		if serviceOutput.Services[0].Status == "INACTIVE" || serviceOutput.Services[0].Status == "MISSING" {
			log.Printf("Creating service %s on cluster %s with task ARN %s\n", applyFlags.ecsServiceName, applyFlags.ecsClusterName, taskArn)
			// create service
			createCmd := exec.Command(awsPath, append([]string{
				"ecs",
				"create-service",
				"--service-name",
				applyFlags.ecsServiceName,
				"--launch-type=FARGATE",
			}, commonArgs...)...)
			err := cmdutil.RunCmd(createCmd)
			if err != nil {
				return err
			}
		} else {
			log.Printf("Updating service %s on cluster %s with task ARN %s\n", applyFlags.ecsServiceName, applyFlags.ecsClusterName, taskArn)
			// update service
			updateCmd := exec.Command(awsPath, append([]string{
				"ecs",
				"update-service",
				"--service",
				applyFlags.ecsServiceName,
			}, commonArgs...)...)
			err := cmdutil.RunCmd(updateCmd)
			if err != nil {
				return err
			}
		}
		waitCmd := exec.Command(awsPath, "ecs", "wait", "services-stable", "--services", applyFlags.ecsServiceName, "--cluster", applyFlags.ecsClusterName)
		return cmdutil.RunCmd(waitCmd)
	},
}

func init() {
	RootCmd.AddCommand(applyCmd)

	applyCmd.Flags().StringVar(&applyFlags.taskDefinitionFile, "task-definition-file", "", "Path to task definition file")
	cmdutil.Must(applyCmd.MarkFlagRequired("task-definition-file"))
	applyCmd.Flags().StringVar(&applyFlags.ecsServiceName, "ecs-service-name", "", "Name of ECS service")
	cmdutil.Must(applyCmd.MarkFlagRequired("ecs-service-name"))
	applyCmd.Flags().StringVar(&applyFlags.ecsClusterName, "ecs-cluster-name", "", "Name of ECS cluster")
	cmdutil.Must(applyCmd.MarkFlagRequired("ecs-cluster-name"))
	applyCmd.Flags().StringVar(&applyFlags.pvnServiceId, "pvn-service-id", "", "Prodvana Service ID")
	cmdutil.Must(applyCmd.MarkFlagRequired("pvn-service-id"))
	applyCmd.Flags().StringVar(&applyFlags.pvnServiceVersion, "pvn-service-version", "", "Prodvana Service Version")
	cmdutil.Must(applyCmd.MarkFlagRequired("pvn-service-version"))
	applyCmd.Flags().IntVar(&applyFlags.desiredCount, "desired-count", 0, "Number of instances desired")
	cmdutil.Must(applyCmd.MarkFlagRequired("desired-count"))
	applyCmd.Flags().StringSliceVar(&applyFlags.subnets, "subnets", nil, "Subnets to use")
	cmdutil.Must(applyCmd.MarkFlagRequired("subnets"))
	applyCmd.Flags().StringSliceVar(&applyFlags.securityGroups, "security-groups", nil, "Security groups to use")
	cmdutil.Must(applyCmd.MarkFlagRequired("security-groups"))
	applyCmd.Flags().BoolVar(&applyFlags.assignPublicIp, "assign-public-ip", false, "Assign public IP")
	cmdutil.Must(applyCmd.MarkFlagRequired("assign-public-ip"))
}
