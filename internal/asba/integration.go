package asba

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

// ASBAClient handles integration with aws-slurm-burst-advisor
type ASBAClient struct {
	logger      *zap.Logger
	asbaCommand string // Path to asba command
	configPath  string // Path to ASBA configuration
}

// NewASBAClient creates a new ASBA integration client
func NewASBAClient(logger *zap.Logger, asbaCommand, configPath string) *ASBAClient {
	return &ASBAClient{
		logger:      logger,
		asbaCommand: asbaCommand,
		configPath:  configPath,
	}
}

// AnalyzeBurstDecision consults ASBA to determine if a job should burst to AWS
func (c *ASBAClient) AnalyzeBurstDecision(ctx context.Context, job *types.SlurmJob) (*types.ASBADecision, error) {
	c.logger.Debug("Consulting ASBA for burst decision", zap.String("job_id", job.JobID))

	// Create temporary job script for ASBA analysis
	tempScript, err := c.createTempJobScript(job)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}

	// Build ASBA command
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

	// Execute ASBA command
	cmd := exec.CommandContext(ctx, c.asbaCommand, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ASBA command failed: %w", err)
	}

	// Parse ASBA decision
	var decision types.ASBADecision
	if err := json.Unmarshal(output, &decision); err != nil {
		return nil, fmt.Errorf("failed to parse ASBA decision: %w", err)
	}

	c.logger.Info("ASBA burst decision received",
		zap.String("job_id", job.JobID),
		zap.Bool("should_burst", decision.ShouldBurst),
		zap.String("action", decision.RecommendedAction),
		zap.Float64("confidence", decision.Confidence))

	return &decision, nil
}

// EnrichJobWithASBAData adds ASBA recommendations to job analysis
func (c *ASBAClient) EnrichJobWithASBAData(ctx context.Context, job *types.SlurmJob, instanceReq *types.InstanceRequirements) error {
	decision, err := c.AnalyzeBurstDecision(ctx, job)
	if err != nil {
		c.logger.Warn("Failed to get ASBA decision, proceeding without it",
			zap.String("job_id", job.JobID),
			zap.Error(err))
		return nil // Don't fail the job, just proceed without ASBA input
	}

	// Apply ASBA cost constraints to instance requirements
	if decision.CostAnalysis.AWSCost > 0 {
		// Set maximum spot price based on ASBA cost analysis
		maxCostPerHour := decision.CostAnalysis.AWSCost / float64(job.Resources.Nodes)
		if instanceReq.MaxSpotPrice == 0 || maxCostPerHour < instanceReq.MaxSpotPrice {
			instanceReq.MaxSpotPrice = maxCostPerHour * 0.8 // 20% buffer
		}

		// Enable spot instances if ASBA indicates significant savings
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

	c.logger.Debug("Enriched job with ASBA data",
		zap.String("job_id", job.JobID),
		zap.Float64("max_spot_price", instanceReq.MaxSpotPrice),
		zap.Bool("prefer_spot", instanceReq.PreferSpot),
		zap.Bool("allow_mixed_pricing", instanceReq.AllowMixedPricing))

	return nil
}

// createTempJobScript creates a temporary job script file for ASBA analysis
func (c *ASBAClient) createTempJobScript(job *types.SlurmJob) (string, error) {
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

# Generated script for ASBA analysis
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

// ValidateASBAAvailability checks if ASBA is available and configured
func (c *ASBAClient) ValidateASBAAvailability(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.asbaCommand, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ASBA command not available: %w", err)
	}

	version := strings.TrimSpace(string(output))
	c.logger.Info("ASBA availability confirmed", zap.String("version", version))

	return nil
}

// GetRecommendedInstanceTypes asks ASBA for instance type recommendations
func (c *ASBAClient) GetRecommendedInstanceTypes(ctx context.Context, job *types.SlurmJob) ([]string, error) {
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

	cmd := exec.CommandContext(ctx, c.asbaCommand, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ASBA instance recommendation failed: %w", err)
	}

	var recommendation struct {
		InstanceTypes []string `json:"instance_types"`
		Reasoning     string   `json:"reasoning"`
	}

	if err := json.Unmarshal(output, &recommendation); err != nil {
		return nil, fmt.Errorf("failed to parse instance recommendations: %w", err)
	}

	c.logger.Debug("Received ASBA instance recommendations",
		zap.String("job_id", job.JobID),
		zap.Strings("instance_types", recommendation.InstanceTypes),
		zap.String("reasoning", recommendation.Reasoning))

	return recommendation.InstanceTypes, nil
}
