package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	burstConfig "github.com/scttfrdmn/aws-slurm-burst/internal/config"
	burstTypes "github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// FleetManager handles EC2 Fleet operations for Slurm node provisioning
type FleetManager struct {
	logger    *zap.Logger
	ec2Client *ec2.Client
	region    string
}

// NewFleetManager creates a new fleet manager
func NewFleetManager(logger *zap.Logger, awsConfig *burstConfig.AWSConfig) (*FleetManager, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(awsConfig.Region),
		config.WithRetryMaxAttempts(awsConfig.RetryMaxAttempts),
		config.WithRetryMode(aws.RetryMode(awsConfig.RetryMode)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	if awsConfig.Profile != "" {
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithSharedConfigProfile(awsConfig.Profile),
			config.WithRegion(awsConfig.Region),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with profile: %w", err)
		}
	}

	return &FleetManager{
		logger:    logger,
		ec2Client: ec2.NewFromConfig(cfg),
		region:    awsConfig.Region,
	}, nil
}

// FleetRequest represents a request to launch EC2 instances
type FleetRequest struct {
	NodeIds              []string
	Partition            string
	NodeGroup            string
	InstanceRequirements *burstTypes.InstanceRequirements
	Job                  *burstTypes.SlurmJob
	LaunchTemplate       LaunchTemplateConfig
	SubnetIds            []string
	SecurityGroupIds     []string
	Tags                 map[string]string
}

// LaunchTemplateConfig represents launch template configuration
type LaunchTemplateConfig struct {
	Name    string
	ID      string
	Version string
}

// FleetResponse represents the result of launching instances
type FleetResponse struct {
	Instances []burstTypes.InstanceInfo
	FleetId   string
	Errors    []string
}

// LaunchInstanceFleet launches EC2 instances using EC2 Fleet API
func (f *FleetManager) LaunchInstanceFleet(ctx context.Context, req *FleetRequest) (*FleetResponse, error) {
	f.logger.Info("Launching EC2 Fleet",
		zap.Strings("node_ids", req.NodeIds),
		zap.String("partition", req.Partition),
		zap.String("node_group", req.NodeGroup),
		zap.Int("instance_count", len(req.NodeIds)),
		zap.Bool("mpi_job", req.Job.IsMPIJob),
		zap.Bool("requires_efa", req.InstanceRequirements.RequiresEFA))

	// Create placement group if needed for MPI jobs
	var placementGroupName string
	if req.InstanceRequirements.PlacementGroupType != "" && req.Job.IsMPIJob {
		pgName, err := f.ensurePlacementGroup(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create placement group: %w", err)
		}
		placementGroupName = pgName
	}

	// Build EC2 Fleet request
	fleetRequest, err := f.buildFleetRequest(req, placementGroupName)
	if err != nil {
		return nil, fmt.Errorf("failed to build fleet request: %w", err)
	}

	// Launch fleet
	fleetResult, err := f.ec2Client.CreateFleet(ctx, fleetRequest)
	if err != nil {
		return nil, fmt.Errorf("EC2 CreateFleet failed: %w", err)
	}

	f.logger.Info("EC2 Fleet created",
		zap.String("fleet_id", aws.ToString(fleetResult.FleetId)),
		zap.Int("instances_launched", len(fleetResult.Instances)))

	// Process results and get instance information
	response, err := f.processFleetResult(ctx, fleetResult, req.NodeIds)
	if err != nil {
		return nil, fmt.Errorf("failed to process fleet result: %w", err)
	}

	return response, nil
}

// buildFleetRequest creates the EC2 Fleet request structure
func (f *FleetManager) buildFleetRequest(req *FleetRequest, placementGroupName string) (*ec2.CreateFleetInput, error) {
	instanceCount := int32(len(req.NodeIds))

	// Build launch template configuration
	launchTemplateConfig := types.FleetLaunchTemplateConfigRequest{
		LaunchTemplateSpecification: &types.FleetLaunchTemplateSpecificationRequest{
			Version: aws.String(req.LaunchTemplate.Version),
		},
	}

	if req.LaunchTemplate.ID != "" {
		launchTemplateConfig.LaunchTemplateSpecification.LaunchTemplateId = aws.String(req.LaunchTemplate.ID)
	} else {
		launchTemplateConfig.LaunchTemplateSpecification.LaunchTemplateName = aws.String(req.LaunchTemplate.Name)
	}

	// Build launch template overrides
	overrides := f.buildLaunchTemplateOverrides(req, placementGroupName)
	launchTemplateConfig.Overrides = overrides

	// Determine purchasing option
	purchasingOption := types.DefaultTargetCapacityTypeOnDemand
	if req.InstanceRequirements.PreferSpot {
		purchasingOption = types.DefaultTargetCapacityTypeSpot
	}

	fleetRequest := &ec2.CreateFleetInput{
		LaunchTemplateConfigs: []types.FleetLaunchTemplateConfigRequest{
			launchTemplateConfig,
		},
		TargetCapacitySpecification: &types.TargetCapacitySpecificationRequest{
			TotalTargetCapacity:       aws.Int32(instanceCount),
			DefaultTargetCapacityType: purchasingOption,
		},
		Type: types.FleetTypeInstant, // Synchronous launching for gang scheduling
	}

	// Configure spot options if using spot instances
	if req.InstanceRequirements.PreferSpot {
		fleetRequest.SpotOptions = &types.SpotOptionsRequest{
			AllocationStrategy:          types.SpotAllocationStrategyLowestPrice,
			InstanceInterruptionBehavior: types.SpotInstanceInterruptionBehaviorTerminate,
		}

		if req.InstanceRequirements.MaxSpotPrice > 0 {
			fleetRequest.SpotOptions.MaxTotalPrice = aws.String(fmt.Sprintf("%.4f", req.InstanceRequirements.MaxSpotPrice))
		}
	}

	// Configure on-demand options
	if !req.InstanceRequirements.PreferSpot || req.InstanceRequirements.AllowMixedPricing {
		fleetRequest.OnDemandOptions = &types.OnDemandOptionsRequest{
			AllocationStrategy: types.FleetOnDemandAllocationStrategyLowestPrice,
		}
	}

	// Add fleet-level tags
	var fleetTags []types.TagSpecification
	if len(req.Tags) > 0 {
		var tagList []types.Tag
		for key, value := range req.Tags {
			tagList = append(tagList, types.Tag{
				Key:   aws.String(key),
				Value: aws.String(value),
			})
		}

		fleetTags = append(fleetTags, types.TagSpecification{
			ResourceType: types.ResourceTypeInstance,
			Tags:         tagList,
		})
	}
	fleetRequest.TagSpecifications = fleetTags

	return fleetRequest, nil
}

// buildLaunchTemplateOverrides creates instance type overrides based on requirements
func (f *FleetManager) buildLaunchTemplateOverrides(req *FleetRequest, placementGroupName string) []types.FleetLaunchTemplateOverridesRequest {
	var overrides []types.FleetLaunchTemplateOverridesRequest

	// Select optimal instance types based on requirements
	instanceTypes := f.selectInstanceTypes(req.InstanceRequirements)

	// Create overrides for each instance type in each subnet
	for _, instanceType := range instanceTypes {
		for _, subnetId := range req.SubnetIds {
			override := types.FleetLaunchTemplateOverridesRequest{
				InstanceType:     types.InstanceType(instanceType),
				SubnetId:         aws.String(subnetId),
				WeightedCapacity: aws.Float64(1.0),
			}

			// Add placement group if specified
			if placementGroupName != "" {
				override.Placement = &types.Placement{
					GroupName: aws.String(placementGroupName),
				}
			}

			overrides = append(overrides, override)
		}
	}

	f.logger.Debug("Built launch template overrides",
		zap.Int("override_count", len(overrides)),
		zap.Strings("instance_types", instanceTypes),
		zap.String("placement_group", placementGroupName))

	return overrides
}

// selectInstanceTypes chooses optimal instance types based on job requirements
func (f *FleetManager) selectInstanceTypes(req *burstTypes.InstanceRequirements) []string {
	// Use instance families from requirements if specified (ASBA mode)
	if len(req.InstanceFamilies) > 0 {
		// ASBA has already selected specific instance types, use them directly
		return req.InstanceFamilies
	}

	// Fallback instance selection for standalone mode
	var instanceTypes []string

	// Select instance types based on resource requirements
	if req.GPUs > 0 {
		// GPU workloads
		instanceTypes = append(instanceTypes, "p3.2xlarge", "g4dn.xlarge")
	} else if req.RequiresEFA {
		// EFA-capable instances for MPI
		instanceTypes = append(instanceTypes, "c5n.large", "c5n.xlarge", "c6i.large", "c6i.xlarge")
	} else {
		// General compute workloads
		instanceTypes = append(instanceTypes, "c5.large", "c5.xlarge", "m5.large", "m5.xlarge")
	}

	return instanceTypes
}

// getInstanceSizesForFamily returns appropriate instance sizes for a family
func (f *FleetManager) getInstanceSizesForFamily(family string, req *burstTypes.InstanceRequirements) []string {
	// Determine minimum instance size based on requirements
	var sizes []string

	// Memory requirements determine minimum size
	memoryGB := req.MinMemoryMB / 1024
	cpus := req.MinCPUs

	// Instance size mapping (simplified - in production would use AWS API)
	switch {
	case memoryGB <= 8 && cpus <= 2:
		sizes = []string{family + ".large"}
	case memoryGB <= 16 && cpus <= 4:
		sizes = []string{family + ".xlarge", family + ".large"}
	case memoryGB <= 32 && cpus <= 8:
		sizes = []string{family + ".2xlarge", family + ".xlarge"}
	case memoryGB <= 64 && cpus <= 16:
		sizes = []string{family + ".4xlarge", family + ".2xlarge"}
	default:
		sizes = []string{family + ".8xlarge", family + ".4xlarge"}
	}

	return sizes
}

// ensurePlacementGroup creates or ensures placement group exists
func (f *FleetManager) ensurePlacementGroup(ctx context.Context, req *FleetRequest) (string, error) {
	groupName := fmt.Sprintf("%s-%s-pg", req.Partition, req.NodeGroup)

	// Check if placement group already exists
	describeInput := &ec2.DescribePlacementGroupsInput{
		GroupNames: []string{groupName},
	}

	result, err := f.ec2Client.DescribePlacementGroups(ctx, describeInput)
	if err == nil && len(result.PlacementGroups) > 0 {
		f.logger.Debug("Using existing placement group", zap.String("name", groupName))
		return groupName, nil
	}

	// Create new placement group
	strategy := types.PlacementStrategy(req.InstanceRequirements.PlacementGroupType)
	createInput := &ec2.CreatePlacementGroupInput{
		GroupName: aws.String(groupName),
		Strategy:  strategy,
	}

	_, err = f.ec2Client.CreatePlacementGroup(ctx, createInput)
	if err != nil {
		return "", fmt.Errorf("failed to create placement group: %w", err)
	}

	f.logger.Info("Created placement group",
		zap.String("name", groupName),
		zap.String("strategy", string(strategy)))

	return groupName, nil
}

// processFleetResult processes the EC2 Fleet creation result and extracts instance information
func (f *FleetManager) processFleetResult(ctx context.Context, result *ec2.CreateFleetOutput, nodeIds []string) (*FleetResponse, error) {
	response := &FleetResponse{
		FleetId: aws.ToString(result.FleetId),
	}

	// Check for errors
	if len(result.Errors) > 0 {
		for _, fleetError := range result.Errors {
			response.Errors = append(response.Errors, aws.ToString(fleetError.ErrorMessage))
		}
	}

	// Process launched instances
	if len(result.Instances) == 0 {
		return response, fmt.Errorf("no instances were launched")
	}

	// Get detailed instance information
	var instanceIds []string
	for _, instance := range result.Instances {
		if len(instance.InstanceIds) > 0 {
			instanceIds = append(instanceIds, instance.InstanceIds[0])
		}
	}

	// Wait for instances to be running and get their details
	instanceInfos, err := f.waitForInstancesRunning(ctx, instanceIds, nodeIds)
	if err != nil {
		return response, fmt.Errorf("failed to get instance details: %w", err)
	}

	response.Instances = instanceInfos

	f.logger.Info("Fleet launch completed",
		zap.String("fleet_id", response.FleetId),
		zap.Int("requested", len(nodeIds)),
		zap.Int("launched", len(response.Instances)),
		zap.Int("errors", len(response.Errors)))

	return response, nil
}

// waitForInstancesRunning waits for instances to reach running state and retrieves their information
func (f *FleetManager) waitForInstancesRunning(ctx context.Context, instanceIds, nodeIds []string) ([]burstTypes.InstanceInfo, error) {
	// Wait for instances to be running (with timeout)
	waiter := ec2.NewInstanceRunningWaiter(f.ec2Client)
	waitInput := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	}

	if err := waiter.Wait(ctx, waitInput, 5*time.Minute); err != nil {
		return nil, fmt.Errorf("instances failed to reach running state: %w", err)
	}

	// Get detailed instance information
	result, err := f.ec2Client.DescribeInstances(ctx, waitInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instances []burstTypes.InstanceInfo
	instanceIndex := 0

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			if instanceIndex >= len(nodeIds) {
				break
			}

			// Map instance to node name
			nodeName := nodeIds[instanceIndex]
			instanceInfo := burstTypes.InstanceInfo{
				NodeName:   nodeName,
				InstanceID: aws.ToString(instance.InstanceId),
				PrivateIP:  aws.ToString(instance.PrivateIpAddress),
				State:      string(instance.State.Name),
				LaunchTime: instance.LaunchTime.Format(time.RFC3339),
			}

			if instance.PublicIpAddress != nil {
				instanceInfo.PublicIP = aws.ToString(instance.PublicIpAddress)
			}

			instances = append(instances, instanceInfo)
			instanceIndex++

			f.logger.Debug("Instance mapped to node",
				zap.String("node_name", nodeName),
				zap.String("instance_id", instanceInfo.InstanceID),
				zap.String("private_ip", instanceInfo.PrivateIP),
				zap.String("state", instanceInfo.State))
		}
	}

	// Tag instances with node names for identification
	if err := f.tagInstancesWithNodeNames(ctx, instances); err != nil {
		f.logger.Warn("Failed to tag instances", zap.Error(err))
		// Continue anyway - tagging is not critical for functionality
	}

	return instances, nil
}

// tagInstancesWithNodeNames tags EC2 instances with their corresponding Slurm node names
func (f *FleetManager) tagInstancesWithNodeNames(ctx context.Context, instances []burstTypes.InstanceInfo) error {
	for _, instance := range instances {
		tags := []types.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(instance.NodeName),
			},
			{
				Key:   aws.String("SlurmNode"),
				Value: aws.String(instance.NodeName),
			},
			{
				Key:   aws.String("ManagedBy"),
				Value: aws.String("aws-slurm-burst"),
			},
		}

		_, err := f.ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
			Resources: []string{instance.InstanceID},
			Tags:      tags,
		})

		if err != nil {
			return fmt.Errorf("failed to tag instance %s: %w", instance.InstanceID, err)
		}
	}

	return nil
}

// TerminateInstances terminates EC2 instances for the specified node names
func (f *FleetManager) TerminateInstances(ctx context.Context, nodeNames []string) error {
	if len(nodeNames) == 0 {
		return nil
	}

	f.logger.Info("Terminating instances", zap.Strings("nodes", nodeNames))

	// Find instances by node name tags
	instanceIds, err := f.findInstancesByNodeNames(ctx, nodeNames)
	if err != nil {
		return fmt.Errorf("failed to find instances for nodes: %w", err)
	}

	if len(instanceIds) == 0 {
		f.logger.Warn("No instances found for termination", zap.Strings("nodes", nodeNames))
		return nil
	}

	// Terminate instances
	_, err = f.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	})

	if err != nil {
		return fmt.Errorf("failed to terminate instances: %w", err)
	}

	f.logger.Info("Instances termination initiated",
		zap.Strings("nodes", nodeNames),
		zap.Strings("instance_ids", instanceIds))

	return nil
}

// findInstancesByNodeNames finds EC2 instance IDs by their Slurm node name tags
func (f *FleetManager) findInstancesByNodeNames(ctx context.Context, nodeNames []string) ([]string, error) {
	// Build filter for instance names
	filters := []types.Filter{
		{
			Name:   aws.String("tag:Name"),
			Values: nodeNames,
		},
		{
			Name:   aws.String("instance-state-name"),
			Values: []string{"pending", "running", "shutting-down", "stopping", "stopped"},
		},
	}

	result, err := f.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: filters,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var instanceIds []string
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceIds = append(instanceIds, aws.ToString(instance.InstanceId))
		}
	}

	return instanceIds, nil
}

// GetInstancePricing retrieves current pricing for instance types
func (f *FleetManager) GetInstancePricing(ctx context.Context, instanceTypes []string) (map[string]float64, error) {
	// Note: Real AWS Pricing API integration planned for Phase 3
	// Using realistic mock pricing for now
	pricing := make(map[string]float64)

	for _, instanceType := range instanceTypes {
		// Mock pricing based on instance size (more realistic progression)
		switch {
		case strings.Contains(instanceType, "2xlarge"):
			pricing[instanceType] = 0.384 // $0.384/hour
		case strings.Contains(instanceType, "xlarge"):
			pricing[instanceType] = 0.192 // $0.192/hour
		case strings.Contains(instanceType, "large"):
			pricing[instanceType] = 0.096 // $0.096/hour
		default:
			pricing[instanceType] = 0.048 // Default small instance
		}
	}

	return pricing, nil
}