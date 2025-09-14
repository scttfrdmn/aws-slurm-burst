# AWS Slurm Burst Architecture

## Overview

`aws-slurm-burst` is a next-generation plugin system for Slurm that provides intelligent workload bursting to AWS with first-class MPI support, EFA integration, and cost optimization through ABSA (aws-slurm-burst-advisor) coordination.

## Key Design Principles

1. **MPI-First**: Designed from the ground up to handle tightly-coupled MPI workloads
2. **Cost-Aware**: Integrates with ABSA for intelligent cost/performance trade-offs
3. **Performance-Optimized**: Leverages EFA, HPC instances, and placement groups
4. **Modern Go**: Built with Go 1.23+ for high performance and maintainability

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                              Slurm Cluster                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────────┐    ┌──────────────────┐                      │
│  │   slurmctld      │    │   User Jobs      │                      │
│  │                  │    │                  │                      │
│  │ ┌──────────────┐ │    │ ┌──────────────┐ │                      │
│  │ │ResumeProgram │ │    │ │#SBATCH --efa │ │                      │
│  │ │SuspendProgram│ │    │ │mpirun ...    │ │                      │
│  │ └──────────────┘ │    │ └──────────────┘ │                      │
│  └──────────────────┘    └──────────────────┘                      │
│           │                        │                               │
│           └────────────┬───────────┘                               │
└─────────────────────────────────────────────────────────────────────┘
                         │
          ┌──────────────┴──────────────┐
          │                             │
          ▼                             ▼
┌─────────────────────┐         ┌─────────────────────┐
│       ABSA          │         │  aws-slurm-burst    │
│  (Cost Advisor)     │         │   (This Project)    │
│                     │         │                     │
│ ┌─────────────────┐ │         │ ┌─────────────────┐ │
│ │Queue Analysis   │ │◄────────┤ │Job Analyzer     │ │
│ │Cost Modeling    │ │         │ │MPI Detector     │ │
│ │Burst Decisions  │ │────────►│ │EFA Scheduler    │ │
│ └─────────────────┘ │         │ │Instance Matcher │ │
└─────────────────────┘         │ └─────────────────┘ │
                                └─────────────────────┘
                                          │
                                          ▼
                                ┌─────────────────────┐
                                │      AWS APIs       │
                                │                     │
                                │ ┌─────────────────┐ │
                                │ │EC2 Fleet        │ │
                                │ │Placement Groups │ │
                                │ │EFA Configuration│ │
                                │ │Pricing API      │ │
                                │ └─────────────────┘ │
                                └─────────────────────┘
                                          │
                                          ▼
                                ┌─────────────────────┐
                                │   EC2 Instances     │
                                │                     │
                                │ ┌─────────────────┐ │
                                │ │HPC Instances    │ │
                                │ │EFA-enabled      │ │
                                │ │Cluster PG       │ │
                                │ │MPI-optimized    │ │
                                │ └─────────────────┘ │
                                └─────────────────────┘
```

## Component Details

### MPI Scheduler (`internal/scheduler/mpi.go`)

**Responsibility**: Analyze jobs to determine MPI requirements and optimal instance configuration.

**Key Features**:
- Multi-detector MPI identification (script analysis, task count, known applications)
- EFA requirement determination (required/preferred/optional/disabled)
- Instance family selection based on workload characteristics
- Placement group configuration for optimal network topology

**Decision Logic**:
```
Job Analysis → MPI Detection → EFA Requirements → Instance Families → Placement Groups
```

### ABSA Integration (`internal/absa/integration.go`)

**Responsibility**: Interface with aws-slurm-burst-advisor for cost-optimized decision making.

**Integration Points**:
- **Burst Decision**: Should this job run on AWS vs. on-premises?
- **Cost Constraints**: Maximum acceptable cost per hour
- **Instance Recommendations**: ABSA-suggested instance types
- **Urgency Assessment**: Time-sensitive jobs get priority treatment

### AWS Client (`internal/aws/`)

**Responsibility**: Manage AWS resources including instances, placement groups, and EFA configuration.

**Key Operations**:
- **Fleet Management**: Create EC2 fleets with instance diversification
- **Placement Groups**: Create and manage cluster/partition/spread placement groups
- **EFA Configuration**: Enable EFA on supported instances
- **Tagging**: Instance tagging for Slurm integration
- **Monitoring**: Instance health and cost tracking

### Slurm Integration (`internal/slurm/`)

**Responsibility**: Interface with Slurm daemons and parse job information.

**Key Features**:
- Job script parsing for resource requirements
- Node list expansion and management
- SBATCH directive analysis
- Node state updates

## MPI Workflow Example

1. **Job Submission**:
   ```bash
   sbatch --partition=aws-burst --nodes=4 --constraint=efa-preferred mpi-job.sbatch
   ```

2. **Job Analysis**:
   - Parse script for `mpirun` commands
   - Detect ntasks > nodes (MPI indicator)
   - Identify known MPI applications (GROMACS, LAMMPS)
   - Determine EFA requirements based on scale and constraints

3. **ABSA Consultation**:
   - Send job requirements to ABSA
   - Receive cost analysis and burst recommendation
   - Get instance type suggestions

4. **Instance Selection**:
   - Filter for EFA-capable instances if required
   - Prioritize HPC families for large-scale jobs
   - Apply cost constraints from ABSA

5. **Resource Provisioning**:
   - Create cluster placement group for low-latency communication
   - Launch EC2 Fleet with EFA-enabled instances
   - Configure instances with proper MPI stack

6. **Job Execution**:
   - Slurm starts job on provisioned nodes
   - MPI benefits from EFA low-latency networking
   - Job completes with optimal performance

## Configuration Structure

```yaml
aws:
  region: us-east-1
  credentials_profile: default

slurm:
  bin_path: /usr/bin
  config_path: /etc/slurm/slurm.conf

absa:
  enabled: true
  command: /usr/local/bin/absa
  config_path: /etc/absa/config.yaml

mpi:
  efa_default: preferred  # required/preferred/optional/disabled
  hpc_instances_threshold: 8  # nodes
  placement_group_threshold: 2  # nodes

logging:
  level: info
  format: json
  file: /var/log/slurm/aws-burst.log
```

## Performance Considerations

### EFA Benefits
- **Latency**: 15.5μs one-way MPI latency (vs 50-100μs TCP)
- **Bandwidth**: Up to 300 Gbps on hpc7a instances
- **CPU Efficiency**: OS-bypass reduces CPU overhead

### Instance Selection Strategy
1. **Large MPI Jobs (≥16 nodes)**: HPC instances (hpc7a, hpc6id, hpc6a) with EFA required
2. **Medium Jobs (4-15 nodes)**: Compute-optimized with EFA (c6in, c6i, c5n)
3. **Small Jobs (2-3 nodes)**: Standard instances with optional EFA
4. **Single-node**: No EFA requirement

### Cost Optimization
- **Spot Integration**: Use spot instances where MPI fault tolerance allows
- **Mixed Pricing**: Combine spot and on-demand for critical jobs
- **Right-sizing**: Match instance specs to job requirements
- **ABSA Guidance**: Use cost models for optimal decisions

## Security Considerations

- **IAM Roles**: Minimal permissions for EC2 operations
- **Network Security**: VPC and security group configuration
- **Instance Security**: Regular AMI updates and security patches
- **Secrets Management**: AWS credentials via IAM roles, not embedded keys