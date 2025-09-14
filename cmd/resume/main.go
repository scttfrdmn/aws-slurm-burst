package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/absa"
	"github.com/scttfrdmn/aws-slurm-burst/internal/aws"
	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/internal/scheduler"
	"github.com/scttfrdmn/aws-slurm-burst/internal/slurm"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configFile  string
	jobMetadata string
	dryRun      bool
	logger      *zap.Logger
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
		Short: "Resume AWS instances for Slurm nodes with MPI and EFA support",
		Long: `Resume AWS instances for Slurm nodes with intelligent MPI detection,
EFA configuration, and dynamic instance selection based on job requirements.

Integrates with aws-slurm-burst-advisor (ABSA) for cost-optimized decision making.`,
		Args: cobra.ExactArgs(1),
		RunE: resumeNodes,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/slurm/aws-burst.yaml", "Configuration file path")
	rootCmd.Flags().StringVar(&jobMetadata, "job-metadata", "", "Path to ABSA job metadata JSON file")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without actually doing it")

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

	// Initialize components
	slurmClient := slurm.NewClient(logger, &cfg.Slurm)
	awsClient := aws.NewClient(logger, &cfg.AWS)
	mpiScheduler := scheduler.NewMPIScheduler(logger)
	absaClient := absa.NewABSAClient(logger, cfg.ABSA.Command, cfg.ABSA.ConfigPath)

	// Validate ABSA availability (optional)
	if cfg.ABSA.Enabled {
		if err := absaClient.ValidateABSAAvailability(ctx); err != nil {
			logger.Warn("ABSA not available, proceeding without cost optimization", zap.Error(err))
			cfg.ABSA.Enabled = false
		}
	}

	// Parse node list
	nodeList := args[0]
	nodes, err := slurmClient.ParseNodeList(nodeList)
	if err != nil {
		return fmt.Errorf("failed to parse node list '%s': %w", nodeList, err)
	}

	logger.Info("Resume request received",
		zap.String("nodes", nodeList),
		zap.Int("node_count", len(nodes)),
		zap.Bool("absa_enabled", cfg.ABSA.Enabled),
		zap.Bool("dry_run", dryRun))

	// Group nodes by partition and node group
	nodeGroups := groupNodesByPartition(nodes)

	for partition, nodesByGroup := range nodeGroups {
		for nodeGroup, nodeIds := range nodesByGroup {
			if err := resumeNodeGroup(ctx, cfg, slurmClient, awsClient, mpiScheduler, absaClient, partition, nodeGroup, nodeIds); err != nil {
				logger.Error("Failed to resume node group",
					zap.String("partition", partition),
					zap.String("node_group", nodeGroup),
					zap.Strings("nodes", nodeIds),
					zap.Error(err))
				// Continue with other node groups
			}
		}
	}

	return nil
}

func resumeNodeGroup(
	ctx context.Context,
	cfg *config.Config,
	slurmClient *slurm.Client,
	awsClient *aws.Client,
	mpiScheduler *scheduler.MPIScheduler,
	absaClient *absa.ABSAClient,
	partition, nodeGroup string,
	nodeIds []string,
) error {
	logger.Info("Resuming node group",
		zap.String("partition", partition),
		zap.String("node_group", nodeGroup),
		zap.Strings("nodes", nodeIds),
		zap.Int("count", len(nodeIds)))

	// Get job information for intelligent scheduling
	job, err := slurmClient.GetJobForNodes(ctx, nodeIds)
	if err != nil {
		logger.Warn("Could not retrieve job information, using defaults", zap.Error(err))
		job = createDefaultJob(partition, nodeGroup, nodeIds)
	}

	// Analyze job for MPI characteristics
	if err := mpiScheduler.AnalyzeJob(ctx, job); err != nil {
		return fmt.Errorf("MPI analysis failed: %w", err)
	}

	// Determine instance requirements
	instanceReq := mpiScheduler.DetermineInstanceRequirements(job)

	// Enrich with ABSA data if available
	if cfg.ABSA.Enabled {
		if err := absaClient.EnrichJobWithABSAData(ctx, job, instanceReq); err != nil {
			logger.Warn("Failed to enrich with ABSA data", zap.Error(err))
		}

		// Get ABSA instance recommendations
		if recommendedTypes, err := absaClient.GetRecommendedInstanceTypes(ctx, job); err == nil && len(recommendedTypes) > 0 {
			logger.Info("Using ABSA instance recommendations", zap.Strings("types", recommendedTypes))
			instanceReq.InstanceFamilies = recommendedTypes
		}
	}

	if dryRun {
		logger.Info("DRY RUN: Would launch instances",
			zap.String("partition", partition),
			zap.String("node_group", nodeGroup),
			zap.Int("node_count", len(nodeIds)),
			zap.Bool("mpi_job", job.IsMPIJob),
			zap.Bool("requires_efa", instanceReq.RequiresEFA),
			zap.Bool("hpc_optimized", instanceReq.HPCOptimized),
			zap.Strings("instance_families", instanceReq.InstanceFamilies),
			zap.String("placement_group", instanceReq.PlacementGroupType))
		return nil
	}

	// Launch instances
	launchResult, err := awsClient.LaunchInstances(ctx, &aws.LaunchRequest{
		NodeIds:              nodeIds,
		Partition:            partition,
		NodeGroup:            nodeGroup,
		InstanceRequirements: instanceReq,
		Job:                  job,
	})
	if err != nil {
		return fmt.Errorf("failed to launch instances: %w", err)
	}

	logger.Info("Instances launched successfully",
		zap.String("partition", partition),
		zap.String("node_group", nodeGroup),
		zap.Int("requested", len(nodeIds)),
		zap.Int("launched", len(launchResult.Instances)),
		zap.String("fleet_id", launchResult.FleetId))

	// Update Slurm node information
	if err := slurmClient.UpdateNodesWithInstanceInfo(ctx, launchResult.Instances); err != nil {
		logger.Error("Failed to update Slurm nodes", zap.Error(err))
		// Don't fail the operation, instances are launched
	}

	return nil
}

func groupNodesByPartition(nodes []string) map[string]map[string][]string {
	// Parse nodes like "aws-gpu-001", "aws-cpu-002" etc.
	result := make(map[string]map[string][]string)

	for _, node := range nodes {
		parts := strings.Split(node, "-")
		if len(parts) < 3 {
			logger.Warn("Invalid node name format", zap.String("node", node))
			continue
		}

		partition := parts[0] // "aws"
		nodeGroup := parts[1] // "gpu" or "cpu"
		nodeId := parts[2]    // "001"

		if result[partition] == nil {
			result[partition] = make(map[string][]string)
		}
		result[partition][nodeGroup] = append(result[partition][nodeGroup], nodeId)
	}

	return result
}

func createDefaultJob(partition, nodeGroup string, nodeIds []string) *types.SlurmJob {
	return &types.SlurmJob{
		JobID:     "unknown",
		Name:      "resume-job",
		Partition: partition,
		NodeList:  nodeIds,
		Resources: types.ResourceSpec{
			Nodes:       len(nodeIds),
			CPUsPerNode: 4,    // Default
			MemoryMB:    8192, // Default
		},
		Constraints: types.JobConstraints{},
		IsMPIJob:    false, // Will be analyzed by MPI scheduler
		MPITopology: types.TopologyAny,
	}
}
