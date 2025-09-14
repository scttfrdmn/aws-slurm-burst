//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/internal/scheduler"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestMPIIntegration tests the complete MPI analysis pipeline
func TestMPIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if not in CI or if AWS credentials not available
	if os.Getenv("CI") == "" && os.Getenv("AWS_REGION") == "" {
		t.Skip("Skipping integration test - not in CI environment")
	}

	logger := zaptest.NewLogger(t)
	scheduler := scheduler.NewMPIScheduler(logger)

	testCases := []struct {
		name                   string
		job                    *types.SlurmJob
		expectedMPI            bool
		expectedEFARequired    bool
		expectedEFAPreferred   bool
		expectedHPCOptimized   bool
		expectedPlacementGroup bool
		expectedInstanceTypes  []string
	}{
		{
			name: "large_scale_gromacs",
			job: &types.SlurmJob{
				JobID:  "test-gromacs-001",
				Name:   "gromacs-protein-fold",
				Script: "#!/bin/bash\n#SBATCH --constraint=efa-required\nmpirun -np 128 gmx_mpi mdrun -deffnm production",
				Resources: types.ResourceSpec{
					Nodes:       16,
					CPUsPerNode: 8,
					MemoryMB:    32768,
				},
				Constraints: types.JobConstraints{
					Features: []string{"efa-required"},
				},
			},
			expectedMPI:            true,
			expectedEFARequired:    true,
			expectedEFAPreferred:   true,
			expectedHPCOptimized:   true,
			expectedPlacementGroup: true,
			expectedInstanceTypes:  []string{"hpc7a", "hpc6id", "hpc6a"},
		},
		{
			name: "medium_scale_lammps",
			job: &types.SlurmJob{
				JobID:  "test-lammps-001",
				Name:   "lammps-molecular-dynamics",
				Script: "#!/bin/bash\nmpirun -np 32 lmp_mpi < input.lammps",
				Resources: types.ResourceSpec{
					Nodes:       4,
					CPUsPerNode: 8,
					MemoryMB:    16384,
				},
			},
			expectedMPI:            true,
			expectedEFARequired:    false,
			expectedEFAPreferred:   true,
			expectedHPCOptimized:   false,
			expectedPlacementGroup: true,
			expectedInstanceTypes:  []string{"c6in", "c6i", "c5n"},
		},
		{
			name: "gpu_ml_workload",
			job: &types.SlurmJob{
				JobID:  "test-ml-001",
				Name:   "pytorch-distributed-training",
				Script: "#!/bin/bash\ntorchrun --nproc_per_node=4 train.py",
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 8,
					MemoryMB:    65536,
					GPUs:        4,
					GPUType:     "V100",
				},
			},
			expectedMPI:            false, // torchrun is not detected as traditional MPI
			expectedEFARequired:    false,
			expectedEFAPreferred:   false,
			expectedHPCOptimized:   false,
			expectedPlacementGroup: true,
			expectedInstanceTypes:  []string{"p4d", "p3dn", "g4dn"},
		},
		{
			name: "embarrassingly_parallel",
			job: &types.SlurmJob{
				JobID:  "test-parallel-001",
				Name:   "parameter-sweep",
				Script: "#!/bin/bash\n./run_simulation.sh $SLURM_ARRAY_TASK_ID",
				Resources: types.ResourceSpec{
					Nodes:       8,
					CPUsPerNode: 4,
					MemoryMB:    8192,
				},
			},
			expectedMPI:            false,
			expectedEFARequired:    false,
			expectedEFAPreferred:   false,
			expectedHPCOptimized:   false,
			expectedPlacementGroup: false,
			expectedInstanceTypes:  []string{"c6i", "c5", "m6i", "m5"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Analyze job for MPI characteristics
			err := scheduler.AnalyzeJob(ctx, tc.job)
			require.NoError(t, err, "MPI analysis should succeed")

			// Verify MPI detection
			assert.Equal(t, tc.expectedMPI, tc.job.IsMPIJob, "MPI detection mismatch")

			// Determine instance requirements
			instanceReq := scheduler.DetermineInstanceRequirements(tc.job)
			require.NotNil(t, instanceReq, "Instance requirements should be determined")

			// Verify EFA requirements
			assert.Equal(t, tc.expectedEFARequired, instanceReq.RequiresEFA, "EFA requirement mismatch")
			assert.Equal(t, tc.expectedEFAPreferred, instanceReq.EFAPreferred, "EFA preference mismatch")

			// Verify HPC optimization
			assert.Equal(t, tc.expectedHPCOptimized, instanceReq.HPCOptimized, "HPC optimization mismatch")

			// Verify placement group configuration
			if tc.expectedPlacementGroup {
				assert.NotEmpty(t, instanceReq.PlacementGroupType, "Placement group should be configured")
				if tc.job.IsMPIJob && tc.job.Resources.Nodes >= 2 {
					assert.Equal(t, "cluster", instanceReq.PlacementGroupType, "MPI jobs should use cluster placement groups")
				}
			} else {
				assert.Empty(t, instanceReq.PlacementGroupType, "Placement group should not be configured")
			}

			// Verify instance family selection
			assert.NotEmpty(t, instanceReq.InstanceFamilies, "Instance families should be selected")

			// Check that expected instance types are included
			instanceFamilySet := make(map[string]bool)
			for _, family := range instanceReq.InstanceFamilies {
				instanceFamilySet[family] = true
			}

			foundExpected := 0
			for _, expected := range tc.expectedInstanceTypes {
				if instanceFamilySet[expected] {
					foundExpected++
				}
			}

			assert.Greater(t, foundExpected, 0, "At least one expected instance type should be present")

			// Log results for debugging
			t.Logf("Job: %s", tc.job.Name)
			t.Logf("  MPI: %v", tc.job.IsMPIJob)
			t.Logf("  EFA Required: %v", instanceReq.RequiresEFA)
			t.Logf("  EFA Preferred: %v", instanceReq.EFAPreferred)
			t.Logf("  HPC Optimized: %v", instanceReq.HPCOptimized)
			t.Logf("  Placement Group: %s", instanceReq.PlacementGroupType)
			t.Logf("  Instance Families: %v", instanceReq.InstanceFamilies)
		})
	}
}

// TestEFAInstanceAvailability tests EFA instance family validation
func TestEFAInstanceAvailability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name           string
		instanceFamily string
		expectedEFA    bool
		expectedGen    int
	}{
		{"hpc7a_supports_efa", "hpc7a", true, 2},
		{"hpc6id_supports_efa", "hpc6id", true, 2},
		{"hpc6a_supports_efa", "hpc6a", true, 2},
		{"c6in_supports_efa", "c6in", true, 2},
		{"c6i_supports_efa", "c6i", true, 2},
		{"c5n_supports_efa", "c5n", true, 1},
		{"r6i_supports_efa", "r6i", true, 2},
		{"r5n_supports_efa", "r5n", true, 1},
		{"c5_no_efa", "c5", false, 0},
		{"m5_no_efa", "m5", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			supportsEFA, generation := types.IsEFASupported(tt.instanceFamily)
			assert.Equal(t, tt.expectedEFA, supportsEFA, "EFA support mismatch")
			assert.Equal(t, tt.expectedGen, generation, "EFA generation mismatch")
		})
	}
}

// TestPerformanceScaling tests that the MPI analyzer performs well with large jobs
func TestPerformanceScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	logger := zaptest.NewLogger(t)
	scheduler := scheduler.NewMPIScheduler(logger)

	// Create a large-scale job
	job := &types.SlurmJob{
		JobID:  "perf-test-001",
		Name:   "large-scale-simulation",
		Script: "#!/bin/bash\n#SBATCH --ntasks=1024\nmpirun -np 1024 ./large_simulation",
		Resources: types.ResourceSpec{
			Nodes:       64,
			CPUsPerNode: 16,
			MemoryMB:    65536,
		},
		NodeList: make([]string, 64),
	}

	// Generate node list
	for i := 0; i < 64; i++ {
		job.NodeList[i] = fmt.Sprintf("aws-hpc-%03d", i)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()

	err := scheduler.AnalyzeJob(ctx, job)
	require.NoError(t, err)

	instanceReq := scheduler.DetermineInstanceRequirements(job)
	require.NotNil(t, instanceReq)

	elapsed := time.Since(start)

	// Performance assertions
	assert.Less(t, elapsed, 1*time.Second, "MPI analysis should complete within 1 second for large jobs")
	assert.True(t, job.IsMPIJob, "Large MPI job should be detected")
	assert.True(t, instanceReq.RequiresEFA, "Large MPI job should require EFA")
	assert.True(t, instanceReq.HPCOptimized, "Large MPI job should use HPC instances")

	t.Logf("Performance test completed in %v for %d nodes", elapsed, job.Resources.Nodes)
}

// TestRealWorldScenarios tests scenarios based on actual HPC workloads
func TestRealWorldScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	scheduler := scheduler.NewMPIScheduler(logger)
	ctx := context.Background()

	scenarios := []struct {
		name        string
		description string
		job         *types.SlurmJob
		assertions  func(t *testing.T, job *types.SlurmJob, req *types.InstanceRequirements)
	}{
		{
			name:        "climate_modeling",
			description: "Large-scale climate modeling with WRF",
			job: &types.SlurmJob{
				JobID: "climate-001",
				Name:  "wrf-climate-model",
				Script: `#!/bin/bash
#SBATCH --job-name=wrf-climate
#SBATCH --nodes=32
#SBATCH --ntasks-per-node=16
#SBATCH --constraint=efa-required
module load WRF/4.3.1
mpirun -np 512 ./wrf.exe`,
				Resources: types.ResourceSpec{
					Nodes:       32,
					CPUsPerNode: 16,
					MemoryMB:    65536,
				},
				Constraints: types.JobConstraints{
					Features: []string{"efa-required"},
				},
			},
			assertions: func(t *testing.T, job *types.SlurmJob, req *types.InstanceRequirements) {
				assert.True(t, job.IsMPIJob)
				assert.True(t, req.RequiresEFA)
				assert.True(t, req.HPCOptimized)
				assert.Equal(t, "cluster", req.PlacementGroupType)
				assert.Contains(t, req.InstanceFamilies, "hpc7a")
			},
		},
		{
			name:        "computational_chemistry",
			description: "Quantum chemistry calculation with Gaussian",
			job: &types.SlurmJob{
				JobID: "chem-001",
				Name:  "gaussian-dft",
				Script: `#!/bin/bash
#SBATCH --job-name=gaussian
#SBATCH --nodes=8
#SBATCH --ntasks-per-node=8
#SBATCH --mem=128GB
g16 -p=64 -m=1024GB input.com`,
				Resources: types.ResourceSpec{
					Nodes:       8,
					CPUsPerNode: 8,
					MemoryMB:    131072, // 128GB
				},
			},
			assertions: func(t *testing.T, job *types.SlurmJob, req *types.InstanceRequirements) {
				// Gaussian typically doesn't use MPI but benefits from high memory
				assert.False(t, job.IsMPIJob)
				assert.False(t, req.RequiresEFA)
				// Should select memory-optimized instances due to high memory requirement
				hasMemoryOptimized := false
				for _, family := range req.InstanceFamilies {
					if strings.HasPrefix(family, "r") || strings.HasPrefix(family, "x") {
						hasMemoryOptimized = true
						break
					}
				}
				assert.True(t, hasMemoryOptimized, "Should select memory-optimized instances")
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Testing scenario: %s", scenario.description)

			err := scheduler.AnalyzeJob(ctx, scenario.job)
			require.NoError(t, err, "Job analysis should succeed")

			req := scheduler.DetermineInstanceRequirements(scenario.job)
			require.NotNil(t, req, "Instance requirements should be determined")

			scenario.assertions(t, scenario.job, req)

			t.Logf("Scenario completed successfully: %s", scenario.name)
		})
	}
}
