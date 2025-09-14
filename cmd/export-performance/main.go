package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/internal/slurm"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	configFile   string
	jobID        string
	outputDir    string
	outputFormat string
	anonymize    bool
	logger       *zap.Logger
)

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Printf("Warning: failed to sync logger: %v\n", err)
		}
	}()

	rootCmd := &cobra.Command{
		Use:   "aws-slurm-burst-export-performance",
		Short: "Export job performance data for ASBA learning and analysis",
		Long: `Export comprehensive performance data from completed Slurm jobs
for ASBA learning algorithms and institutional research analytics.

Supports multiple output formats and can be integrated with Slurm epilog scripts
for automatic performance data collection.`,
		RunE: exportPerformanceData,
	}

	rootCmd.Flags().StringVarP(&configFile, "config", "c", "/etc/slurm/aws-burst.yaml", "Configuration file path")
	rootCmd.Flags().StringVar(&jobID, "job-id", "", "Slurm job ID to export performance data for (required)")
	rootCmd.Flags().StringVar(&outputDir, "output-dir", "/var/spool/asba/learning", "Directory to write performance data")
	rootCmd.Flags().StringVar(&outputFormat, "format", "asba-learning", "Output format: asba-learning, json, slurm-comment, asbb-reconciliation")
	rootCmd.Flags().BoolVar(&anonymize, "anonymize", false, "Anonymize user and project data for institutional sharing")

	rootCmd.MarkFlagRequired("job-id")

	if err := rootCmd.Execute(); err != nil {
		logger.Error("Performance export failed", zap.Error(err))
		os.Exit(1)
	}
}

func exportPerformanceData(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize Slurm client
	slurmClient := slurm.NewClient(logger, &cfg.Slurm)

	logger.Info("Exporting performance data",
		zap.String("job_id", jobID),
		zap.String("format", outputFormat),
		zap.String("output_dir", outputDir),
		zap.Bool("anonymize", anonymize))

	// Collect performance data
	perfData, err := collectPerformanceData(ctx, slurmClient, jobID)
	if err != nil {
		return fmt.Errorf("failed to collect performance data: %w", err)
	}

	// Apply anonymization if requested
	if anonymize {
		anonymizePerformanceData(perfData)
	}

	// Export in requested format
	if err := exportData(perfData, outputFormat, outputDir); err != nil {
		return fmt.Errorf("failed to export data: %w", err)
	}

	logger.Info("Performance data exported successfully",
		zap.String("job_id", jobID),
		zap.String("output_dir", outputDir),
		zap.Float64("total_cost", perfData.CostAnalysis.TotalCostUSD))

	return nil
}

// collectPerformanceData gathers comprehensive performance metrics for a job
func collectPerformanceData(ctx context.Context, slurmClient *slurm.Client, jobID string) (*types.PerformanceFeedback, error) {
	// Get job information from Slurm accounting
	jobInfo, err := getJobAccountingInfo(ctx, slurmClient, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job info: %w", err)
	}

	// Build performance feedback structure
	perfData := &types.PerformanceFeedback{
		JobMetadata: types.JobMetadata{
			JobID:     jobID,
			JobName:   jobInfo.JobName,
			UserID:    jobInfo.UserID,
			ProjectID: jobInfo.Account,
			Partition: jobInfo.Partition,
			ActualExecution: types.ActualExecution{
				InstanceTypesUsed: parseInstanceTypesFromComment(jobInfo.Comment),
				ActualCostUSD:     parseActualCostFromComment(jobInfo.Comment),
				ExecutionDuration: types.Duration(jobInfo.Elapsed),
				Success:           jobInfo.State == "COMPLETED",
				NodeCount:         jobInfo.NodeCount,
				StartTime:         jobInfo.Start,
				EndTime:           jobInfo.End,
			},
		},
		PredictionValidation:  calculatePredictionAccuracy(jobInfo),
		AWSPerformanceMetrics: collectAWSMetrics(ctx, jobInfo),
		CostAnalysis:          analyzeCosts(jobInfo),
		ExecutionContext: types.ExecutionContext{
			AWSRegion:     "us-east-1", // TODO: Get from config
			PluginVersion: "0.2.0",
			ExecutionMode: determineExecutionMode(jobInfo.Comment),
			SlurmVersion:  getSlurmVersion(),
		},
	}

	// Add MPI metrics if this was an MPI job
	if isMPIJob(jobInfo) {
		mpiMetrics, err := collectMPIMetrics(ctx, jobInfo)
		if err != nil {
			logger.Warn("Failed to collect MPI metrics", zap.Error(err))
		} else {
			perfData.MPIOptimizationResults = mpiMetrics
		}
	}

	return perfData, nil
}

// JobAccountingInfo represents Slurm job accounting information
type JobAccountingInfo struct {
	JobID     string
	JobName   string
	UserID    string
	Account   string
	Partition string
	State     string
	NodeCount int
	CPUs      int
	Memory    int
	Start     time.Time
	End       time.Time
	Elapsed   time.Duration
	Comment   string
}

// getJobAccountingInfo retrieves job information from Slurm accounting
func getJobAccountingInfo(ctx context.Context, slurmClient *slurm.Client, jobID string) (*JobAccountingInfo, error) {
	// In a real implementation, this would call `sacct` to get job accounting data
	// For now, return mock data for development
	return &JobAccountingInfo{
		JobID:     jobID,
		JobName:   "example-job",
		UserID:    "researcher",
		Account:   "NSF-ABC123",
		Partition: "aws-cpu",
		State:     "COMPLETED",
		NodeCount: 4,
		CPUs:      16,
		Memory:    32768,
		Start:     time.Now().Add(-2 * time.Hour),
		End:       time.Now(),
		Elapsed:   2 * time.Hour,
		Comment:   "aws_meta:{\"instances\":[\"c5n.xlarge\"],\"cost\":12.45,\"efa\":true}",
	}, nil
}

// Helper functions for data processing
func parseInstanceTypesFromComment(comment string) []string {
	// Parse AWS metadata from Slurm job comment
	// For now, return mock data
	return []string{"c5n.xlarge"}
}

func parseActualCostFromComment(comment string) float64 {
	// Parse cost from AWS metadata in comment
	// For now, return mock data
	return 12.45
}

func calculatePredictionAccuracy(jobInfo *JobAccountingInfo) types.PredictionValidation {
	// Calculate how accurate ASBA predictions were
	// For now, return mock data
	return types.PredictionValidation{
		CostAccuracy:           0.94,
		RuntimeAccuracy:        0.91,
		InstanceTypeOptimal:    true,
		DomainDetectionCorrect: true,
		OverallAccuracyScore:   0.92,
	}
}

func collectAWSMetrics(ctx context.Context, jobInfo *JobAccountingInfo) types.AWSPerformanceMetrics {
	// Collect AWS-specific performance metrics
	// For now, return mock data
	return types.AWSPerformanceMetrics{
		EFAUtilization:              0.89,
		PlacementGroupEffectiveness: 0.95,
		SpotInterruptions:           0,
		NetworkThroughputGbps:       45.2,
		CPUUtilization:              0.87,
		MemoryUtilization:           0.76,
		ProvisioningTime:            types.Duration(5 * time.Minute),
		AvailabilityZones:           []string{"us-east-1a", "us-east-1b"},
	}
}

func analyzeCosts(jobInfo *JobAccountingInfo) types.ActualCostAnalysis {
	// Analyze actual costs vs predictions
	return types.ActualCostAnalysis{
		ComputeCostUSD: 12.45,
		StorageCostUSD: 0.25,
		NetworkCostUSD: 0.05,
		TotalCostUSD:   12.75,
		SpotSavingsUSD: 3.21,
		CostPerCPUHour: 0.78,
	}
}

func isMPIJob(jobInfo *JobAccountingInfo) bool {
	// Determine if job was MPI-based
	// Check for MPI indicators in job name or comment
	return jobInfo.NodeCount > 1 // Simple heuristic
}

func collectMPIMetrics(ctx context.Context, jobInfo *JobAccountingInfo) (*types.MPIOptimizationResults, error) {
	// Collect MPI-specific performance metrics
	return &types.MPIOptimizationResults{
		CommunicationOverhead:    0.12,
		ScalingEfficiency:        0.87,
		LoadBalance:              0.93,
		MPICollectiveEfficiency:  0.91,
		CommunicationPattern:     "nearest_neighbor",
		SynchronizationFrequency: 2.5,
	}, nil
}

func determineExecutionMode(comment string) string {
	if comment != "" && (contains(comment, "asba") || contains(comment, "execution_plan")) {
		return "asba"
	}
	return "standalone"
}

func getSlurmVersion() string {
	// Get Slurm version from system
	return "21.08.8" // Mock for development
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(s) > len(substr) && s[1:len(substr)+1] == substr)))
}

func anonymizePerformanceData(perfData *types.PerformanceFeedback) {
	// Remove or hash identifying information
	perfData.JobMetadata.UserID = "anonymous"
	perfData.JobMetadata.ProjectID = "anonymized"
	perfData.ExecutionContext.EnvironmentVariables = map[string]string{}
}

func exportData(perfData *types.PerformanceFeedback, format, outputDir string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	switch format {
	case "asba-learning", "json":
		return exportJSON(perfData, outputDir)
	case "slurm-comment":
		return exportSlurmComment(perfData, outputDir)
	case "asbb-reconciliation":
		return exportASBBReconciliation(perfData, outputDir)
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

func exportJSON(perfData *types.PerformanceFeedback, outputDir string) error {
	filename := filepath.Join(outputDir, fmt.Sprintf("job-%s-performance.json", perfData.JobMetadata.JobID))

	data, err := json.MarshalIndent(perfData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal performance data: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write performance data: %w", err)
	}

	logger.Info("Performance data exported",
		zap.String("format", "json"),
		zap.String("file", filename),
		zap.Int("size_bytes", len(data)))

	return nil
}

func exportSlurmComment(perfData *types.PerformanceFeedback, outputDir string) error {
	// Create compact metadata for Slurm comment field
	metadata := map[string]interface{}{
		"cost":      perfData.CostAnalysis.TotalCostUSD,
		"instances": perfData.JobMetadata.ActualExecution.InstanceTypesUsed,
		"duration":  perfData.JobMetadata.ActualExecution.ExecutionDuration,
		"success":   perfData.JobMetadata.ActualExecution.Success,
	}

	if perfData.MPIOptimizationResults != nil {
		metadata["mpi_eff"] = perfData.MPIOptimizationResults.ScalingEfficiency
	}

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %w", err)
	}

	commentData := fmt.Sprintf("aws_meta:%s", string(jsonData))
	filename := filepath.Join(outputDir, fmt.Sprintf("job-%s-comment.txt", perfData.JobMetadata.JobID))

	if err := os.WriteFile(filename, []byte(commentData), 0644); err != nil {
		return fmt.Errorf("failed to write comment data: %w", err)
	}

	logger.Info("Slurm comment data exported",
		zap.String("format", "slurm-comment"),
		zap.String("file", filename),
		zap.String("comment", commentData))

	return nil
}

func exportASBBReconciliation(perfData *types.PerformanceFeedback, outputDir string) error {
	// Create ASBB-compatible cost reconciliation data
	reconciliationData := map[string]interface{}{
		"job_id":         perfData.JobMetadata.JobID,
		"account":        perfData.JobMetadata.ProjectID,
		"user_id":        perfData.JobMetadata.UserID,
		"partition":      perfData.JobMetadata.Partition,
		"actual_cost":    perfData.CostAnalysis.TotalCostUSD,
		"compute_cost":   perfData.CostAnalysis.ComputeCostUSD,
		"storage_cost":   perfData.CostAnalysis.StorageCostUSD,
		"network_cost":   perfData.CostAnalysis.NetworkCostUSD,
		"spot_savings":   perfData.CostAnalysis.SpotSavingsUSD,
		"instance_types": perfData.JobMetadata.ActualExecution.InstanceTypesUsed,
		"duration_hours": time.Duration(perfData.JobMetadata.ActualExecution.ExecutionDuration).Hours(),
		"success":        perfData.JobMetadata.ActualExecution.Success,
		"export_time":    time.Now().Format(time.RFC3339),
		"plugin_version": perfData.ExecutionContext.PluginVersion,
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("job-%s-asbb-reconciliation.json", perfData.JobMetadata.JobID))

	data, err := json.MarshalIndent(reconciliationData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ASBB reconciliation data: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write ASBB reconciliation data: %w", err)
	}

	logger.Info("ASBB reconciliation data exported",
		zap.String("format", "asbb-reconciliation"),
		zap.String("file", filename),
		zap.Float64("total_cost", perfData.CostAnalysis.TotalCostUSD))

	return nil
}
