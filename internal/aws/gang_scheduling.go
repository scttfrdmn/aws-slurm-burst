package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

// GangScheduler handles atomic provisioning for MPI jobs
type GangScheduler struct {
	logger       *zap.Logger
	ec2Client    *ec2.Client
	fleetManager *FleetManager
}

// NewGangScheduler creates a new gang scheduler
func NewGangScheduler(logger *zap.Logger, ec2Client *ec2.Client, fleetManager *FleetManager) *GangScheduler {
	return &GangScheduler{
		logger:       logger,
		ec2Client:    ec2Client,
		fleetManager: fleetManager,
	}
}

// AtomicProvision ensures all instances launch successfully or none do (gang scheduling)
func (g *GangScheduler) AtomicProvision(ctx context.Context, req *FleetRequest) (*FleetResponse, error) {
	if !req.Job.IsMPIJob || !req.InstanceRequirements.RequiresEFA {
		// Non-MPI jobs don't need gang scheduling
		return g.fleetManager.LaunchInstanceFleet(ctx, req)
	}

	g.logger.Info("Starting gang scheduling for MPI job",
		zap.String("job_id", req.Job.JobID),
		zap.Int("required_nodes", len(req.NodeIds)),
		zap.Bool("requires_efa", req.InstanceRequirements.RequiresEFA))

	// Pre-flight capacity check
	if err := g.checkCapacityAvailability(ctx, req); err != nil {
		return nil, fmt.Errorf("pre-flight capacity check failed: %w", err)
	}

	// Attempt atomic launch
	response, err := g.attemptAtomicLaunch(ctx, req)
	if err != nil {
		// If atomic launch fails, cleanup any partial instances
		g.cleanupPartialLaunch(ctx, response)
		return nil, fmt.Errorf("gang scheduling failed: %w", err)
	}

	// Verify all instances are running before returning success
	if err := g.verifyAllInstancesRunning(ctx, response); err != nil {
		// Cleanup on verification failure
		g.cleanupPartialLaunch(ctx, response)
		return nil, fmt.Errorf("instance verification failed: %w", err)
	}

	g.logger.Info("Gang scheduling completed successfully",
		zap.String("job_id", req.Job.JobID),
		zap.Int("launched_instances", len(response.Instances)),
		zap.String("fleet_id", response.FleetId))

	return response, nil
}

// checkCapacityAvailability performs pre-flight checks for instance availability
func (g *GangScheduler) checkCapacityAvailability(ctx context.Context, req *FleetRequest) error {
	// Get instance type availability in target subnets
	instanceTypes := g.fleetManager.selectInstanceTypes(req.InstanceRequirements)

	for _, subnet := range req.SubnetIds {
		for _, instanceType := range instanceTypes {
			available, err := g.checkInstanceTypeAvailability(ctx, instanceType, subnet)
			if err != nil {
				g.logger.Warn("Failed to check capacity",
					zap.String("instance_type", instanceType),
					zap.String("subnet", subnet),
					zap.Error(err))
				continue
			}

			if available {
				g.logger.Debug("Capacity confirmed",
					zap.String("instance_type", instanceType),
					zap.String("subnet", subnet))
				return nil // At least one instance type has capacity
			}
		}
	}

	return fmt.Errorf("insufficient capacity in target subnets for required instance types")
}

// checkInstanceTypeAvailability checks if instance type is available in subnet
func (g *GangScheduler) checkInstanceTypeAvailability(ctx context.Context, instanceType, subnetID string) (bool, error) {
	// Use EC2 describe-instance-type-offerings to check availability
	input := &ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: types.LocationTypeAvailabilityZone,
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-type"),
				Values: []string{instanceType},
			},
		},
	}

	result, err := g.ec2Client.DescribeInstanceTypeOfferings(ctx, input)
	if err != nil {
		return false, err
	}

	// If any offerings exist, assume capacity is available
	// Note: This is a simplified check - real capacity checking would require Spot Fleet dry-run
	return len(result.InstanceTypeOfferings) > 0, nil
}

// attemptAtomicLaunch tries to launch all instances atomically
func (g *GangScheduler) attemptAtomicLaunch(ctx context.Context, req *FleetRequest) (*FleetResponse, error) {
	// For MPI gang scheduling, use instant fleet type to ensure synchronous launch
	g.logger.Info("Attempting atomic instance launch",
		zap.Int("required_count", len(req.NodeIds)))

	return g.fleetManager.LaunchInstanceFleet(ctx, req)
}

// verifyAllInstancesRunning ensures all instances reach running state
func (g *GangScheduler) verifyAllInstancesRunning(ctx context.Context, response *FleetResponse) error {
	if len(response.Instances) == 0 {
		return fmt.Errorf("no instances were launched")
	}

	var instanceIds []string
	for _, instance := range response.Instances {
		instanceIds = append(instanceIds, instance.InstanceID)
	}

	// Wait for all instances to be running with timeout
	g.logger.Info("Verifying all instances are running",
		zap.Strings("instance_ids", instanceIds))

	waiter := ec2.NewInstanceRunningWaiter(g.ec2Client)
	waitInput := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	}

	// MPI jobs need all instances running quickly
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	if err := waiter.Wait(ctx, waitInput, 10*time.Minute); err != nil {
		return fmt.Errorf("instances failed to reach running state within timeout: %w", err)
	}

	g.logger.Info("All instances verified running", zap.Int("count", len(instanceIds)))
	return nil
}

// cleanupPartialLaunch terminates any instances that were launched during a failed gang schedule
func (g *GangScheduler) cleanupPartialLaunch(ctx context.Context, response *FleetResponse) {
	if response == nil || len(response.Instances) == 0 {
		return
	}

	g.logger.Warn("Cleaning up partial launch due to gang scheduling failure",
		zap.Int("instances_to_cleanup", len(response.Instances)))

	var instanceIds []string
	for _, instance := range response.Instances {
		instanceIds = append(instanceIds, instance.InstanceID)
	}

	// Terminate partial instances
	_, err := g.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	})

	if err != nil {
		g.logger.Error("Failed to cleanup partial instances",
			zap.Strings("instance_ids", instanceIds),
			zap.Error(err))
	} else {
		g.logger.Info("Partial instances cleaned up successfully",
			zap.Strings("instance_ids", instanceIds))
	}
}