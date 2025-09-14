package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
		Use:   "aws-slurm-burst-state-manager",
		Short: "Manage Slurm node states for AWS instances",
		Long: `Periodic state management for Slurm nodes in AWS partitions.
Handles stuck nodes, failed launches, and state transitions.`,
		RunE: manageStates,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/slurm/aws-burst.yaml", "Configuration file path")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without actually doing it")

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}

func manageStates(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize Slurm client
	slurmClient := slurm.NewClient(logger, &cfg.Slurm)

	logger.Info("Starting state management cycle", zap.Bool("dry_run", dryRun))

	// Get all AWS nodes from all partitions
	var allNodes []string
	for _, partition := range cfg.Slurm.Partitions {
		for _, nodeGroup := range partition.NodeGroups {
			nodeRange := cfg.GetNodeRange(partition.PartitionName, nodeGroup.NodeGroupName, nodeGroup.MaxNodes)
			nodes, err := slurmClient.ParseNodeList(nodeRange)
			if err != nil {
				logger.Error("Failed to parse node list",
					zap.String("partition", partition.PartitionName),
					zap.String("node_group", nodeGroup.NodeGroupName),
					zap.String("node_range", nodeRange),
					zap.Error(err))
				continue
			}
			allNodes = append(allNodes, nodes...)
		}
	}

	if len(allNodes) == 0 {
		logger.Info("No AWS nodes found to manage")
		return nil
	}

	// Get current node states
	nodeStates, err := slurmClient.GetNodeState(allNodes)
	if err != nil {
		return fmt.Errorf("failed to get node states: %w", err)
	}

	logger.Info("Retrieved node states",
		zap.Int("total_nodes", len(allNodes)),
		zap.Int("state_entries", len(nodeStates)))

	// Process each node state and fix issues
	for _, nodeInfo := range nodeStates {
		if err := processNodeState(ctx, slurmClient, nodeInfo); err != nil {
			logger.Error("Failed to process node state",
				zap.String("node", nodeInfo.NodeName),
				zap.String("state", nodeInfo.State),
				zap.Error(err))
		}
	}

	logger.Info("State management cycle completed")
	return nil
}

func processNodeState(ctx context.Context, slurmClient *slurm.Client, nodeInfo slurm.NodeInfo) error {
	// Parse node states (can be comma-separated like "IDLE+CLOUD+POWER")
	states := parseNodeStates(nodeInfo.State)

	logger.Debug("Processing node state",
		zap.String("node", nodeInfo.NodeName),
		zap.String("raw_state", nodeInfo.State),
		zap.Strings("parsed_states", states))

	// Handle stuck or problematic states following original change_state.py logic
	if hasState(states, "DOWN*") || hasState(states, "IDLE*") {
		return changeNodeState(slurmClient, nodeInfo.NodeName, "POWER_DOWN", "node_not_responding")
	}

	if hasState(states, "COMPLETING") && hasState(states, "DRAIN") {
		return changeNodeState(slurmClient, nodeInfo.NodeName, "DOWN", "node_stuck")
	}

	if hasState(states, "DOWN") && hasState(states, "POWER") {
		return changeNodeState(slurmClient, nodeInfo.NodeName, "IDLE", "")
	}

	if hasState(states, "DOWN") && !hasState(states, "POWER") {
		return changeNodeState(slurmClient, nodeInfo.NodeName, "POWER_DOWN", "node_stuck")
	}

	if hasState(states, "DRAIN") && hasState(states, "POWER") {
		return changeNodeState(slurmClient, nodeInfo.NodeName, "UNDRAIN", "")
	}

	return nil
}

func parseNodeStates(stateStr string) []string {
	if stateStr == "" {
		return nil
	}
	return strings.Split(stateStr, "+")
}

func hasState(states []string, targetState string) bool {
	for _, state := range states {
		if state == targetState {
			return true
		}
	}
	return false
}

func changeNodeState(slurmClient *slurm.Client, nodeName, newState, reason string) error {
	if dryRun {
		logger.Info("DRY RUN: Would change node state",
			zap.String("node", nodeName),
			zap.String("new_state", newState),
			zap.String("reason", reason))
		return nil
	}

	return slurmClient.SetNodeState(nodeName, newState, reason)
}
