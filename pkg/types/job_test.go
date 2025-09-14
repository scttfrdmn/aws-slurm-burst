package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlurmJob_JSON(t *testing.T) {
	job := &SlurmJob{
		JobID:     "12345",
		Name:      "test-job",
		Partition: "aws",
		NodeList:  []string{"aws-cpu-001", "aws-cpu-002"},
		Resources: ResourceSpec{
			Nodes:       2,
			CPUsPerNode: 8,
			MemoryMB:    16384,
			GPUs:        2,
			GPUType:     "V100",
		},
		Constraints: JobConstraints{
			Features:         []string{"efa", "nvme"},
			ExcludeNodes:     []string{"aws-cpu-003"},
			MaxSpotPrice:     0.50,
			AvailabilityZone: "us-east-1a",
		},
		IsMPIJob:     true,
		MPIProcesses: 16,
		MPITopology:  TopologyCluster,
		SubmitTime:   time.Date(2025, 9, 13, 12, 0, 0, 0, time.UTC),
		TimeLimit:    Duration(2 * time.Hour),
		Environment: map[string]string{
			"OMP_NUM_THREADS": "4",
			"MPI_TYPE":        "openmpi",
		},
	}

	// Test marshaling
	data, err := json.Marshal(job)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled SlurmJob
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, job.JobID, unmarshaled.JobID)
	assert.Equal(t, job.Name, unmarshaled.Name)
	assert.Equal(t, job.Partition, unmarshaled.Partition)
	assert.Equal(t, job.NodeList, unmarshaled.NodeList)
	assert.Equal(t, job.Resources, unmarshaled.Resources)
	assert.Equal(t, job.Constraints, unmarshaled.Constraints)
	assert.Equal(t, job.IsMPIJob, unmarshaled.IsMPIJob)
	assert.Equal(t, job.MPIProcesses, unmarshaled.MPIProcesses)
	assert.Equal(t, job.MPITopology, unmarshaled.MPITopology)
	assert.Equal(t, job.Environment, unmarshaled.Environment)
	assert.Equal(t, time.Duration(job.TimeLimit), time.Duration(unmarshaled.TimeLimit))
}

func TestDuration_JSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		expected string
	}{
		{
			name:     "2 hours",
			duration: Duration(2 * time.Hour),
			expected: "\"2h0m0s\"",
		},
		{
			name:     "30 minutes",
			duration: Duration(30 * time.Minute),
			expected: "\"30m0s\"",
		},
		{
			name:     "1 hour 30 minutes",
			duration: Duration(90 * time.Minute),
			expected: "\"1h30m0s\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.duration)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(data))

			// Test unmarshaling
			var unmarshaled Duration
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)
			assert.Equal(t, tt.duration, unmarshaled)
		})
	}
}

func TestNetworkTopology_Values(t *testing.T) {
	tests := []struct {
		name     string
		topology NetworkTopology
		expected string
	}{
		{"cluster", TopologyCluster, "cluster"},
		{"spread", TopologySpread, "spread"},
		{"partition", TopologyPartition, "partition"},
		{"any", TopologyAny, "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.topology))
		})
	}
}

func TestASBADecision_JSON(t *testing.T) {
	decision := &ASBADecision{
		ShouldBurst:       true,
		RecommendedAction: "burst_with_spot",
		CostAnalysis: CostAnalysis{
			OnPremiseCost:  100.50,
			AWSCost:        85.25,
			SavingsPercent: 15.2,
			BreakEvenHours: 2.5,
		},
		PerformanceModel: PerformanceModel{
			OnPremiseWaitTime: 45 * time.Minute,
			AWSProvisionTime:  5 * time.Minute,
			NetworkLatency:    2 * time.Millisecond,
			StorageLatency:    10 * time.Millisecond,
		},
		Confidence:      0.85,
		DecisionFactors: []string{"queue_depth", "cost_savings", "urgency"},
	}

	// Test marshaling
	data, err := json.Marshal(decision)
	require.NoError(t, err)

	// Test unmarshaling
	var unmarshaled ASBADecision
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, decision.ShouldBurst, unmarshaled.ShouldBurst)
	assert.Equal(t, decision.RecommendedAction, unmarshaled.RecommendedAction)
	assert.Equal(t, decision.CostAnalysis, unmarshaled.CostAnalysis)
	assert.Equal(t, decision.Confidence, unmarshaled.Confidence)
	assert.Equal(t, decision.DecisionFactors, unmarshaled.DecisionFactors)

	// Check performance model duration fields
	assert.Equal(t, decision.PerformanceModel.OnPremiseWaitTime, unmarshaled.PerformanceModel.OnPremiseWaitTime)
	assert.Equal(t, decision.PerformanceModel.AWSProvisionTime, unmarshaled.PerformanceModel.AWSProvisionTime)
	assert.Equal(t, decision.PerformanceModel.NetworkLatency, unmarshaled.PerformanceModel.NetworkLatency)
	assert.Equal(t, decision.PerformanceModel.StorageLatency, unmarshaled.PerformanceModel.StorageLatency)
}

func TestResourceSpec_Validation(t *testing.T) {
	tests := []struct {
		name     string
		resource ResourceSpec
		valid    bool
	}{
		{
			name: "valid basic resource",
			resource: ResourceSpec{
				Nodes:       2,
				CPUsPerNode: 4,
				MemoryMB:    8192,
			},
			valid: true,
		},
		{
			name: "valid GPU resource",
			resource: ResourceSpec{
				Nodes:       1,
				CPUsPerNode: 8,
				MemoryMB:    32768,
				GPUs:        4,
				GPUType:     "V100",
			},
			valid: true,
		},
		{
			name: "invalid zero nodes",
			resource: ResourceSpec{
				Nodes:       0,
				CPUsPerNode: 4,
				MemoryMB:    8192,
			},
			valid: false,
		},
		{
			name: "invalid zero CPUs",
			resource: ResourceSpec{
				Nodes:       2,
				CPUsPerNode: 0,
				MemoryMB:    8192,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.resource.Nodes > 0 && tt.resource.CPUsPerNode > 0 && tt.resource.MemoryMB > 0
			assert.Equal(t, tt.valid, valid)
		})
	}
}

func TestJobConstraints_Features(t *testing.T) {
	constraints := JobConstraints{
		Features: []string{"efa", "nvme", "large-memory"},
	}

	// Test feature checking
	hasEFA := false
	for _, feature := range constraints.Features {
		if feature == "efa" {
			hasEFA = true
			break
		}
	}
	assert.True(t, hasEFA, "Should have EFA feature")

	hasInfiniband := false
	for _, feature := range constraints.Features {
		if feature == "infiniband" {
			hasInfiniband = true
			break
		}
	}
	assert.False(t, hasInfiniband, "Should not have InfiniBand feature")
}

// Test for edge cases and error conditions
func TestDuration_InvalidJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{
			name:        "valid duration",
			jsonData:    `"2h30m15s"`,
			expectError: false,
		},
		{
			name:        "invalid duration format",
			jsonData:    `"invalid"`,
			expectError: true,
		},
		{
			name:        "empty string",
			jsonData:    `""`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := json.Unmarshal([]byte(tt.jsonData), &d)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkSlurmJob_JSON(b *testing.B) {
	job := &SlurmJob{
		JobID:     "12345",
		Name:      "benchmark-job",
		Partition: "aws",
		NodeList:  []string{"aws-cpu-001", "aws-cpu-002", "aws-cpu-003", "aws-cpu-004"},
		Resources: ResourceSpec{
			Nodes:       4,
			CPUsPerNode: 8,
			MemoryMB:    16384,
			GPUs:        0,
		},
		Constraints: JobConstraints{
			Features:     []string{"efa", "nvme"},
			MaxSpotPrice: 0.50,
		},
		IsMPIJob:     true,
		MPIProcesses: 32,
		MPITopology:  TopologyCluster,
		TimeLimit:    Duration(2 * time.Hour),
		Environment: map[string]string{
			"OMP_NUM_THREADS": "4",
			"MPI_TYPE":        "openmpi",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(job)
		if err != nil {
			b.Fatal(err)
		}

		var unmarshaled SlurmJob
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			b.Fatal(err)
		}
	}
}
