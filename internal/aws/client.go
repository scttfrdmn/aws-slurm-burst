package aws

import (
	"context"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// Client provides AWS integration functionality
type Client struct {
	logger *zap.Logger
	config *config.AWSConfig
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
func NewClient(logger *zap.Logger, awsConfig *config.AWSConfig) *Client {
	return &Client{
		logger: logger,
		config: awsConfig,
	}
}

// LaunchInstances launches EC2 instances for the specified nodes
func (c *Client) LaunchInstances(ctx context.Context, req *LaunchRequest) (*LaunchResult, error) {
	c.logger.Info("Launching instances",
		zap.Strings("nodes", req.NodeIds),
		zap.String("partition", req.Partition),
		zap.String("node_group", req.NodeGroup))

	// TODO: Implement actual EC2 Fleet launching
	instances := make([]types.InstanceInfo, len(req.NodeIds))
	for i, nodeId := range req.NodeIds {
		instances[i] = types.InstanceInfo{
			NodeName:   nodeId,
			InstanceID: "i-" + nodeId,
			PrivateIP:  "10.0.0." + nodeId,
		}
	}

	return &LaunchResult{
		Instances: instances,
		FleetId:   "fleet-12345",
	}, nil
}