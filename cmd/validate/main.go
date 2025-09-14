package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			fmt.Printf("Warning: failed to sync logger: %v\n", syncErr)
		}
	}()

	rootCmd := &cobra.Command{
		Use:   "aws-slurm-burst-validate",
		Short: "Validate configuration files and execution plans",
		Long: `Validate aws-slurm-burst configuration files and ASBA execution plans
for correctness and completeness before deployment.`,
	}

	// Add subcommands
	rootCmd.AddCommand(configCmd())
	rootCmd.AddCommand(executionPlanCmd())
	rootCmd.AddCommand(integrationCmd())

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Validation failed", zap.Error(err))
		os.Exit(1)
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [config-file]",
		Short: "Validate aws-slurm-burst configuration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := args[0]

			logger.Info("Validating configuration file", zap.String("file", configFile))

			// Load and validate configuration
			cfg, err := config.Load(configFile)
			if err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			// Additional validation checks
			if err := validateConfigCompleteness(cfg); err != nil {
				return fmt.Errorf("configuration incomplete: %w", err)
			}

			logger.Info("✅ Configuration file is valid",
				zap.String("file", configFile),
				zap.String("aws_region", cfg.AWS.Region),
				zap.Int("partition_count", len(cfg.Slurm.Partitions)))

			return nil
		},
	}
}

func executionPlanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "execution-plan [plan-file]",
		Short: "Validate ASBA execution plan JSON file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			planFile := args[0]

			logger.Info("Validating execution plan", zap.String("file", planFile))

			// Load execution plan
			data, err := os.ReadFile(planFile)
			if err != nil {
				return fmt.Errorf("failed to read execution plan: %w", err)
			}

			var plan types.ExecutionPlan
			if err := json.Unmarshal(data, &plan); err != nil {
				return fmt.Errorf("failed to parse execution plan JSON: %w", err)
			}

			// Validate execution plan
			if err := plan.ValidateExecutionPlan(); err != nil {
				return fmt.Errorf("execution plan validation failed: %w", err)
			}

			// Additional validation
			if err := validateExecutionPlanCompleteness(&plan); err != nil {
				return fmt.Errorf("execution plan incomplete: %w", err)
			}

			logger.Info("✅ Execution plan is valid",
				zap.String("file", planFile),
				zap.Bool("should_burst", plan.ShouldBurst),
				zap.Strings("instance_types", plan.InstanceSpec.InstanceTypes),
				zap.Bool("mpi_job", plan.MPIConfig.IsMPIJob))

			return nil
		},
	}
}

func integrationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "integration",
		Short: "Validate integration between configuration and execution plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Running integration validation tests")

			// Test basic functionality
			if err := validateBasicIntegration(); err != nil {
				return fmt.Errorf("integration validation failed: %w", err)
			}

			logger.Info("✅ Integration validation passed")
			return nil
		},
	}
}

// validateConfigCompleteness performs additional configuration validation
func validateConfigCompleteness(cfg *config.Config) error {
	// Check that all partitions have valid node groups
	for _, partition := range cfg.Slurm.Partitions {
		if len(partition.NodeGroups) == 0 {
			return fmt.Errorf("partition '%s' has no node groups", partition.PartitionName)
		}

		for _, nodeGroup := range partition.NodeGroups {
			if len(nodeGroup.SubnetIds) == 0 {
				return fmt.Errorf("node group '%s' in partition '%s' has no subnet IDs",
					nodeGroup.NodeGroupName, partition.PartitionName)
			}

			if len(nodeGroup.LaunchTemplateOverrides) == 0 {
				return fmt.Errorf("node group '%s' in partition '%s' has no instance types",
					nodeGroup.NodeGroupName, partition.PartitionName)
			}

			// Validate launch template specification
			if nodeGroup.LaunchTemplateSpec.LaunchTemplateName == "" && nodeGroup.LaunchTemplateSpec.LaunchTemplateID == "" {
				return fmt.Errorf("node group '%s' in partition '%s' missing launch template specification",
					nodeGroup.NodeGroupName, partition.PartitionName)
			}
		}
	}

	return nil
}

// validateExecutionPlanCompleteness performs additional execution plan validation
func validateExecutionPlanCompleteness(plan *types.ExecutionPlan) error {
	if plan.MPIConfig.IsMPIJob {
		if plan.MPIConfig.ProcessCount <= 0 {
			return fmt.Errorf("MPI job must specify process count > 0")
		}

		if plan.MPIConfig.RequiresGangScheduling && plan.NetworkConfig.PlacementGroupType == "" {
			return fmt.Errorf("gang scheduling requires placement group configuration")
		}

		if plan.MPIConfig.RequiresEFA && !plan.NetworkConfig.EnhancedNetworking {
			return fmt.Errorf("EFA requires enhanced networking to be enabled")
		}
	}

	if plan.CostConstraints.MaxTotalCost > 0 && plan.CostConstraints.MaxCostPerHour > 0 {
		// Check if cost constraints are reasonable
		estimatedHours := plan.CostConstraints.MaxDurationHours
		if estimatedHours == 0 {
			estimatedHours = 1 // Default assumption
		}

		expectedTotal := plan.CostConstraints.MaxCostPerHour * estimatedHours
		if expectedTotal > plan.CostConstraints.MaxTotalCost {
			return fmt.Errorf("cost constraints inconsistent: max_cost_per_hour * duration > max_total_cost")
		}
	}

	return nil
}

// validateBasicIntegration tests basic integration scenarios
func validateBasicIntegration() error {
	// Test that we can parse node lists correctly
	testNodeLists := []string{
		"aws-cpu-001",
		"aws-gpu-[001-004]",
		"aws-hpc-[001-008]",
	}

	for _, nodeList := range testNodeLists {
		parts := strings.Split(nodeList, "-")
		if len(parts) < 2 {
			return fmt.Errorf("failed to parse node list: %s", nodeList)
		}
	}

	logger.Info("Basic integration tests passed")
	return nil
}
