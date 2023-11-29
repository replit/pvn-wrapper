package awsecs

import (
	"github.com/prodvana/pvn-wrapper/cmdutil"
	"github.com/spf13/cobra"
)

var commonFlags = struct {
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

func registerCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&commonFlags.taskDefinitionFile, "task-definition-file", "", "Path to task definition file")
	cmdutil.Must(cmd.MarkFlagRequired("task-definition-file"))
	cmd.Flags().StringVar(&commonFlags.ecsServiceName, "ecs-service-name", "", "Name of ECS service")
	cmdutil.Must(cmd.MarkFlagRequired("ecs-service-name"))
	cmd.Flags().StringVar(&commonFlags.ecsClusterName, "ecs-cluster-name", "", "Name of ECS cluster")
	cmdutil.Must(cmd.MarkFlagRequired("ecs-cluster-name"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceId, "pvn-service-id", "", "Prodvana Service ID")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-id"))
	cmd.Flags().StringVar(&commonFlags.pvnServiceVersion, "pvn-service-version", "", "Prodvana Service Version")
	cmdutil.Must(cmd.MarkFlagRequired("pvn-service-version"))
	cmd.Flags().IntVar(&commonFlags.desiredCount, "desired-count", 0, "Number of instances desired")
	cmdutil.Must(cmd.MarkFlagRequired("desired-count"))
	cmd.Flags().StringSliceVar(&commonFlags.subnets, "subnets", nil, "Subnets to use")
	cmdutil.Must(cmd.MarkFlagRequired("subnets"))
	cmd.Flags().StringSliceVar(&commonFlags.securityGroups, "security-groups", nil, "Security groups to use")
	cmdutil.Must(cmd.MarkFlagRequired("security-groups"))
	cmd.Flags().BoolVar(&commonFlags.assignPublicIp, "assign-public-ip", false, "Assign public IP")
	cmdutil.Must(cmd.MarkFlagRequired("assign-public-ip"))
}
