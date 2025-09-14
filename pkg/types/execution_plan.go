package types

import (
	"fmt"
	"time"
)

// ExecutionPlan represents a complete execution plan from ASBA
type ExecutionPlan struct {
	ShouldBurst         bool                 `json:"should_burst"`
	InstanceSpec        InstanceSpecification `json:"instance_specification"`
	MPIConfig           MPIConfiguration     `json:"mpi_configuration"`
	CostConstraints     CostConstraints      `json:"cost_constraints"`
	NetworkConfig       NetworkConfiguration `json:"network_configuration"`
	ExecutionMetadata   ExecutionMetadata    `json:"execution_metadata"`
}

// InstanceSpecification defines exactly what instances to launch
type InstanceSpecification struct {
	InstanceTypes       []string `json:"instance_types"`        // Exact types: ["hpc7a.2xlarge", "c6i.xlarge"]
	PurchasingOption    string   `json:"purchasing_option"`     // "spot", "on-demand", "mixed"
	MaxSpotPrice        float64  `json:"max_spot_price"`        // Per-hour limit
	SubnetIds           []string `json:"subnet_ids"`            // Target subnets
	LaunchTemplateName  string   `json:"launch_template_name"`  // AWS launch template
	LaunchTemplateID    string   `json:"launch_template_id"`    // Alternative to name
	SecurityGroupIds    []string `json:"security_group_ids"`    // Security groups
	IAMInstanceProfile  string   `json:"iam_instance_profile"`  // Instance role
	UserData            string   `json:"user_data,omitempty"`   // Bootstrap script
}

// MPIConfiguration defines MPI-specific requirements
type MPIConfiguration struct {
	IsMPIJob              bool   `json:"is_mpi_job"`
	ProcessCount          int    `json:"process_count"`
	RequiresGangScheduling bool   `json:"requires_gang_scheduling"`
	MPIImplementation     string `json:"mpi_implementation"`      // "openmpi", "intelmpi", "mpich"
	RequiresEFA           bool   `json:"requires_efa"`
	EFAGeneration         int    `json:"efa_generation"`          // 1 or 2
}

// CostConstraints defines cost and budget limits
type CostConstraints struct {
	MaxTotalCost        float64       `json:"max_total_cost"`         // Total budget for job
	MaxDurationHours    float64       `json:"max_duration_hours"`     // Expected job duration
	MaxCostPerHour      float64       `json:"max_cost_per_hour"`      // Hourly limit
	PreferSpot          bool          `json:"prefer_spot"`            // Cost optimization preference
	AllowMixedPricing   bool          `json:"allow_mixed_pricing"`    // Spot + on-demand
	CostAlertThreshold  float64       `json:"cost_alert_threshold"`   // Alert when exceeded
	AutoTerminateHours  float64       `json:"auto_terminate_hours"`   // Safety termination
}

// NetworkConfiguration defines networking requirements
type NetworkConfiguration struct {
	PlacementGroupType    string   `json:"placement_group_type"`    // "cluster", "partition", "spread"
	PlacementGroupName    string   `json:"placement_group_name"`    // Existing group or auto-generate
	EnhancedNetworking    bool     `json:"enhanced_networking"`     // SR-IOV
	AvailabilityZones     []string `json:"availability_zones"`      // Preferred AZs
	SingleAZRequired      bool     `json:"single_az_required"`      // Force single AZ
	NetworkLatencyClass   string   `json:"network_latency_class"`   // "ultra-low", "low", "standard"
}

// ExecutionMetadata contains additional execution context
type ExecutionMetadata struct {
	JobID               string            `json:"job_id"`
	UserID              string            `json:"user_id"`
	ProjectID           string            `json:"project_id"`
	Priority            string            `json:"priority"`             // "urgent", "normal", "low"
	AnalysisTimestamp   time.Time         `json:"analysis_timestamp"`
	ASBAVersion         string            `json:"asba_version"`
	DecisionFactors     []string          `json:"decision_factors"`     // Why these choices were made
	ExpectedPerformance PerformanceModel  `json:"expected_performance"`
	Tags                map[string]string `json:"tags"`                 // Additional AWS tags
}

// ExecutionResult represents the result of executing an execution plan
type ExecutionResult struct {
	Success              bool                 `json:"success"`
	LaunchedInstances    []InstanceInfo       `json:"launched_instances"`
	FailedInstances      []FailedInstance     `json:"failed_instances"`
	FleetID              string               `json:"fleet_id"`
	PlacementGroupName   string               `json:"placement_group_name"`
	TotalCostEstimate    float64              `json:"total_cost_estimate"`
	ExecutionStartTime   time.Time            `json:"execution_start_time"`
	ExecutionEndTime     time.Time            `json:"execution_end_time"`
	ExecutionDuration    Duration             `json:"execution_duration"`
	Errors               []ExecutionError     `json:"errors,omitempty"`
}

// FailedInstance represents an instance that failed to launch
type FailedInstance struct {
	NodeName      string `json:"node_name"`
	InstanceType  string `json:"instance_type"`
	SubnetID      string `json:"subnet_id"`
	ErrorCode     string `json:"error_code"`
	ErrorMessage  string `json:"error_message"`
}

// ExecutionError represents an error during execution
type ExecutionError struct {
	Type        string    `json:"type"`         // "aws_api", "slurm_api", "validation"
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Recoverable bool      `json:"recoverable"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// ValidateExecutionPlan validates that an execution plan is complete and valid
func (ep *ExecutionPlan) ValidateExecutionPlan() error {
	if !ep.ShouldBurst {
		return fmt.Errorf("execution plan indicates bursting should not occur")
	}

	if len(ep.InstanceSpec.InstanceTypes) == 0 {
		return fmt.Errorf("no instance types specified in execution plan")
	}

	if len(ep.InstanceSpec.SubnetIds) == 0 {
		return fmt.Errorf("no subnet IDs specified in execution plan")
	}

	if ep.InstanceSpec.PurchasingOption != "spot" && ep.InstanceSpec.PurchasingOption != "on-demand" && ep.InstanceSpec.PurchasingOption != "mixed" {
		return fmt.Errorf("invalid purchasing option: %s", ep.InstanceSpec.PurchasingOption)
	}

	if ep.MPIConfig.IsMPIJob && ep.NetworkConfig.PlacementGroupType == "" {
		return fmt.Errorf("MPI jobs require placement group configuration")
	}

	return nil
}

// GetRequiredInstanceCount calculates how many instances are needed
func (ep *ExecutionPlan) GetRequiredInstanceCount(nodeIds []string) int {
	return len(nodeIds)
}

// GetCostEstimate calculates estimated cost for the execution plan
func (ep *ExecutionPlan) GetCostEstimate(nodeCount int, durationHours float64) float64 {
	if len(ep.InstanceSpec.InstanceTypes) == 0 {
		return 0.0
	}

	// Use first instance type for estimation (ASBA would provide more accurate estimates)
	costPerHour := ep.CostConstraints.MaxCostPerHour
	if costPerHour == 0 {
		// Fallback estimation
		costPerHour = 0.10 // Default estimate
	}

	return costPerHour * float64(nodeCount) * durationHours
}