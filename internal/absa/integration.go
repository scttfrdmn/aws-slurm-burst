package absa

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// ABSAClient handles integration with aws-slurm-burst-advisor
type ABSAClient struct {
	logger       *zap.Logger
	absaCommand  string // Path to absa command
	configPath   string // Path to ABSA configuration
}

// NewABSAClient creates a new ABSA integration client
func NewABSAClient(logger *zap.Logger, absaCommand, configPath string) *ABSAClient {
	return &ABSAClient{
		logger:      logger,
		absaCommand: absaCommand,
		configPath:  configPath,
	}
}

// AnalyzeBurstDecision consults ABSA to determine if a job should burst to AWS
func (c *ABSAClient) AnalyzeBurstDecision(ctx context.Context, job *types.SlurmJob) (*types.ABSADecision, error) {
	c.logger.Debug("Consulting ABSA for burst decision", zap.String("job_id", job.JobID))

	// Create temporary job script for ABSA analysis
	tempScript, err := c.createTempJobScript(job)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}

	// Build ABSA command
	args := []string{
		"analyze",
		tempScript,
		"--output=json",
		"--job-id=" + job.JobID,
	}

	if c.configPath != "" {
		args = append(args, "--config="+c.configPath)
	}

	// Add MPI-specific parameters for better cost modeling
	if job.IsMPIJob {
		args = append(args,
			"--mpi-job=true",
			fmt.Sprintf("--processes=%d", job.MPIProcesses),
			fmt.Sprintf("--nodes=%d", job.Resources.Nodes),
		)

		// Include network requirements for cost calculation
		if job.MPITopology == types.TopologyCluster {
			args = append(args, "--requires-low-latency=true")
		}
	}

	// Execute ABSA command
	cmd := exec.CommandContext(ctx, c.absaCommand, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ABSA command failed: %w", err)
	}

	// Parse ABSA decision
	var decision types.ABSADecision
	if err := json.Unmarshal(output, &decision); err != nil {
		return nil, fmt.Errorf("failed to parse ABSA decision: %w", err)
	}

	c.logger.Info("ABSA burst decision received",
		zap.String("job_id", job.JobID),
		zap.Bool("should_burst", decision.ShouldBurst),
		zap.String("action", decision.RecommendedAction),
		zap.Float64("confidence", decision.Confidence))

	return &decision, nil
}

// EnrichJobWithABSAData adds ABSA recommendations to job analysis
func (c *ABSAClient) EnrichJobWithABSAData(ctx context.Context, job *types.SlurmJob, instanceReq *types.InstanceRequirements) error {
	decision, err := c.AnalyzeBurstDecision(ctx, job)
	if err != nil {
		c.logger.Warn("Failed to get ABSA decision, proceeding without it",
			zap.String("job_id", job.JobID),
			zap.Error(err))
		return nil // Don't fail the job, just proceed without ABSA input
	}

	// Apply ABSA cost constraints to instance requirements
	if decision.CostAnalysis.AWSCost > 0 {
		// Set maximum spot price based on ABSA cost analysis
		maxCostPerHour := decision.CostAnalysis.AWSCost / float64(job.Resources.Nodes)
		if instanceReq.MaxSpotPrice == 0 || maxCostPerHour < instanceReq.MaxSpotPrice {
			instanceReq.MaxSpotPrice = maxCostPerHour * 0.8 // 20% buffer
		}

		// Enable spot instances if ABSA indicates significant savings
		if decision.CostAnalysis.SavingsPercent > 30 {
			instanceReq.PreferSpot = true
		}
	}

	// Adjust strategy based on urgency
	if decision.PerformanceModel.OnPremiseWaitTime.Minutes() > 60 {
		// Job is urgent, prefer reliability over cost
		instanceReq.AllowMixedPricing = true
		instanceReq.PreferSpot = false
	}

	c.logger.Debug("Enriched job with ABSA data",
		zap.String("job_id", job.JobID),
		zap.Float64("max_spot_price", instanceReq.MaxSpotPrice),
		zap.Bool("prefer_spot", instanceReq.PreferSpot),
		zap.Bool("allow_mixed_pricing", instanceReq.AllowMixedPricing))

	return nil
}

// createTempJobScript creates a temporary job script file for ABSA analysis
func (c *ABSAClient) createTempJobScript(job *types.SlurmJob) (string, error) {
	// For now, return the job script directly if available
	// In production, you might want to write to a temporary file
	if job.Script != "" {
		return job.Script, nil
	}

	// Generate basic script from job parameters
	script := fmt.Sprintf(`#!/bin/bash
#SBATCH --job-name=%s
#SBATCH --partition=%s
#SBATCH --nodes=%d
#SBATCH --ntasks-per-node=%d
#SBATCH --mem=%dMB
#SBATCH --time=%s

# Generated script for ABSA analysis
echo "Job analysis for %s"
`,
		job.Name,
		job.Partition,
		job.Resources.Nodes,
		job.Resources.CPUsPerNode,
		job.Resources.MemoryMB,
		time.Duration(job.TimeLimit).String(),
		job.JobID)

	return script, nil
}

// ValidateABSAAvailability checks if ABSA is available and configured
func (c *ABSAClient) ValidateABSAAvailability(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.absaCommand, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ABSA command not available: %w", err)
	}

	version := strings.TrimSpace(string(output))
	c.logger.Info("ABSA availability confirmed", zap.String("version", version))

	return nil
}

// GetRecommendedInstanceTypes asks ABSA for instance type recommendations
func (c *ABSAClient) GetRecommendedInstanceTypes(ctx context.Context, job *types.SlurmJob) ([]string, error) {
	args := []string{
		"recommend-instances",
		"--job-id=" + job.JobID,
		fmt.Sprintf("--cpus=%d", job.Resources.CPUsPerNode),
		fmt.Sprintf("--memory=%d", job.Resources.MemoryMB),
		fmt.Sprintf("--nodes=%d", job.Resources.Nodes),
		"--output=json",
	}

	if job.IsMPIJob {
		args = append(args, "--workload-type=mpi")
		if job.MPITopology == types.TopologyCluster {
			args = append(args, "--network-intensive=true")
		}
	}

	cmd := exec.CommandContext(ctx, c.absaCommand, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ABSA instance recommendation failed: %w", err)
	}

	var recommendation struct {
		InstanceTypes []string `json:"instance_types"`
		Reasoning     string   `json:"reasoning"`
	}

	if err := json.Unmarshal(output, &recommendation); err != nil {
		return nil, fmt.Errorf("failed to parse instance recommendations: %w", err)
	}

	c.logger.Debug("Received ABSA instance recommendations",
		zap.String("job_id", job.JobID),
		zap.Strings("instance_types", recommendation.InstanceTypes),
		zap.String("reasoning", recommendation.Reasoning))

	return recommendation.InstanceTypes, nil
}