package awsecs

import (
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

var commonFlags = struct {
	taskDefinitionFile       string
	serviceSpecFile          string
	ecsClusterName           string
	ecsServiceName           string
	pvnServiceId             string
	pvnServiceVersion        string
	updateTaskDefinitionOnly bool
}{}

func registerCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&commonFlags.taskDefinitionFile, "task-definition-file", "", "Path to ECS task definition file")
	cmdutil.Must(cmd.MarkFlagRequired("task-definition-file"))
	cmd.Flags().StringVar(&commonFlags.serviceSpecFile, "service-spec-file", "", "Path to ECS service spec file")
	cmdutil.Must(cmd.MarkFlagRequired("service-spec-file"))
	cmd.Flags().StringVar(&commonFlags.ecsServiceName, "ecs-service-name", "", "Name of ECS service")
	cmdutil.Must(cmd.MarkFlagRequired("ecs-service-name"))
	cmd.Flags().StringVar(&commonFlags.ecsClusterName, "ecs-cluster-name", "", "Name of ECS cluster")
	cmdutil.Must(cmd.MarkFlagRequired("ecs-cluster-name"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceId, "pvn-service-id", "", "Prodvana Service ID")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-id"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceVersion, "pvn-service-version", "", "Prodvana Service Version")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-version"))
	cmd.Flags().BoolVar(&commonFlags.updateTaskDefinitionOnly, "update-task-definition-only", false, "Update task definition only")
	cmdutil.Must(cmd.MarkFlagRequired("update-task-definition-only"))
}
