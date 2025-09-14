package aws

import (
	"context"
	"fmt"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// Client provides AWS integration functionality
type Client struct {
	logger       *zap.Logger
	config       *config.AWSConfig
	fleetManager *FleetManager
}

// LaunchRequest represents a request to launch AWS instances
type LaunchRequest struct {
	NodeIds              []string
	Partition            string
	NodeGroup            string
	InstanceRequirements *types.InstanceRequirements
	Job                  *types.SlurmJob
}

// LaunchResult represents the result of launching instances
type LaunchResult struct {
	Instances []types.InstanceInfo
	FleetId   string
}

// NewClient creates a new AWS client
func NewClient(logger *zap.Logger, awsConfig *config.AWSConfig) (*Client, error) {
	fleetManager, err := NewFleetManager(logger, awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create fleet manager: %w", err)
	}

	return &Client{
		logger:       logger,
		config:       awsConfig,
		fleetManager: fleetManager,
	}, nil
}

// LaunchInstances launches EC2 instances for the specified nodes
func (c *Client) LaunchInstances(ctx context.Context, req *LaunchRequest) (*LaunchResult, error) {
	c.logger.Info("Launching instances",
		zap.Strings("nodes", req.NodeIds),
		zap.String("partition", req.Partition),
		zap.String("node_group", req.NodeGroup))

	// Build fleet request from launch request
	fleetReq := &FleetRequest{
		NodeIds:              req.NodeIds,
		Partition:            req.Partition,
		NodeGroup:            req.NodeGroup,
		InstanceRequirements: req.InstanceRequirements,
		Job:                  req.Job,
		LaunchTemplate: LaunchTemplateConfig{
			Name:    fmt.Sprintf("%s-%s-template", req.Partition, req.NodeGroup),
			Version: "$Latest",
		},
		SubnetIds: []string{"subnet-default"}, // TODO: Get from configuration
		Tags: map[string]string{
			"Partition":  req.Partition,
			"NodeGroup":  req.NodeGroup,
			"ManagedBy":  "aws-slurm-burst",
			"JobID":      req.Job.JobID,
		},
	}

	// Launch fleet
	fleetResult, err := c.fleetManager.LaunchInstanceFleet(ctx, fleetReq)
	if err != nil {
		return nil, err
	}

	return &LaunchResult{
		Instances: fleetResult.Instances,
		FleetId:   fleetResult.FleetId,
	}, nil
}

// TerminateInstances terminates instances for the specified node names
func (c *Client) TerminateInstances(ctx context.Context, nodeNames []string) error {
	return c.fleetManager.TerminateInstances(ctx, nodeNames)
}
