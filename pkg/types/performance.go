package types

import "time"

// PerformanceFeedback represents comprehensive performance data for ASBA learning
type PerformanceFeedback struct {
	JobMetadata            JobMetadata             `json:"job_metadata"`
	PredictionValidation   PredictionValidation    `json:"prediction_validation"`
	AWSPerformanceMetrics  AWSPerformanceMetrics   `json:"aws_performance_metrics"`
	MPIOptimizationResults *MPIOptimizationResults `json:"mpi_optimization_results,omitempty"`
	CostAnalysis           ActualCostAnalysis      `json:"cost_analysis"`
	ExecutionContext       ExecutionContext        `json:"execution_context"`
}

// JobMetadata contains basic job information and original ASBA prediction
type JobMetadata struct {
	JobID                  string          `json:"job_id"`
	JobName                string          `json:"job_name"`
	UserID                 string          `json:"user_id"`
	ProjectID              string          `json:"project_id"`
	Partition              string          `json:"partition"`
	OriginalASBAPrediction *ExecutionPlan  `json:"original_asba_prediction,omitempty"`
	ActualExecution        ActualExecution `json:"actual_execution"`
}

// ActualExecution contains what actually happened during job execution
type ActualExecution struct {
	InstanceTypesUsed []string  `json:"instance_types_used"`
	ActualCostUSD     float64   `json:"actual_cost_usd"`
	ExecutionDuration Duration  `json:"execution_duration"`
	Success           bool      `json:"success"`
	ErrorDetails      string    `json:"error_details,omitempty"`
	NodeCount         int       `json:"node_count"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
}

// PredictionValidation compares ASBA predictions vs actual results
type PredictionValidation struct {
	CostAccuracy           float64 `json:"cost_accuracy"`            // 0.0-1.0, how accurate was cost prediction
	RuntimeAccuracy        float64 `json:"runtime_accuracy"`         // 0.0-1.0, how accurate was runtime prediction
	InstanceTypeOptimal    bool    `json:"instance_type_optimal"`    // Were chosen instance types actually best?
	DomainDetectionCorrect bool    `json:"domain_detection_correct"` // Was domain correctly identified?
	OverallAccuracyScore   float64 `json:"overall_accuracy_score"`   // Combined accuracy metric
}

// AWSPerformanceMetrics contains AWS-specific performance data
type AWSPerformanceMetrics struct {
	EFAUtilization              float64    `json:"efa_utilization"`               // 0.0-1.0, EFA bandwidth utilization
	PlacementGroupEffectiveness float64    `json:"placement_group_effectiveness"` // 0.0-1.0, placement group benefit
	SpotInterruptions           int        `json:"spot_interruptions"`            // Number of spot interruptions
	NetworkThroughputGbps       float64    `json:"network_throughput_gbps"`       // Peak network throughput
	CPUUtilization              float64    `json:"cpu_utilization"`               // Average CPU utilization
	MemoryUtilization           float64    `json:"memory_utilization"`            // Average memory utilization
	ProvisioningTime            Duration   `json:"provisioning_time"`             // Time to launch instances
	AvailabilityZones           []string   `json:"availability_zones"`            // AZs where instances ran
	InstanceLaunchTimes         []Duration `json:"instance_launch_times"`         // Individual instance launch times
}

// MPIOptimizationResults contains MPI-specific performance metrics (only for MPI jobs)
type MPIOptimizationResults struct {
	CommunicationOverhead    float64              `json:"communication_overhead"`    // 0.0-1.0, % time spent in MPI communication
	ScalingEfficiency        float64              `json:"scaling_efficiency"`        // 0.0-1.0, parallel efficiency
	LoadBalance              float64              `json:"load_balance"`              // 0.0-1.0, work distribution efficiency
	MPICollectiveEfficiency  float64              `json:"mpi_collective_efficiency"` // 0.0-1.0, collective operation efficiency
	CommunicationPattern     string               `json:"communication_pattern"`     // "nearest_neighbor", "all_reduce", etc.
	MessageSizeDistribution  MessageSizeHistogram `json:"message_size_distribution"`
	SynchronizationFrequency float64              `json:"synchronization_frequency"` // MPI barriers per second
	NetworkBottlenecks       []NetworkBottleneck  `json:"network_bottlenecks"`
}

// MessageSizeHistogram represents distribution of MPI message sizes
type MessageSizeHistogram struct {
	SmallMessages  int `json:"small_messages"`  // <1KB
	MediumMessages int `json:"medium_messages"` // 1KB-1MB
	LargeMessages  int `json:"large_messages"`  // >1MB
	AverageSize    int `json:"average_size"`    // Average message size in bytes
}

// NetworkBottleneck represents a detected network performance issue
type NetworkBottleneck struct {
	Type        string  `json:"type"`     // "latency", "bandwidth", "contention"
	Severity    string  `json:"severity"` // "low", "medium", "high"
	Description string  `json:"description"`
	Impact      float64 `json:"impact"` // Performance impact percentage
}

// ActualCostAnalysis contains detailed cost breakdown and analysis
type ActualCostAnalysis struct {
	ComputeCostUSD        float64               `json:"compute_cost_usd"`
	StorageCostUSD        float64               `json:"storage_cost_usd"`
	NetworkCostUSD        float64               `json:"network_cost_usd"`
	TotalCostUSD          float64               `json:"total_cost_usd"`
	SpotSavingsUSD        float64               `json:"spot_savings_usd"`
	CostPerCPUHour        float64               `json:"cost_per_cpu_hour"`
	CostPerGPUHour        float64               `json:"cost_per_gpu_hour,omitempty"`
	InstanceCostBreakdown []InstanceCostDetails `json:"instance_cost_breakdown"`
}

// InstanceCostDetails provides per-instance cost information
type InstanceCostDetails struct {
	InstanceID     string  `json:"instance_id"`
	InstanceType   string  `json:"instance_type"`
	PurchaseOption string  `json:"purchase_option"` // "spot", "on-demand"
	CostUSD        float64 `json:"cost_usd"`
	DurationHours  float64 `json:"duration_hours"`
	Interrupted    bool    `json:"interrupted"` // Was this a spot interruption?
}

// ExecutionContext contains environment and configuration details
type ExecutionContext struct {
	AWSRegion            string            `json:"aws_region"`
	AvailabilityZones    []string          `json:"availability_zones"`
	PluginVersion        string            `json:"plugin_version"`
	ASBAVersion          string            `json:"asba_version,omitempty"`
	SlurmVersion         string            `json:"slurm_version"`
	ExecutionMode        string            `json:"execution_mode"` // "standalone", "asba"
	ConfigurationUsed    map[string]string `json:"configuration_used"`
	EnvironmentVariables map[string]string `json:"environment_variables"`
}

// LearningDataExportOptions configures what data to export for ASBA learning
type LearningDataExportOptions struct {
	IncludePredictionValidation bool   `json:"include_prediction_validation"`
	IncludeAWSMetrics           bool   `json:"include_aws_metrics"`
	IncludeMPIMetrics           bool   `json:"include_mpi_metrics"`
	IncludeCostBreakdown        bool   `json:"include_cost_breakdown"`
	IncludeEnvironmentContext   bool   `json:"include_environment_context"`
	AnonymizeUserData           bool   `json:"anonymize_user_data"`
	ExportFormat                string `json:"export_format"` // "json", "csv", "slurm-comment"
	CompressionEnabled          bool   `json:"compression_enabled"`
}

// LearningDataSummary provides aggregated performance metrics for institutional reporting
type LearningDataSummary struct {
	TimeRange              DateRange              `json:"time_range"`
	JobCount               int                    `json:"job_count"`
	TotalCostUSD           float64                `json:"total_cost_usd"`
	AverageAccuracy        PredictionValidation   `json:"average_accuracy"`
	DomainBreakdown        []DomainPerformance    `json:"domain_breakdown"`
	InstanceTypeEfficiency []InstanceTypeAnalysis `json:"instance_type_efficiency"`
	ImprovementMetrics     ImprovementMetrics     `json:"improvement_metrics"`
}

// DateRange represents a time period for analysis
type DateRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// DomainPerformance contains performance metrics for a research domain
type DomainPerformance struct {
	Domain                 string  `json:"domain"` // "climate_modeling", "machine_learning"
	JobCount               int     `json:"job_count"`
	AverageAccuracy        float64 `json:"average_accuracy"`
	AverageCostSavings     float64 `json:"average_cost_savings"`
	AveragePerformanceGain float64 `json:"average_performance_gain"`
}

// InstanceTypeAnalysis contains efficiency metrics for instance types
type InstanceTypeAnalysis struct {
	InstanceType        string  `json:"instance_type"`
	UsageCount          int     `json:"usage_count"`
	AverageUtilization  float64 `json:"average_utilization"`
	CostEfficiency      float64 `json:"cost_efficiency"`
	RecommendationScore float64 `json:"recommendation_score"` // How often this type was optimal
}

// ImprovementMetrics tracks learning progress over time
type ImprovementMetrics struct {
	AccuracyImprovement     float64 `json:"accuracy_improvement"`     // Improvement in prediction accuracy
	CostOptimization        float64 `json:"cost_optimization"`        // Cost savings improvement
	PerformanceOptimization float64 `json:"performance_optimization"` // Performance improvement
	LearningVelocity        float64 `json:"learning_velocity"`        // Rate of improvement
}
