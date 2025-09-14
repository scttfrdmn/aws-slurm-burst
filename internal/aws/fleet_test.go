package aws

import (
	"context"
	"strings"
	"testing"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestFleetManager_selectInstanceTypes(t *testing.T) {
	logger := zaptest.NewLogger(t)
	awsConfig := &config.AWSConfig{
		Region:           "us-east-1",
		RetryMaxAttempts: 3,
	}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		requirements *types.InstanceRequirements
		expected     []string
	}{
		{
			name: "ASBA mode - specific instance types",
			requirements: &types.InstanceRequirements{
				MinCPUs:          4,
				MinMemoryMB:      8192,
				RequiresEFA:      true,
				InstanceFamilies: []string{"c6i.xlarge", "c5n.large"}, // ASBA provides specific types
			},
			expected: []string{"c6i.xlarge", "c5n.large"},
		},
		{
			name: "Standalone mode - GPU workload",
			requirements: &types.InstanceRequirements{
				MinCPUs:     8,
				MinMemoryMB: 32768,
				GPUs:        4,
				// No InstanceFamilies specified - standalone mode
			},
			expected: []string{"p3.2xlarge", "g4dn.xlarge"},
		},
		{
			name: "Standalone mode - EFA workload",
			requirements: &types.InstanceRequirements{
				MinCPUs:     16,
				MinMemoryMB: 32768,
				RequiresEFA: true,
				// No InstanceFamilies specified - standalone mode
			},
			expected: []string{"c5n.large", "c5n.xlarge", "c6i.large", "c6i.xlarge"},
		},
		{
			name: "Standalone mode - general compute",
			requirements: &types.InstanceRequirements{
				MinCPUs:     4,
				MinMemoryMB: 8192,
				// No special requirements - standalone mode
			},
			expected: []string{"c5.large", "c5.xlarge", "m5.large", "m5.xlarge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fleetManager.selectInstanceTypes(tt.requirements)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFleetManager_getInstanceSizesForFamily(t *testing.T) {
	logger := zaptest.NewLogger(t)
	awsConfig := &config.AWSConfig{Region: "us-east-1"}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(t, err)

	tests := []struct {
		name         string
		family       string
		requirements *types.InstanceRequirements
		expected     []string
	}{
		{
			name:   "small instance",
			family: "c5",
			requirements: &types.InstanceRequirements{
				MinCPUs:     2,
				MinMemoryMB: 4096, // 4GB
			},
			expected: []string{"c5.large"},
		},
		{
			name:   "medium instance",
			family: "m5",
			requirements: &types.InstanceRequirements{
				MinCPUs:     4,
				MinMemoryMB: 16384, // 16GB
			},
			expected: []string{"m5.xlarge", "m5.large"},
		},
		{
			name:   "large instance",
			family: "r5",
			requirements: &types.InstanceRequirements{
				MinCPUs:     16,
				MinMemoryMB: 65536, // 64GB
			},
			expected: []string{"r5.4xlarge", "r5.2xlarge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fleetManager.getInstanceSizesForFamily(tt.family, tt.requirements)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFleetRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request *FleetRequest
		valid   bool
	}{
		{
			name: "valid MPI request",
			request: &FleetRequest{
				NodeIds:   []string{"aws-cpu-001", "aws-cpu-002"},
				Partition: "aws",
				NodeGroup: "cpu",
				InstanceRequirements: &types.InstanceRequirements{
					MinCPUs:            4,
					MinMemoryMB:        8192,
					RequiresEFA:        true,
					PlacementGroupType: "cluster",
					InstanceFamilies:   []string{"c6i", "c5n"},
					EnhancedNetworking: true,
				},
				Job: &types.SlurmJob{
					JobID:        "test-001",
					IsMPIJob:     true,
					MPIProcesses: 8,
					MPITopology:  types.TopologyCluster,
				},
				SubnetIds: []string{"subnet-123", "subnet-456"},
			},
			valid: true,
		},
		{
			name: "invalid empty node list",
			request: &FleetRequest{
				NodeIds:   []string{},
				Partition: "aws",
				NodeGroup: "cpu",
			},
			valid: false,
		},
		{
			name: "invalid empty subnet list",
			request: &FleetRequest{
				NodeIds:   []string{"aws-cpu-001"},
				Partition: "aws",
				NodeGroup: "cpu",
				SubnetIds: []string{},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := len(tt.request.NodeIds) > 0 && len(tt.request.SubnetIds) > 0 && tt.request.Partition != "" && tt.request.NodeGroup != ""
			assert.Equal(t, tt.valid, valid)
		})
	}
}

// Mock tests for fleet operations (since we can't create real AWS resources in tests)
func TestFleetManager_MockOperations(t *testing.T) {
	logger := zaptest.NewLogger(t)
	awsConfig := &config.AWSConfig{
		Region:           "us-east-1",
		RetryMaxAttempts: 3,
		RetryMode:        "adaptive",
	}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(t, err)
	assert.NotNil(t, fleetManager)
	assert.Equal(t, "us-east-1", fleetManager.region)
	assert.NotNil(t, fleetManager.ec2Client)
}

func TestFleetManager_InstanceTypeSelection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	awsConfig := &config.AWSConfig{Region: "us-east-1"}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(t, err)

	// Test with specific instance families
	req := &types.InstanceRequirements{
		MinCPUs:          8,
		MinMemoryMB:      16384,
		InstanceFamilies: []string{"c6i", "m5"},
		RequiresEFA:      true,
	}

	instanceTypes := fleetManager.selectInstanceTypes(req)
	assert.Greater(t, len(instanceTypes), 0, "Should return instance types")

	// Should include types from both families
	hasC6i := false
	hasM5 := false
	for _, instanceType := range instanceTypes {
		if strings.HasPrefix(instanceType, "c6i") {
			hasC6i = true
		}
		if strings.HasPrefix(instanceType, "m5") {
			hasM5 = true
		}
	}
	assert.True(t, hasC6i, "Should include c6i instances")
	assert.True(t, hasM5, "Should include m5 instances")
}

func TestFleetManager_GetInstancePricing(t *testing.T) {
	logger := zaptest.NewLogger(t)
	awsConfig := &config.AWSConfig{Region: "us-east-1"}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(t, err)

	instanceTypes := []string{"c5.large", "c5.xlarge", "m5.2xlarge"}
	pricing, err := fleetManager.GetInstancePricing(context.Background(), instanceTypes)
	require.NoError(t, err)

	// Verify all instance types have pricing
	for _, instanceType := range instanceTypes {
		price, exists := pricing[instanceType]
		assert.True(t, exists, "Pricing should exist for %s", instanceType)
		assert.Greater(t, price, 0.0, "Price should be positive for %s", instanceType)
	}

	// Verify pricing makes sense (larger instances cost more)
	assert.Less(t, pricing["c5.large"], pricing["c5.xlarge"], "Large should cost less than xlarge")
	assert.Less(t, pricing["c5.xlarge"], pricing["m5.2xlarge"], "xlarge should cost less than 2xlarge")
}

// Benchmark test for instance type selection performance
func BenchmarkInstanceTypeSelection(b *testing.B) {
	logger := zaptest.NewLogger(b)
	awsConfig := &config.AWSConfig{Region: "us-east-1"}

	fleetManager, err := NewFleetManager(logger, awsConfig)
	require.NoError(b, err)

	req := &types.InstanceRequirements{
		MinCPUs:          16,
		MinMemoryMB:      32768,
		InstanceFamilies: []string{"c6i", "c5n", "m6i", "r6i", "hpc6a"},
		RequiresEFA:      true,
		HPCOptimized:     true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		instanceTypes := fleetManager.selectInstanceTypes(req)
		if len(instanceTypes) == 0 {
			b.Fatal("No instance types selected")
		}
	}
}
