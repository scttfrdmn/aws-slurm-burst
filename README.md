# AWS Slurm Burst

A high-performance Go-based plugin system for intelligent Slurm workload bursting to AWS, with first-class MPI support and dynamic instance optimization.

## Features

- 🚀 **MPI-Aware**: Gang scheduling with cluster placement groups for tightly-coupled workloads
- 📊 **Smart Instance Selection**: Dynamic right-sizing based on job requirements and real-time pricing
- 🔄 **ABSA Integration**: Coordinates with aws-slurm-burst-advisor for intelligent burst decisions
- ⚡ **High Performance**: Go concurrency for fast node provisioning and management
- 📈 **Observability**: Prometheus metrics and structured logging
- 🎯 **Cost Optimization**: Spot instance management with MPI-aware interruption handling

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Slurm Daemon  │    │  ABSA Advisor   │    │   AWS APIs      │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ResumeProgram│ │    │ │Burst Advisor│ │    │ │   EC2 Fleet │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ aws-slurm-burst │
                    │                 │
                    │ ┌─────────────┐ │
                    │ │MPI Scheduler│ │
                    │ ├─────────────┤ │
                    │ │Job Analyzer │ │
                    │ ├─────────────┤ │
                    │ │Cost Optimizer│ │
                    │ └─────────────┘ │
                    └─────────────────┘
```

## Quick Start

```bash
# Install
go install github.com/scttfrdmn/aws-slurm-burst/cmd/...@latest

# Configure
aws-slurm-burst config init

# Generate Slurm configuration
aws-slurm-burst config generate-slurm

# Test MPI job bursting
sbatch --partition=aws-burst examples/mpi-job.sbatch
```

## Integration with ABSA

This project coordinates with [aws-slurm-burst-advisor](https://github.com/scttfrdmn/aws-slurm-burst-advisor) to make intelligent bursting decisions:

```bash
# ABSA determines if job should burst to AWS
absa analyze job.sbatch --output=decision.json

# aws-slurm-burst executes the burst with optimal instance selection
aws-slurm-burst resume --job-metadata=decision.json node-[1-4]
```