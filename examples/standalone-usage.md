# Usage Examples

## Standalone Mode (Like Original Plugin)

Uses static configuration from YAML file, similar to the original AWS Slurm plugin:

```bash
# Basic usage - uses config.yaml instance types
aws-slurm-burst-resume aws-cpu-[001-004]

# With specific config file
aws-slurm-burst-resume aws-gpu-[001-002] --config=/etc/slurm/custom-burst.yaml

# Dry run to see what would happen
aws-slurm-burst-resume aws-hpc-[001-008] --dry-run
```

**Configuration-driven behavior**:
- Instance types defined in `config.yaml`
- Spot/on-demand from `purchasing_option`
- No MPI analysis (treats all jobs as embarrassingly parallel)
- No cost optimization (uses configured settings)

## ASBA Mode (Intelligent)

Uses execution plan from aws-slurm-burst-advisor for optimized decisions:

```bash
# Step 1: ASBA analyzes job and generates execution plan
asba analyze job.sbatch --output=execution-plan.json

# Step 2: Execute the plan
aws-slurm-burst-resume aws-hpc-[001-016] --execution-plan=execution-plan.json

# Or in one command (if ASBA supports it)
asba analyze job.sbatch --execute-with=aws-slurm-burst-resume aws-hpc-[001-016]
```

**ASBA-driven behavior**:
- Instance types selected by ASBA analysis
- MPI detection and EFA optimization
- Cost/performance trade-offs
- Intelligent spot instance usage
- Gang scheduling for MPI workloads

## Hybrid Workflows

### Research Workflow
```bash
# Quick test with defaults
aws-slurm-burst-resume test-node-001

# Production run with optimization
asba analyze production-job.sbatch --output=plan.json
aws-slurm-burst-resume production-[001-032] --execution-plan=plan.json
```

### Development Workflow
```bash
# Development: fast, cheap instances
aws-slurm-burst-resume dev-[001-004] --config=dev-config.yaml

# Production: ASBA-optimized
asba analyze app.sbatch --budget=50.00 --deadline=2h --output=prod-plan.json
aws-slurm-burst-resume prod-[001-064] --execution-plan=prod-plan.json
```

## Configuration for Standalone Mode

```yaml
# config.yaml - Traditional static configuration
aws:
  region: us-east-1

slurm:
  partitions:
    - partition_name: aws
      node_groups:
        - node_group_name: cpu
          max_nodes: 20
          purchasing_option: spot
          launch_template_overrides:
            - instance_type: c5.large
            - instance_type: c5.xlarge
          subnet_ids:
            - subnet-12345678
        - node_group_name: gpu
          max_nodes: 10
          purchasing_option: on-demand
          launch_template_overrides:
            - instance_type: p3.2xlarge
          subnet_ids:
            - subnet-12345678
```

## ASBA Communication Patterns

### File-based (Current)
```bash
asba analyze → execution-plan.json → aws-slurm-burst-resume
```

### Environment Variables
```bash
# ASBA sets environment variables
export ASBA_INSTANCE_TYPES="hpc7a.2xlarge,c6i.2xlarge"
export ASBA_REQUIRES_EFA="true"
export ASBA_PLACEMENT_GROUP="cluster"
aws-slurm-burst-resume aws-hpc-[001-008]
```

### Named Pipes / Unix Sockets
```bash
# Real-time communication
asba daemon --socket=/tmp/asba.sock &
aws-slurm-burst-resume aws-hpc-[001-008] --asba-socket=/tmp/asba.sock
```

### HTTP API
```bash
# ASBA as microservice
asba serve --port=8080 &
aws-slurm-burst-resume aws-hpc-[001-008] --asba-url=http://localhost:8080
```