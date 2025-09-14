package types

import (
	"time"
)

// SlurmJob represents a parsed Slurm job with resource requirements
type SlurmJob struct {
	JobID       string            `json:"job_id"`
	Name        string            `json:"name"`
	Partition   string            `json:"partition"`
	NodeList    []string          `json:"node_list"`
	Resources   ResourceSpec      `json:"resources"`
	Constraints JobConstraints    `json:"constraints"`
	Script      string            `json:"script,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`

	// MPI-specific fields
	IsMPIJob     bool            `json:"is_mpi_job"`
	MPIProcesses int             `json:"mpi_processes,omitempty"`
	MPITopology  NetworkTopology `json:"mpi_topology,omitempty"`

	// Timing
	SubmitTime time.Time  `json:"submit_time"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	TimeLimit  Duration   `json:"time_limit"`
}

type ResourceSpec struct {
	Nodes        int    `json:"nodes"`
	CPUsPerNode  int    `json:"cpus_per_node"`
	MemoryMB     int    `json:"memory_mb"`
	GPUs         int    `json:"gpus,omitempty"`
	GPUType      string `json:"gpu_type,omitempty"`
	LocalStorage int    `json:"local_storage_gb,omitempty"`
}

type JobConstraints struct {
	Features         []string `json:"features,omitempty"`
	ExcludeNodes     []string `json:"exclude_nodes,omitempty"`
	RequiredNodes    []string `json:"required_nodes,omitempty"`
	AvailabilityZone string   `json:"availability_zone,omitempty"`
	MaxSpotPrice     float64  `json:"max_spot_price,omitempty"`
}

type NetworkTopology string

const (
	TopologyCluster   NetworkTopology = "cluster"   // Cluster placement group
	TopologySpread    NetworkTopology = "spread"    // Spread across AZs
	TopologyPartition NetworkTopology = "partition" // Partition placement group
	TopologyAny       NetworkTopology = "any"       // No specific requirements
)

// ASBADecision represents decision data from aws-slurm-burst-advisor
type ASBADecision struct {
	ShouldBurst       bool             `json:"should_burst"`
	RecommendedAction string           `json:"recommended_action"`
	CostAnalysis      CostAnalysis     `json:"cost_analysis"`
	PerformanceModel  PerformanceModel `json:"performance_model"`
	Confidence        float64          `json:"confidence"`
	DecisionFactors   []string         `json:"decision_factors"`
}

type CostAnalysis struct {
	OnPremiseCost  float64 `json:"onpremise_cost"`
	AWSCost        float64 `json:"aws_cost"`
	SavingsPercent float64 `json:"savings_percent"`
	BreakEvenHours float64 `json:"break_even_hours"`
}

type PerformanceModel struct {
	OnPremiseWaitTime time.Duration `json:"onpremise_wait_time"`
	AWSProvisionTime  time.Duration `json:"aws_provision_time"`
	NetworkLatency    time.Duration `json:"network_latency"`
	StorageLatency    time.Duration `json:"storage_latency"`
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	str := string(data[1 : len(data)-1]) // Remove quotes
	dur, err := time.ParseDuration(str)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}
