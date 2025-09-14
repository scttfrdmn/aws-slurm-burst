package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/aws"
	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/internal/slurm"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configFile    string
	executionPlan string
	dryRun        bool
	logger        *zap.Logger
)

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	rootCmd := &cobra.Command{
		Use:   "aws-slurm-burst-resume [node-list]",
		Short: "Execute AWS instance provisioning based on ASBA execution plan",
		Long: `Execute AWS instance provisioning for Slurm nodes based on a pre-analyzed
execution plan from aws-slurm-burst-advisor (ASBA).

This tool is a pure execution engine - all analysis, instance type selection,
and cost optimization decisions are made by ASBA.`,
		Args: cobra.ExactArgs(1),
		RunE: resumeNodes,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/slurm/aws-burst.yaml", "Configuration file path")
	rootCmd.Flags().StringVar(&executionPlan, "execution-plan", "", "Path to ASBA execution plan JSON file (optional)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without executing")

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

func resumeNodes(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine execution mode: ASBA-driven or standalone
	var plan *types.ExecutionPlan

	if executionPlan != "" {
		// ASBA Mode: Load execution plan from ASBA
		plan, err = loadExecutionPlan(executionPlan)
		if err != nil {
			return fmt.Errorf("failed to load execution plan: %w", err)
		}

		// Check if ASBA recommends bursting
		if !plan.ShouldBurst {
			logger.Info("ASBA recommends not bursting - job should run on-premises")
			return nil
		}

		logger.Info("Using ASBA execution plan", zap.String("plan_file", executionPlan))
	} else {
		// Standalone Mode: Generate default execution plan from configuration
		plan, err = generateDefaultExecutionPlan(cfg, args[0])
		if err != nil {
			return fmt.Errorf("failed to generate default execution plan: %w", err)
		}

		logger.Info("Using standalone mode with static configuration")
	}

	// Validate execution plan
	if err := plan.ValidateExecutionPlan(); err != nil {
		return fmt.Errorf("invalid execution plan: %w", err)
	}

	// Initialize AWS client
	awsClient, err := aws.NewClient(logger, &cfg.AWS, cfg)
	if err != nil {
		return fmt.Errorf("failed to create AWS client: %w", err)
	}

	// Initialize Slurm client
	slurmClient := slurm.NewClient(logger, &cfg.Slurm)

	// Parse node list
	nodeList := args[0]
	nodes, err := slurmClient.ParseNodeList(nodeList)
	if err != nil {
		return fmt.Errorf("failed to parse node list '%s': %w", nodeList, err)
	}

	logger.Info("Executing ASBA plan",
		zap.String("nodes", nodeList),
		zap.Int("node_count", len(nodes)),
		zap.String("plan_file", executionPlan),
		zap.Bool("mpi_job", plan.MPIConfig.IsMPIJob),
		zap.Strings("instance_types", plan.InstanceSpec.InstanceTypes),
		zap.Bool("requires_efa", plan.MPIConfig.RequiresEFA),
		zap.Bool("dry_run", dryRun))

	if dryRun {
		return executeDryRun(plan, nodes)
	}

	// Execute the plan
	result, err := executeProvisioningPlan(ctx, awsClient, slurmClient, plan, nodes)
	if err != nil {
		return fmt.Errorf("failed to execute provisioning plan: %w", err)
	}

	// Log execution results
	logger.Info("Provisioning completed",
		zap.Bool("success", result.Success),
		zap.Int("launched", len(result.LaunchedInstances)),
		zap.Int("failed", len(result.FailedInstances)),
		zap.String("fleet_id", result.FleetID),
		zap.Float64("estimated_cost", result.TotalCostEstimate))

	return nil
}

// loadExecutionPlan loads and parses the ASBA execution plan
func loadExecutionPlan(planPath string) (*types.ExecutionPlan, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read execution plan file: %w", err)
	}

	var plan types.ExecutionPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse execution plan JSON: %w", err)
	}

	logger.Debug("Loaded execution plan",
		zap.String("file", planPath),
		zap.Bool("should_burst", plan.ShouldBurst),
		zap.Strings("instance_types", plan.InstanceSpec.InstanceTypes),
		zap.Bool("mpi_job", plan.MPIConfig.IsMPIJob))

	return &plan, nil
}

// executeDryRun shows what would be executed without doing it
func executeDryRun(plan *types.ExecutionPlan, nodes []string) error {
	logger.Info("DRY RUN: Would execute the following plan:")
	logger.Info("  Instance Types", zap.Strings("types", plan.InstanceSpec.InstanceTypes))
	logger.Info("  Purchasing", zap.String("option", plan.InstanceSpec.PurchasingOption))
	logger.Info("  Max Spot Price", zap.Float64("price", plan.InstanceSpec.MaxSpotPrice))
	logger.Info("  Subnets", zap.Strings("subnet_ids", plan.InstanceSpec.SubnetIds))
	logger.Info("  Nodes", zap.Strings("node_list", nodes))

	if plan.MPIConfig.IsMPIJob {
		logger.Info("  MPI Configuration:")
		logger.Info("    Process Count", zap.Int("processes", plan.MPIConfig.ProcessCount))
		logger.Info("    Requires EFA", zap.Bool("efa", plan.MPIConfig.RequiresEFA))
		logger.Info("    Gang Scheduling", zap.Bool("gang", plan.MPIConfig.RequiresGangScheduling))
		logger.Info("    Placement Group", zap.String("type", plan.NetworkConfig.PlacementGroupType))
	}

	logger.Info("  Cost Constraints:")
	logger.Info("    Max Total Cost", zap.Float64("total", plan.CostConstraints.MaxTotalCost))
	logger.Info("    Max Cost/Hour", zap.Float64("hourly", plan.CostConstraints.MaxCostPerHour))

	estimatedCost := plan.GetCostEstimate(len(nodes), plan.CostConstraints.MaxDurationHours)
	logger.Info("  Estimated Total Cost", zap.Float64("cost", estimatedCost))

	return nil
}

// executeProvisioningPlan executes the ASBA plan by launching AWS instances
func executeProvisioningPlan(
	ctx context.Context,
	awsClient *aws.Client,
	slurmClient *slurm.Client,
	plan *types.ExecutionPlan,
	nodes []string,
) (*types.ExecutionResult, error) {

	result := &types.ExecutionResult{
		ExecutionStartTime: time.Now(),
	}

	// Build launch request from execution plan
	launchReq := &aws.LaunchRequest{
		NodeIds:   nodes,
		Partition: "aws", // TODO: Extract from node names
		NodeGroup: "cpu", // TODO: Extract from node names
		InstanceRequirements: &types.InstanceRequirements{
			// Direct execution of ASBA decisions
			InstanceFamilies:      plan.InstanceSpec.InstanceTypes,
			RequiresEFA:           plan.MPIConfig.RequiresEFA,
			PlacementGroupType:    plan.NetworkConfig.PlacementGroupType,
			MaxSpotPrice:          plan.InstanceSpec.MaxSpotPrice,
			PreferSpot:            plan.InstanceSpec.PurchasingOption == "spot",
			AllowMixedPricing:     plan.InstanceSpec.PurchasingOption == "mixed",
			EnhancedNetworking:    plan.NetworkConfig.EnhancedNetworking,
		},
		Job: &types.SlurmJob{
			JobID:        plan.ExecutionMetadata.JobID,
			IsMPIJob:     plan.MPIConfig.IsMPIJob,
			MPIProcesses: plan.MPIConfig.ProcessCount,
		},
	}

	// Launch instances
	launchResult, err := awsClient.LaunchInstances(ctx, launchReq)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, types.ExecutionError{
			Type:        "aws_api",
			Message:     err.Error(),
			Timestamp:   time.Now(),
			Recoverable: true,
		})
		return result, fmt.Errorf("failed to launch instances: %w", err)
	}

	result.LaunchedInstances = launchResult.Instances
	result.FleetID = launchResult.FleetId

	// Update Slurm with instance information
	if err := slurmClient.UpdateNodesWithInstanceInfo(ctx, launchResult.Instances); err != nil {
		logger.Error("Failed to update Slurm nodes", zap.Error(err))
		result.Errors = append(result.Errors, types.ExecutionError{
			Type:        "slurm_api",
			Message:     err.Error(),
			Timestamp:   time.Now(),
			Recoverable: true,
		})
		// Don't fail the operation - instances are launched
	}

	result.Success = true
	result.ExecutionEndTime = time.Now()
	result.ExecutionDuration = types.Duration(result.ExecutionEndTime.Sub(result.ExecutionStartTime))
	result.TotalCostEstimate = plan.GetCostEstimate(len(nodes), plan.CostConstraints.MaxDurationHours)

	return result, nil
}

// generateDefaultExecutionPlan creates a basic execution plan from static configuration (original plugin style)
func generateDefaultExecutionPlan(cfg *config.Config, nodeList string) (*types.ExecutionPlan, error) {
	// Parse node list to determine partition/nodegroup
	partition, nodeGroup, err := parseNodeListForPartition(nodeList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse node list: %w", err)
	}

	// Find matching node group configuration
	nodeGroupConfig := cfg.FindNodeGroup(partition, nodeGroup)
	if nodeGroupConfig == nil {
		return nil, fmt.Errorf("no configuration found for partition '%s' nodegroup '%s'", partition, nodeGroup)
	}

	// Build instance types from configuration
	var instanceTypes []string
	for _, override := range nodeGroupConfig.LaunchTemplateOverrides {
		instanceTypes = append(instanceTypes, override.InstanceType)
	}

	// Create default execution plan following original plugin patterns
	plan := &types.ExecutionPlan{
		ShouldBurst: true, // Always burst in standalone mode
		InstanceSpec: types.InstanceSpecification{
			InstanceTypes:      instanceTypes,
			PurchasingOption:   nodeGroupConfig.PurchasingOption,
			SubnetIds:          nodeGroupConfig.SubnetIds,
			LaunchTemplateName: nodeGroupConfig.LaunchTemplateSpec.LaunchTemplateName,
			LaunchTemplateID:   nodeGroupConfig.LaunchTemplateSpec.LaunchTemplateID,
			SecurityGroupIds:   nodeGroupConfig.SecurityGroupIds,
			IAMInstanceProfile: nodeGroupConfig.IAMInstanceProfile,
		},
		MPIConfig: types.MPIConfiguration{
			IsMPIJob:         false, // Default to non-MPI (ASBA would detect this)
			RequiresEFA:      false,
			RequiresGangScheduling: false,
		},
		CostConstraints: types.CostConstraints{
			PreferSpot:        nodeGroupConfig.PurchasingOption == "spot",
			AllowMixedPricing: false,
			MaxDurationHours:  24, // Default 24 hours
		},
		NetworkConfig: types.NetworkConfiguration{
			PlacementGroupType:  "", // No placement group by default
			EnhancedNetworking:  true,
			SingleAZRequired:    false,
		},
		ExecutionMetadata: types.ExecutionMetadata{
			JobID:             "standalone",
			Priority:          "normal",
			AnalysisTimestamp: time.Now(),
			DecisionFactors:   []string{"static_configuration"},
		},
	}

	logger.Debug("Generated default execution plan",
		zap.String("partition", partition),
		zap.String("node_group", nodeGroup),
		zap.Strings("instance_types", instanceTypes),
		zap.String("purchasing", nodeGroupConfig.PurchasingOption))

	return plan, nil
}

// parseNodeListForPartition extracts partition and nodegroup from node list like "aws-cpu-[001-004]"
func parseNodeListForPartition(nodeList string) (string, string, error) {
	// Simple parsing for node lists like "aws-cpu-001" or "aws-gpu-[001-004]"
	parts := strings.Split(nodeList, "-")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid node list format: %s", nodeList)
	}

	partition := parts[0]
	nodeGroup := parts[1]

	return partition, nodeGroup, nil
}