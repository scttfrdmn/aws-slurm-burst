# ABSA Integration Patterns

## Overview

`aws-slurm-burst` supports multiple communication mechanisms with `aws-slurm-burst-advisor` (ABSA) to enable intelligent workload optimization while maintaining standalone functionality.

## Communication Mechanisms

### 1. File-based Communication (Current)

**Workflow**:
```bash
# ABSA analyzes job and writes execution plan
absa analyze job.sbatch --output=/tmp/execution-plan.json

# aws-slurm-burst executes the plan
aws-slurm-burst-resume aws-hpc-[001-008] --execution-plan=/tmp/execution-plan.json
```

**Benefits**:
- ✅ Simple integration
- ✅ Debugging friendly (can inspect plans)
- ✅ Async operation support
- ✅ No network dependencies

**Use Cases**: Batch job submission, research workflows, debugging

### 2. Environment Variable Communication (Proposed)

**Workflow**:
```bash
# ABSA sets execution environment
eval $(absa analyze job.sbatch --export-env)

# aws-slurm-burst reads from environment
aws-slurm-burst-resume aws-hpc-[001-008]
```

**Environment Variables**:
```bash
ABSA_SHOULD_BURST=true
ABSA_INSTANCE_TYPES="hpc7a.2xlarge,c6i.2xlarge"
ABSA_PURCHASING_OPTION=spot
ABSA_MAX_SPOT_PRICE=0.85
ABSA_REQUIRES_EFA=true
ABSA_PLACEMENT_GROUP=cluster
ABSA_MPI_PROCESSES=128
ABSA_GANG_SCHEDULING=true
```

**Benefits**:
- ✅ Shell-friendly integration
- ✅ Works with existing Slurm scripts
- ✅ No temporary files
- ✅ Standard Unix pattern

**Use Cases**: Interactive sessions, shell scripts, cron jobs

### 3. Named Pipe Communication (Advanced)

**Workflow**:
```bash
# ABSA creates named pipe for communication
absa daemon --pipe=/tmp/absa-decisions &

# aws-slurm-burst reads from pipe
aws-slurm-burst-resume aws-hpc-[001-008] --absa-pipe=/tmp/absa-decisions
```

**Benefits**:
- ✅ Real-time communication
- ✅ Low latency decisions
- ✅ Streaming data support
- ✅ No file system pollution

**Use Cases**: High-throughput job submission, daemon mode operation

### 4. HTTP API Communication (Future)

**Workflow**:
```bash
# ABSA as microservice
absa serve --port=8080 &

# aws-slurm-burst makes HTTP requests
aws-slurm-burst-resume aws-hpc-[001-008] --absa-url=http://localhost:8080
```

**API Endpoints**:
```
POST /analyze
{
  "job_script": "#!/bin/bash\n...",
  "node_count": 8,
  "partition": "aws-hpc"
}

Response:
{
  "execution_plan": { ... },
  "analysis_id": "uuid",
  "confidence": 0.95
}
```

**Benefits**:
- ✅ Language agnostic
- ✅ Network distributed
- ✅ REST API standard
- ✅ Monitoring/observability

**Use Cases**: Multi-cluster deployments, web interfaces, monitoring systems

## Integration Modes

### Standalone Mode (Default)
```bash
# No ABSA - uses static configuration
aws-slurm-burst-resume aws-cpu-[001-004]
```

Uses configuration-defined instance types, like original plugin behavior.

### ABSA-Enhanced Mode
```bash
# ABSA optimizes - aws-slurm-burst executes
absa analyze job.sbatch --output=plan.json
aws-slurm-burst-resume aws-hpc-[001-016] --execution-plan=plan.json
```

ABSA provides optimal instance selection and cost optimization.

### Auto-Discovery Mode (Proposed)
```bash
# Automatically detect ABSA and use if available
aws-slurm-burst-resume aws-hpc-[001-008] --auto-absa
```

Checks for ABSA in PATH, uses it if available, falls back to standalone.

## ABSA Feature Requests for aws-slurm-burst

Based on the integration patterns, here are feature requests for ABSA:

### 1. Execution Plan Export
```bash
absa analyze job.sbatch --format=execution-plan --output=plan.json
```

**Required**: JSON schema for execution plans compatible with aws-slurm-burst

### 2. Environment Variable Export
```bash
eval $(absa analyze job.sbatch --export-env)
```

**Required**: Environment variable format specification

### 3. Slurm Integration Hooks
```bash
# ABSA as Slurm prolog/epilog
absa slurm-prolog --job-id=$SLURM_JOB_ID --export-env
```

**Required**: Slurm job metadata parsing

### 4. Daemon Mode
```bash
absa daemon --pipe=/tmp/absa --log-level=debug
```

**Required**: Streaming analysis for high-throughput scenarios

### 5. Instance Type Recommendations
```bash
absa recommend-instances --cpus=16 --memory=64GB --mpi=true --efa-required
```

**Required**: Instance type database and optimization algorithms

## Error Handling and Fallbacks

### ABSA Unavailable
```
aws-slurm-burst-resume aws-cpu-[001-004] --execution-plan=missing.json
↓
WARN: Execution plan not found, falling back to standalone mode
INFO: Using static configuration from config.yaml
```

### Invalid Execution Plan
```
aws-slurm-burst-resume aws-cpu-[001-004] --execution-plan=invalid.json
↓
ERROR: Invalid execution plan: no instance types specified
SUGGESTION: Run 'absa analyze' to generate valid plan
```

### Partial ABSA Data
```
# ABSA provides instance types but no cost constraints
{
  "should_burst": true,
  "instance_specification": {
    "instance_types": ["c6i.xlarge"]
  }
  // Missing other fields
}
↓
INFO: Using ABSA instance types with default cost constraints
```

## Development Integration

### For ABSA Development
```bash
# Test execution plans
aws-slurm-burst-resume test-[001-002] --execution-plan=test-plan.json --dry-run

# Validate plan format
aws-slurm-burst validate-plan test-plan.json
```

### For aws-slurm-burst Development
```bash
# Test without ABSA
aws-slurm-burst-resume test-[001-002] --config=test-config.yaml

# Test with mock ABSA plan
aws-slurm-burst-resume test-[001-002] --execution-plan=examples/absa-execution-plan.json
```

This design maintains clear separation of concerns while providing multiple integration options for different use cases.