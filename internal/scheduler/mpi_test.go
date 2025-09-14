package scheduler

import (
	"context"
	"strings"
	"testing"

	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestMPIScheduler_AnalyzeJob(t *testing.T) {
	logger := zaptest.NewLogger(t)
	scheduler := NewMPIScheduler(logger)

	tests := []struct {
		name        string
		job         *types.SlurmJob
		expectedMPI bool
		expectedEFA types.EFACapability
	}{
		{
			name: "mpirun in script",
			job: &types.SlurmJob{
				JobID:   "test1",
				Name:    "test-job",
				Script:  "#!/bin/bash\nmpirun -np 8 ./myapp",
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 4,
				},
			},
			expectedMPI: true,
			expectedEFA: types.EFAOptional, // 2 nodes = small job, only optional EFA
		},
		{
			name: "GROMACS application",
			job: &types.SlurmJob{
				JobID:   "test2",
				Name:    "gromacs-simulation",
				Script:  "#!/bin/bash\ngmx_mpi mdrun -deffnm prod",
				Resources: types.ResourceSpec{
					Nodes:       4,
					CPUsPerNode: 8,
				},
			},
			expectedMPI: true,
			expectedEFA: types.EFAPreferred,
		},
		{
			name: "large scale MPI",
			job: &types.SlurmJob{
				JobID:   "test3",
				Name:    "large-mpi",
				Script:  "#!/bin/bash\nmpiexec -n 128 ./app",
				Resources: types.ResourceSpec{
					Nodes:       16,
					CPUsPerNode: 8,
				},
			},
			expectedMPI: true,
			expectedEFA: types.EFARequired,
		},
		{
			name: "non-MPI job",
			job: &types.SlurmJob{
				JobID:   "test4",
				Name:    "single-thread",
				Script:  "#!/bin/bash\n./sequential_app",
				Resources: types.ResourceSpec{
					Nodes:       1,
					CPUsPerNode: 4,
				},
			},
			expectedMPI: false,
			expectedEFA: types.EFADisabled,
		},
		{
			name: "EFA explicitly disabled",
			job: &types.SlurmJob{
				JobID:   "test5",
				Name:    "mpi-no-efa",
				Script:  "#!/bin/bash\nmpirun -np 8 ./myapp",
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 4,
				},
				Constraints: types.JobConstraints{
					Features: []string{"no-efa"},
				},
			},
			expectedMPI: true,
			expectedEFA: types.EFADisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := scheduler.AnalyzeJob(ctx, tt.job)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedMPI, tt.job.IsMPIJob, "MPI detection mismatch")

			// Test instance requirements
			req := scheduler.DetermineInstanceRequirements(tt.job)
			require.NotNil(t, req)

			switch tt.expectedEFA {
			case types.EFARequired:
				assert.True(t, req.RequiresEFA, "EFA should be required")
			case types.EFAPreferred:
				assert.True(t, req.EFAPreferred, "EFA should be preferred")
			case types.EFADisabled:
				assert.False(t, req.RequiresEFA, "EFA should not be required")
				assert.False(t, req.EFAPreferred, "EFA should not be preferred")
			}
		})
	}
}

func TestTaskCountDetector(t *testing.T) {
	detector := &TaskCountDetector{}

	tests := []struct {
		name           string
		job            *types.SlurmJob
		expectedMPI    bool
		expectedConf   float64
	}{
		{
			name: "more tasks than nodes",
			job: &types.SlurmJob{
				Script: "#SBATCH --ntasks=8\n#SBATCH --nodes=2",
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 4,
				},
			},
			expectedMPI:  true,
			expectedConf: float64(ConfidenceHigh),
		},
		{
			name: "single task per node",
			job: &types.SlurmJob{
				Script: "#SBATCH --ntasks=2\n#SBATCH --nodes=2",
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 4,
				},
			},
			expectedMPI:  false,
			expectedConf: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isMPI, confidence := detector.Detect(tt.job)
			assert.Equal(t, tt.expectedMPI, isMPI)
			assert.Equal(t, tt.expectedConf, confidence)
		})
	}
}

func TestScriptContentDetector(t *testing.T) {
	detector := &ScriptContentDetector{}

	tests := []struct {
		name         string
		script       string
		expectedMPI  bool
		expectedConf float64
	}{
		{
			name:         "mpirun command",
			script:       "mpirun -np 8 ./app",
			expectedMPI:  true,
			expectedConf: float64(ConfidenceHigh),
		},
		{
			name:         "mpi include",
			script:       "#include <mpi.h>\nint main() { MPI_Init(); }",
			expectedMPI:  true,
			expectedConf: float64(ConfidenceHigh),
		},
		{
			name:         "openmpi reference",
			script:       "module load openmpi\n./app",
			expectedMPI:  true,
			expectedConf: float64(ConfidenceMedium),
		},
		{
			name:         "no MPI indicators",
			script:       "#!/bin/bash\n./sequential_app",
			expectedMPI:  false,
			expectedConf: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &types.SlurmJob{Script: tt.script}
			isMPI, confidence := detector.Detect(job)
			assert.Equal(t, tt.expectedMPI, isMPI)
			assert.Equal(t, tt.expectedConf, confidence)
		})
	}
}

func TestApplicationDetector(t *testing.T) {
	detector := &ApplicationDetector{}

	tests := []struct {
		name         string
		jobName      string
		script       string
		expectedMPI  bool
		expectedConf float64
	}{
		{
			name:         "GROMACS in name",
			jobName:      "gromacs-simulation",
			script:       "",
			expectedMPI:  true,
			expectedConf: float64(ConfidenceHigh),
		},
		{
			name:         "LAMMPS in script",
			jobName:      "test",
			script:       "lammps -in input.lmp",
			expectedMPI:  true,
			expectedConf: float64(ConfidenceHigh),
		},
		{
			name:         "unknown application",
			jobName:      "my-app",
			script:       "./unknown_app",
			expectedMPI:  false,
			expectedConf: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &types.SlurmJob{
				Name:   tt.jobName,
				Script: tt.script,
			}
			isMPI, confidence := detector.Detect(job)
			assert.Equal(t, tt.expectedMPI, isMPI)
			assert.Equal(t, tt.expectedConf, confidence)
		})
	}
}

func TestInstanceFamilySelection(t *testing.T) {
	logger := zaptest.NewLogger(t)
	scheduler := NewMPIScheduler(logger)

	tests := []struct {
		name             string
		job              *types.SlurmJob
		efaCapability    types.EFACapability
		expectedFamilies []string
		shouldContainHPC bool
	}{
		{
			name: "large MPI job with EFA required",
			job: &types.SlurmJob{
				Resources: types.ResourceSpec{
					Nodes:       16,
					CPUsPerNode: 8,
					MemoryMB:    8192,
				},
			},
			efaCapability:    types.EFARequired,
			expectedFamilies: []string{"hpc7a", "hpc6id", "hpc6a", "c6in", "c6i", "c5n"},
			shouldContainHPC: true,
		},
		{
			name: "memory intensive job",
			job: &types.SlurmJob{
				Resources: types.ResourceSpec{
					Nodes:       4,
					CPUsPerNode: 4,
					MemoryMB:    32768, // 32GB per node = 8GB per core (> 4GB threshold)
				},
			},
			efaCapability:    types.EFAPreferred,
			expectedFamilies: []string{"r6i", "r5n"},
			shouldContainHPC: false,
		},
		{
			name: "GPU job",
			job: &types.SlurmJob{
				Resources: types.ResourceSpec{
					Nodes:       2,
					CPUsPerNode: 8,
					MemoryMB:    8192,
					GPUs:        4,
				},
			},
			efaCapability:    types.EFAOptional,
			expectedFamilies: []string{"p4d", "p3dn", "g4dn"},
			shouldContainHPC: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			families := scheduler.selectOptimalInstanceFamilies(tt.job, tt.efaCapability)

			if tt.shouldContainHPC {
				hasHPC := false
				for _, family := range families {
					if strings.HasPrefix(family, "hpc") {
						hasHPC = true
						break
					}
				}
				assert.True(t, hasHPC, "Expected HPC instance families for large job")
			}

			// Check that expected families are present
			for _, expected := range tt.expectedFamilies {
				found := false
				for _, family := range families {
					if family == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected family %s not found in %v", expected, families)
				}
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkMPIAnalysis(b *testing.B) {
	logger := zaptest.NewLogger(b)
	scheduler := NewMPIScheduler(logger)
	ctx := context.Background()

	job := &types.SlurmJob{
		JobID:   "benchmark",
		Name:    "gromacs-md",
		Script:  "#!/bin/bash\n#SBATCH --ntasks=32\nmpirun -np 32 gmx_mpi mdrun -deffnm prod",
		Resources: types.ResourceSpec{
			Nodes:       4,
			CPUsPerNode: 8,
			MemoryMB:    16384,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := scheduler.AnalyzeJob(ctx, job)
		if err != nil {
			b.Fatal(err)
		}
		scheduler.DetermineInstanceRequirements(job)
	}
}