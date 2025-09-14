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

// SpotManager handles spot instance lifecycle and interruption management
type SpotManager struct {
	logger    *zap.Logger
	ec2Client *ec2.Client
	region    string
}

// NewSpotManager creates a new spot instance manager
func NewSpotManager(logger *zap.Logger, ec2Client *ec2.Client, region string) *SpotManager {
	return &SpotManager{
		logger:    logger,
		ec2Client: ec2Client,
		region:    region,
	}
}

// SpotPricingStrategy represents spot instance pricing strategy
type SpotPricingStrategy struct {
	MaxSpotPrice        float64 `json:"max_spot_price"`
	AllowMixedPricing   bool    `json:"allow_mixed_pricing"`
	SpotAllocationRatio float64 `json:"spot_allocation_ratio"` // 0.0-1.0, percentage of instances to launch as spot
	OnDemandFallback    bool    `json:"on_demand_fallback"`    // Fallback to on-demand if spot unavailable
}

// OptimizeSpotStrategy determines optimal spot instance strategy for a job
func (s *SpotManager) OptimizeSpotStrategy(ctx context.Context, req *FleetRequest) (*SpotPricingStrategy, error) {
	strategy := &SpotPricingStrategy{
		MaxSpotPrice:      req.InstanceRequirements.MaxSpotPrice,
		AllowMixedPricing: req.InstanceRequirements.AllowMixedPricing,
		OnDemandFallback:  true, // Default to safe fallback
	}

	// MPI jobs need special spot handling
	if req.Job.IsMPIJob {
		strategy = s.optimizeForMPIJob(req, strategy)
	} else {
		strategy = s.optimizeForRegularJob(req, strategy)
	}

	s.logger.Info("Optimized spot strategy",
		zap.String("job_id", req.Job.JobID),
		zap.Bool("mpi_job", req.Job.IsMPIJob),
		zap.Float64("max_spot_price", strategy.MaxSpotPrice),
		zap.Float64("spot_ratio", strategy.SpotAllocationRatio),
		zap.Bool("mixed_pricing", strategy.AllowMixedPricing))

	return strategy, nil
}

// optimizeForMPIJob creates spot strategy optimized for MPI workloads
func (s *SpotManager) optimizeForMPIJob(req *FleetRequest, baseStrategy *SpotPricingStrategy) *SpotPricingStrategy {
	// MPI jobs are sensitive to node failures
	if req.InstanceRequirements.RequiresEFA {
		// High-performance MPI: prefer reliability over cost
		baseStrategy.SpotAllocationRatio = 0.3 // Only 30% spot instances
		baseStrategy.AllowMixedPricing = true
		baseStrategy.OnDemandFallback = true

		s.logger.Debug("EFA MPI job: preferring reliability over cost savings")
	} else {
		// Regular MPI: balanced approach
		baseStrategy.SpotAllocationRatio = 0.7 // 70% spot instances
		baseStrategy.AllowMixedPricing = true
	}

	return baseStrategy
}

// optimizeForRegularJob creates spot strategy for embarrassingly parallel jobs
func (s *SpotManager) optimizeForRegularJob(req *FleetRequest, baseStrategy *SpotPricingStrategy) *SpotPricingStrategy {
	// Regular jobs can handle spot interruptions better
	if req.InstanceRequirements.PreferSpot {
		baseStrategy.SpotAllocationRatio = 0.9 // 90% spot instances
		baseStrategy.AllowMixedPricing = true
	} else {
		baseStrategy.SpotAllocationRatio = 0.5 // Balanced approach
	}

	return baseStrategy
}

// GetCurrentSpotPrices retrieves current spot prices for instance types
func (s *SpotManager) GetCurrentSpotPrices(ctx context.Context, instanceTypes []string, availabilityZones []string) (map[string]float64, error) {
	// Convert string slice to InstanceType slice
	var instanceTypeEnums []types.InstanceType
	for _, instanceType := range instanceTypes {
		instanceTypeEnums = append(instanceTypeEnums, types.InstanceType(instanceType))
	}

	input := &ec2.DescribeSpotPriceHistoryInput{
		InstanceTypes:       instanceTypeEnums,
		ProductDescriptions: []string{"Linux/UNIX"},
		MaxResults:          aws.Int32(100),
	}

	result, err := s.ec2Client.DescribeSpotPriceHistory(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get spot prices: %w", err)
	}

	// Get most recent price for each instance type
	priceMap := make(map[string]float64)
	for _, price := range result.SpotPriceHistory {
		instanceType := string(price.InstanceType)
		spotPrice := 0.0
		if price.SpotPrice != nil {
			fmt.Sscanf(aws.ToString(price.SpotPrice), "%f", &spotPrice)
		}
		priceMap[instanceType] = spotPrice
	}

	s.logger.Debug("Retrieved current spot prices",
		zap.Int("instance_types", len(instanceTypes)),
		zap.Int("prices_found", len(priceMap)))

	return priceMap, nil
}

// ValidateSpotPricing checks if spot pricing is within acceptable limits
func (s *SpotManager) ValidateSpotPricing(ctx context.Context, req *FleetRequest, strategy *SpotPricingStrategy) error {
	if !req.InstanceRequirements.PreferSpot {
		return nil // No validation needed for on-demand
	}

	instanceTypes := s.getInstanceTypesFromRequest(req)
	currentPrices, err := s.GetCurrentSpotPrices(ctx, instanceTypes, []string{})
	if err != nil {
		s.logger.Warn("Failed to get current spot prices for validation", zap.Error(err))
		return nil // Don't fail the job, just proceed without validation
	}

	// Check if any instance types are within price limits
	withinBudget := false
	for instanceType, currentPrice := range currentPrices {
		if strategy.MaxSpotPrice == 0 || currentPrice <= strategy.MaxSpotPrice {
			withinBudget = true
			s.logger.Debug("Instance type within spot budget",
				zap.String("instance_type", instanceType),
				zap.Float64("current_price", currentPrice),
				zap.Float64("max_price", strategy.MaxSpotPrice))
		} else {
			s.logger.Warn("Instance type exceeds spot budget",
				zap.String("instance_type", instanceType),
				zap.Float64("current_price", currentPrice),
				zap.Float64("max_price", strategy.MaxSpotPrice))
		}
	}

	if !withinBudget && !strategy.OnDemandFallback {
		return fmt.Errorf("no instance types within spot price budget and on-demand fallback disabled")
	}

	return nil
}

// getInstanceTypesFromRequest extracts instance types from fleet request
func (s *SpotManager) getInstanceTypesFromRequest(req *FleetRequest) []string {
	if len(req.InstanceRequirements.InstanceFamilies) > 0 {
		return req.InstanceRequirements.InstanceFamilies
	}

	// Fallback to basic instance types
	return []string{"c5.large", "c5.xlarge", "m5.large", "m5.xlarge"}
}

// MonitorSpotInterruptions monitors running spot instances for interruption warnings
func (s *SpotManager) MonitorSpotInterruptions(ctx context.Context, instanceIds []string) (<-chan SpotInterruptionEvent, error) {
	eventChan := make(chan SpotInterruptionEvent, len(instanceIds))

	// Start monitoring goroutine
	go func() {
		defer close(eventChan)

		ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkForInterruptions(ctx, instanceIds, eventChan)
			}
		}
	}()

	return eventChan, nil
}

// SpotInterruptionEvent represents a spot instance interruption event
type SpotInterruptionEvent struct {
	InstanceID         string    `json:"instance_id"`
	NodeName           string    `json:"node_name"`
	InterruptionTime   time.Time `json:"interruption_time"`
	InterruptionReason string    `json:"interruption_reason"`
	Action             string    `json:"action"` // "terminate", "hibernate", "stop"
}

// checkForInterruptions checks instances for spot interruption warnings
func (s *SpotManager) checkForInterruptions(ctx context.Context, instanceIds []string, eventChan chan<- SpotInterruptionEvent) {
	// Check instance metadata for spot interruption warnings
	// Note: In real implementation, this would check EC2 instance metadata service
	// For now, this is a framework for spot interruption monitoring

	input := &ec2.DescribeInstancesInput{
		InstanceIds: instanceIds,
	}

	result, err := s.ec2Client.DescribeInstances(ctx, input)
	if err != nil {
		s.logger.Error("Failed to check instance status", zap.Error(err))
		return
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			// Check if instance is marked for spot interruption
			if instance.State.Name == types.InstanceStateNameShuttingDown {
				// Get instance name from tags
				nodeName := s.getNodeNameFromInstance(instance)

				event := SpotInterruptionEvent{
					InstanceID:         aws.ToString(instance.InstanceId),
					NodeName:           nodeName,
					InterruptionTime:   time.Now(),
					InterruptionReason: "spot_interruption",
					Action:             "terminate",
				}

				select {
				case eventChan <- event:
					s.logger.Warn("Spot interruption detected",
						zap.String("instance_id", event.InstanceID),
						zap.String("node_name", event.NodeName))
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// getNodeNameFromInstance extracts node name from instance tags
func (s *SpotManager) getNodeNameFromInstance(instance types.Instance) string {
	for _, tag := range instance.Tags {
		if aws.ToString(tag.Key) == "Name" || aws.ToString(tag.Key) == "SlurmNode" {
			return aws.ToString(tag.Value)
		}
	}
	return aws.ToString(instance.InstanceId) // Fallback to instance ID
}
