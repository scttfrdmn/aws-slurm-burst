package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		expectedAWS   AWSConfig
	}{
		{
			name: "valid config",
			configContent: `
aws:
  region: us-west-2
  profile: test-profile
slurm:
  bin_path: /usr/local/bin/
  partitions:
    - partition_name: aws
      node_groups:
        - node_group_name: cpu
          max_nodes: 10
          region: us-west-2
          purchasing_option: on-demand
          launch_template_overrides:
            - instance_type: c5.large
          subnet_ids:
            - subnet-123456
`,
			expectError: false,
			expectedAWS: AWSConfig{
				Region:  "us-west-2",
				Profile: "test-profile",
			},
		},
		{
			name: "missing required fields",
			configContent: `
slurm:
  partitions: []
`,
			expectError: true,
		},
		{
			name: "invalid partition name",
			configContent: `
aws:
  region: us-east-1
slurm:
  partitions:
    - partition_name: "aws-invalid"
      node_groups:
        - node_group_name: cpu
          max_nodes: 10
          region: us-east-1
          purchasing_option: on-demand
          launch_template_overrides:
            - instance_type: c5.large
          subnet_ids:
            - subnet-123456
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			config, err := Load(configPath)

			if tt.expectError {
				assert.Error(t, err, "Expected error for test: %s", tt.name)
				if err != nil {
					t.Logf("Got expected error: %v", err)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedAWS.Region, config.AWS.Region)
			assert.Equal(t, tt.expectedAWS.Profile, config.AWS.Profile)
		})
	}
}

func TestValidatePartition(t *testing.T) {
	tests := []struct {
		name        string
		partition   PartitionConfig
		expectError bool
	}{
		{
			name: "valid partition",
			partition: PartitionConfig{
				PartitionName: "aws",
				NodeGroups: []NodeGroupConfig{
					{
						NodeGroupName:    "cpu",
						MaxNodes:         10,
						Region:           "us-east-1",
						PurchasingOption: "on-demand",
						LaunchTemplateOverrides: []LaunchTemplateOverride{
							{InstanceType: "c5.large"},
						},
						SubnetIds: []string{"subnet-123456"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty partition name",
			partition: PartitionConfig{
				PartitionName: "",
				NodeGroups:    []NodeGroupConfig{},
			},
			expectError: true,
		},
		{
			name: "invalid partition name with hyphen",
			partition: PartitionConfig{
				PartitionName: "aws-test",
				NodeGroups:    []NodeGroupConfig{},
			},
			expectError: true,
		},
		{
			name: "empty node groups",
			partition: PartitionConfig{
				PartitionName: "aws",
				NodeGroups:    []NodeGroupConfig{},
			},
			expectError: true,
		},
		{
			name: "invalid purchasing option",
			partition: PartitionConfig{
				PartitionName: "aws",
				NodeGroups: []NodeGroupConfig{
					{
						NodeGroupName:    "cpu",
						MaxNodes:         10,
						Region:           "us-east-1",
						PurchasingOption: "invalid",
						LaunchTemplateOverrides: []LaunchTemplateOverride{
							{InstanceType: "c5.large"},
						},
						SubnetIds: []string{"subnet-123456"},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePartition(tt.partition, 0)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetNodeName(t *testing.T) {
	config := &Config{}

	tests := []struct {
		name          string
		partitionName string
		nodeGroupName string
		nodeID        string
		expected      string
	}{
		{
			name:          "with node ID",
			partitionName: "aws",
			nodeGroupName: "cpu",
			nodeID:        "001",
			expected:      "aws-cpu-001",
		},
		{
			name:          "without node ID",
			partitionName: "aws",
			nodeGroupName: "gpu",
			nodeID:        "",
			expected:      "aws-gpu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetNodeName(tt.partitionName, tt.nodeGroupName, tt.nodeID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNodeRange(t *testing.T) {
	config := &Config{}

	tests := []struct {
		name          string
		partitionName string
		nodeGroupName string
		maxNodes      int
		expected      string
	}{
		{
			name:          "single node",
			partitionName: "aws",
			nodeGroupName: "cpu",
			maxNodes:      1,
			expected:      "aws-cpu-0",
		},
		{
			name:          "multiple nodes",
			partitionName: "aws",
			nodeGroupName: "gpu",
			maxNodes:      5,
			expected:      "aws-gpu-[0-4]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.GetNodeRange(tt.partitionName, tt.nodeGroupName, tt.maxNodes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindNodeGroup(t *testing.T) {
	config := &Config{
		Slurm: SlurmConfig{
			Partitions: []PartitionConfig{
				{
					PartitionName: "aws",
					NodeGroups: []NodeGroupConfig{
						{
							NodeGroupName: "cpu",
							MaxNodes:      10,
						},
						{
							NodeGroupName: "gpu",
							MaxNodes:      5,
						},
					},
				},
				{
					PartitionName: "local",
					NodeGroups: []NodeGroupConfig{
						{
							NodeGroupName: "compute",
							MaxNodes:      20,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		partitionName string
		nodeGroupName string
		expected      *NodeGroupConfig
	}{
		{
			name:          "found node group",
			partitionName: "aws",
			nodeGroupName: "cpu",
			expected: &NodeGroupConfig{
				NodeGroupName: "cpu",
				MaxNodes:      10,
			},
		},
		{
			name:          "node group not found",
			partitionName: "aws",
			nodeGroupName: "nonexistent",
			expected:      nil,
		},
		{
			name:          "partition not found",
			partitionName: "nonexistent",
			nodeGroupName: "cpu",
			expected:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.FindNodeGroup(tt.partitionName, tt.nodeGroupName)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.NodeGroupName, result.NodeGroupName)
				assert.Equal(t, tt.expected.MaxNodes, result.MaxNodes)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected Config
	}{
		{
			name: "add trailing slash to bin path",
			config: Config{
				Slurm: SlurmConfig{
					BinPath: "/usr/bin",
				},
			},
			expected: Config{
				Slurm: SlurmConfig{
					BinPath: "/usr/bin/",
				},
			},
		},
		{
			name: "bin path already has slash",
			config: Config{
				Slurm: SlurmConfig{
					BinPath: "/usr/bin/",
				},
			},
			expected: Config{
				Slurm: SlurmConfig{
					BinPath: "/usr/bin/",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalize(&tt.config)
			assert.Equal(t, tt.expected.Slurm.BinPath, tt.config.Slurm.BinPath)
		})
	}
}

// Integration test with actual config file
func TestLoadIntegration(t *testing.T) {
	configContent := `
aws:
  region: us-east-1
  retry_max_attempts: 5

slurm:
  bin_path: /opt/slurm/bin
  private_data: CLOUD
  resume_rate: 50
  partitions:
    - partition_name: aws
      node_groups:
        - node_group_name: cpu
          max_nodes: 20
          region: us-east-1
          purchasing_option: spot
          launch_template_overrides:
            - instance_type: c5.large
            - instance_type: c5.xlarge
          subnet_ids:
            - subnet-12345
            - subnet-67890
        - node_group_name: gpu
          max_nodes: 5
          region: us-east-1
          purchasing_option: on-demand
          launch_template_overrides:
            - instance_type: p3.2xlarge
          subnet_ids:
            - subnet-12345

absa:
  enabled: true
  command: /usr/local/bin/absa
  timeout_seconds: 60

mpi:
  efa_default: required
  hpc_instances_threshold: 4

logging:
  level: debug
  format: json
  max_size_mb: 50
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := Load(configPath)
	require.NoError(t, err)

	// Validate loaded configuration
	assert.Equal(t, "us-east-1", config.AWS.Region)
	assert.Equal(t, 5, config.AWS.RetryMaxAttempts)
	assert.Equal(t, "/opt/slurm/bin/", config.Slurm.BinPath) // Should be normalized
	assert.Equal(t, "CLOUD", config.Slurm.PrivateData)
	assert.Equal(t, 50, config.Slurm.ResumeRate)

	require.Len(t, config.Slurm.Partitions, 1)
	partition := config.Slurm.Partitions[0]
	assert.Equal(t, "aws", partition.PartitionName)
	require.Len(t, partition.NodeGroups, 2)

	cpuGroup := partition.NodeGroups[0]
	assert.Equal(t, "cpu", cpuGroup.NodeGroupName)
	assert.Equal(t, 20, cpuGroup.MaxNodes)
	assert.Equal(t, "spot", cpuGroup.PurchasingOption)
	assert.Len(t, cpuGroup.LaunchTemplateOverrides, 2)
	assert.Len(t, cpuGroup.SubnetIds, 2)

	gpuGroup := partition.NodeGroups[1]
	assert.Equal(t, "gpu", gpuGroup.NodeGroupName)
	assert.Equal(t, 5, gpuGroup.MaxNodes)
	assert.Equal(t, "on-demand", gpuGroup.PurchasingOption)

	assert.True(t, config.ABSA.Enabled)
	assert.Equal(t, "/usr/local/bin/absa", config.ABSA.Command)
	assert.Equal(t, 60, config.ABSA.Timeout)

	assert.Equal(t, "required", config.MPI.EFADefault)
	assert.Equal(t, 4, config.MPI.HPCInstancesThreshold)

	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.Equal(t, 50, config.Logging.MaxSize)
}