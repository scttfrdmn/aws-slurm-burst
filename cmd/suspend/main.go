package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/aws"
	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/internal/slurm"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configFile string
	dryRun     bool
	logger     *zap.Logger
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
		Use:   "aws-slurm-burst-suspend [node-list]",
		Short: "Suspend AWS instances for Slurm nodes",
		Long:  `Suspend (terminate) AWS instances associated with Slurm nodes to save costs.`,
		Args:  cobra.ExactArgs(1),
		RunE:  suspendNodes,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/slurm/aws-burst.yaml", "Configuration file path")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without actually doing it")

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

func suspendNodes(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize components
	slurmClient := slurm.NewClient(logger, &cfg.Slurm)
	awsClient := aws.NewClient(logger, &cfg.AWS)

	// Parse node list
	nodeList := args[0]
	nodes, err := slurmClient.ParseNodeList(nodeList)
	if err != nil {
		return fmt.Errorf("failed to parse node list '%s': %w", nodeList, err)
	}

	logger.Info("Suspend request received",
		zap.String("nodes", nodeList),
		zap.Int("node_count", len(nodes)),
		zap.Bool("dry_run", dryRun))

	if dryRun {
		logger.Info("DRY RUN: Would terminate instances for nodes", zap.Strings("nodes", nodes))
		return nil
	}

	// Group nodes by partition and node group
	nodeGroups := slurmClient.ParseNodeNames(nodes)

	for partition, nodesByGroup := range nodeGroups {
		for nodeGroup, nodeIds := range nodesByGroup {
			if err := suspendNodeGroup(ctx, awsClient, partition, nodeGroup, nodeIds); err != nil {
				logger.Error("Failed to suspend node group",
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

func suspendNodeGroup(ctx context.Context, awsClient *aws.Client, partition, nodeGroup string, nodeIds []string) error {
	logger.Info("Suspending node group",
		zap.String("partition", partition),
		zap.String("node_group", nodeGroup),
		zap.Strings("nodes", nodeIds),
		zap.Int("count", len(nodeIds)))

	// TODO: Implement actual instance termination
	// For now, just log what would be done
	logger.Info("Would terminate instances for nodes",
		zap.String("partition", partition),
		zap.String("node_group", nodeGroup),
		zap.Strings("node_ids", nodeIds))

	return nil
}
