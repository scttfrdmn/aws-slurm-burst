package scheduler

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// MPIScheduler handles MPI-specific job analysis and scheduling requirements
type MPIScheduler struct {
	logger    *zap.Logger
	detectors []MPIDetector
}

type MPIDetector interface {
	Name() string
	Detect(job *types.SlurmJob) (bool, float64) // returns (isMPI, confidence)
	RequiredTopology(job *types.SlurmJob) types.NetworkTopology
}

type Confidence float64

const (
	ConfidenceLow    Confidence = 0.3
	ConfidenceMedium Confidence = 0.6
	ConfidenceHigh   Confidence = 0.9
)

func NewMPIScheduler(logger *zap.Logger) *MPIScheduler {
	return &MPIScheduler{
		logger: logger,
		detectors: []MPIDetector{
			&TaskCountDetector{},
			&ScriptContentDetector{},
			&ApplicationDetector{},
			&EnvironmentDetector{},
		},
	}
}

// AnalyzeJob determines if a job is MPI and its requirements
func (m *MPIScheduler) AnalyzeJob(ctx context.Context, job *types.SlurmJob) error {
	m.logger.Debug("Analyzing job for MPI characteristics", zap.String("job_id", job.JobID))

	var maxConfidence float64
	var requiredTopology types.NetworkTopology = types.TopologyAny

	for _, detector := range m.detectors {
		isMPI, confidence := detector.Detect(job)
		m.logger.Debug("MPI detector result",
			zap.String("detector", detector.Name()),
			zap.Bool("is_mpi", isMPI),
			zap.Float64("confidence", confidence))

		if isMPI && confidence > maxConfidence {
			maxConfidence = confidence
			requiredTopology = detector.RequiredTopology(job)
		}
	}

	// Consider it MPI if confidence > 0.5
	job.IsMPIJob = maxConfidence > 0.5
	job.MPITopology = requiredTopology

	if job.IsMPIJob {
		m.calculateMPIProcesses(job)
		m.logger.Info("Job identified as MPI",
			zap.String("job_id", job.JobID),
			zap.Float64("confidence", maxConfidence),
			zap.String("topology", string(requiredTopology)),
			zap.Int("processes", job.MPIProcesses))
	}

	return nil
}

func (m *MPIScheduler) calculateMPIProcesses(job *types.SlurmJob) {
	// Default: assume 1 process per CPU
	job.MPIProcesses = job.Resources.Nodes * job.Resources.CPUsPerNode
}

// DetermineInstanceRequirements analyzes an MPI job to determine optimal instance requirements
func (m *MPIScheduler) DetermineInstanceRequirements(job *types.SlurmJob) *types.InstanceRequirements {
	req := &types.InstanceRequirements{
		MinCPUs:     job.Resources.CPUsPerNode,
		MinMemoryMB: job.Resources.MemoryMB,
		GPUs:        job.Resources.GPUs,
		GPUType:     job.Resources.GPUType,
		NetworkTopology: job.MPITopology,
	}

	if !job.IsMPIJob {
		// Non-MPI jobs don't need EFA
		req.RequiresEFA = false
		req.EFAPreferred = false
		req.HPCOptimized = false
		return req
	}

	// MPI-specific requirements
	efaCapability := m.determineEFARequirement(job)
	switch efaCapability {
	case types.EFARequired:
		req.RequiresEFA = true
		req.EFAPreferred = true
	case types.EFAPreferred:
		req.RequiresEFA = false
		req.EFAPreferred = true
	case types.EFAOptional:
		req.RequiresEFA = false
		req.EFAPreferred = false
	case types.EFADisabled:
		req.RequiresEFA = false
		req.EFAPreferred = false
	}

	// Determine optimal instance families for MPI
	req.InstanceFamilies = m.selectOptimalInstanceFamilies(job, efaCapability)
	req.HPCOptimized = m.shouldUseHPCInstances(job)
	req.EnhancedNetworking = true // Always enable for MPI

	// Placement group for MPI jobs
	if job.Resources.Nodes >= 2 {
		switch job.MPITopology {
		case types.TopologyCluster:
			req.PlacementGroupType = "cluster"
		case types.TopologySpread:
			req.PlacementGroupType = "spread"
		case types.TopologyPartition:
			req.PlacementGroupType = "partition"
		}
	}

	m.logger.Info("Determined MPI instance requirements",
		zap.String("job_id", job.JobID),
		zap.Bool("requires_efa", req.RequiresEFA),
		zap.Bool("efa_preferred", req.EFAPreferred),
		zap.Bool("hpc_optimized", req.HPCOptimized),
		zap.Strings("instance_families", req.InstanceFamilies),
		zap.String("placement_group", req.PlacementGroupType))

	return req
}

// determineEFARequirement analyzes job characteristics to determine EFA needs
func (m *MPIScheduler) determineEFARequirement(job *types.SlurmJob) types.EFACapability {
	// Check for explicit EFA configuration in constraints
	for _, feature := range job.Constraints.Features {
		switch strings.ToLower(feature) {
		case "efa", "efa-required":
			return types.EFARequired
		case "no-efa", "efa-disabled":
			return types.EFADisabled
		case "efa-preferred":
			return types.EFAPreferred
		}
	}

	// Heuristics based on job characteristics
	nodeCount := job.Resources.Nodes
	processes := job.MPIProcesses

	// Large-scale MPI jobs almost always benefit from EFA
	if nodeCount >= 16 || processes >= 64 {
		return types.EFARequired
	}

	// Medium-scale jobs prefer EFA
	if nodeCount >= 4 || processes >= 16 {
		return types.EFAPreferred
	}

	// Small jobs can use EFA but don't need it
	if nodeCount >= 2 {
		return types.EFAOptional
	}

	// Single-node jobs don't need EFA
	return types.EFADisabled
}

// selectOptimalInstanceFamilies chooses the best instance families for the job
func (m *MPIScheduler) selectOptimalInstanceFamilies(job *types.SlurmJob, efaCapability types.EFACapability) []string {
	var families []string

	// If EFA is required or preferred, prioritize EFA-capable instances
	if efaCapability == types.EFARequired || efaCapability == types.EFAPreferred {
		// HPC-optimized instances first for large MPI jobs
		if job.Resources.Nodes >= 8 {
			families = append(families, "hpc7a", "hpc6id", "hpc6a")
		}

		// Compute-optimized with EFA
		memoryPerCPU := job.Resources.MemoryMB / job.Resources.CPUsPerNode
		if memoryPerCPU <= 4096 { // CPU-bound jobs
			families = append(families, "c6in", "c6i", "c5n")
		} else { // Memory-bound jobs
			families = append(families, "r6i", "r5n")
		}
	} else {
		// Standard instances without EFA requirement
		families = append(families, "c6i", "c5", "m6i", "m5", "r6i", "r5")
	}

	// GPU jobs
	if job.Resources.GPUs > 0 {
		families = append([]string{"p4d", "p3dn", "g4dn"}, families...)
	}

	return families
}

// shouldUseHPCInstances determines if HPC-optimized instances should be preferred
func (m *MPIScheduler) shouldUseHPCInstances(job *types.SlurmJob) bool {
	// Large-scale MPI jobs benefit from HPC instances
	if job.Resources.Nodes >= 8 {
		return true
	}

	// Check for known HPC applications
	script := strings.ToLower(job.Script)
	name := strings.ToLower(job.Name)

	hpcApplications := []string{
		"gromacs", "lammps", "namd", "quantum", "espresso",
		"vasp", "abinit", "cp2k", "nwchem", "gamess",
		"wrf", "openfoam", "ansys", "fluent",
	}

	for _, app := range hpcApplications {
		if strings.Contains(script, app) || strings.Contains(name, app) {
			return true
		}
	}

	return false
}

// TaskCountDetector identifies MPI jobs by task count vs node count ratio
type TaskCountDetector struct{}

func (t *TaskCountDetector) Name() string { return "task_count" }

func (t *TaskCountDetector) Detect(job *types.SlurmJob) (bool, float64) {
	// If ntasks > nodes * cpus_per_node, likely MPI
	totalCPUs := job.Resources.Nodes * job.Resources.CPUsPerNode

	// Look for ntasks in job script or environment
	ntasks := t.extractNTasks(job)
	if ntasks == 0 {
		return false, 0.0
	}

	if ntasks > job.Resources.Nodes && ntasks <= totalCPUs {
		return true, float64(ConfidenceHigh)
	} else if ntasks == job.Resources.Nodes {
		return false, 0.1 // Single task per node, unlikely MPI
	}

	return false, 0.0
}

func (t *TaskCountDetector) RequiredTopology(job *types.SlurmJob) types.NetworkTopology {
	// High task count suggests need for low-latency communication
	if job.Resources.Nodes >= 4 {
		return types.TopologyCluster
	}
	return types.TopologyAny
}

func (t *TaskCountDetector) extractNTasks(job *types.SlurmJob) int {
	// Parse #SBATCH --ntasks or environment variables
	patterns := []string{
		`#SBATCH\s+--ntasks[=\s](\d+)`,
		`#SBATCH\s+-n[=\s](\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(job.Script); len(matches) > 1 {
			if n, err := strconv.Atoi(matches[1]); err == nil {
				return n
			}
		}
	}

	// Check environment
	if ntasks, exists := job.Environment["SLURM_NTASKS"]; exists {
		if n, err := strconv.Atoi(ntasks); err == nil {
			return n
		}
	}

	return 0
}

// ScriptContentDetector identifies MPI by examining job script content
type ScriptContentDetector struct{}

func (s *ScriptContentDetector) Name() string { return "script_content" }

func (s *ScriptContentDetector) Detect(job *types.SlurmJob) (bool, float64) {
	if job.Script == "" {
		return false, 0.0
	}

	script := strings.ToLower(job.Script)

	// High confidence indicators
	highConfidencePatterns := []string{
		"mpirun", "mpiexec", "srun.*-n\\s+\\d+",
		"#include.*<mpi.h>", "mpi_init", "mpi_finalize",
	}

	for _, pattern := range highConfidencePatterns {
		if matched, _ := regexp.MatchString(pattern, script); matched {
			return true, float64(ConfidenceHigh)
		}
	}

	// Medium confidence indicators
	mediumConfidencePatterns := []string{
		"openmpi", "mpich", "intel.*mpi", "parallel.*computation",
	}

	for _, pattern := range mediumConfidencePatterns {
		if matched, _ := regexp.MatchString(pattern, script); matched {
			return true, float64(ConfidenceMedium)
		}
	}

	return false, 0.0
}

func (s *ScriptContentDetector) RequiredTopology(job *types.SlurmJob) types.NetworkTopology {
	// Script-based detection suggests intentional MPI usage
	return types.TopologyCluster
}

// ApplicationDetector identifies known MPI applications
type ApplicationDetector struct{}

func (a *ApplicationDetector) Name() string { return "application" }

var mpiApplications = map[string]float64{
	"gromacs":     float64(ConfidenceHigh),
	"lammps":      float64(ConfidenceHigh),
	"namd":        float64(ConfidenceHigh),
	"quantum":     float64(ConfidenceHigh),
	"espresso":    float64(ConfidenceHigh),
	"abinit":      float64(ConfidenceHigh),
	"vasp":        float64(ConfidenceHigh),
	"amber":       float64(ConfidenceMedium),
	"blast":       float64(ConfidenceLow),
}

func (a *ApplicationDetector) Detect(job *types.SlurmJob) (bool, float64) {
	script := strings.ToLower(job.Script)
	name := strings.ToLower(job.Name)

	for app, confidence := range mpiApplications {
		if strings.Contains(script, app) || strings.Contains(name, app) {
			return true, confidence
		}
	}

	return false, 0.0
}

func (a *ApplicationDetector) RequiredTopology(job *types.SlurmJob) types.NetworkTopology {
	return types.TopologyCluster
}

// EnvironmentDetector checks environment variables for MPI indicators
type EnvironmentDetector struct{}

func (e *EnvironmentDetector) Name() string { return "environment" }

func (e *EnvironmentDetector) Detect(job *types.SlurmJob) (bool, float64) {
	mpiEnvVars := []string{
		"OMPI_", "MPI_", "I_MPI_", "MPICH_",
	}

	for key := range job.Environment {
		for _, prefix := range mpiEnvVars {
			if strings.HasPrefix(key, prefix) {
				return true, float64(ConfidenceMedium)
			}
		}
	}

	return false, 0.0
}

func (e *EnvironmentDetector) RequiredTopology(job *types.SlurmJob) types.NetworkTopology {
	return types.TopologyCluster
}