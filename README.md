# AWS Slurm Burst (ASBX)

[![Go Report Card](https://goreportcard.com/badge/github.com/scttfrdmn/aws-slurm-burst)](https://goreportcard.com/report/github.com/scttfrdmn/aws-slurm-burst)
[![Go Reference](https://pkg.go.dev/badge/github.com/scttfrdmn/aws-slurm-burst.svg)](https://pkg.go.dev/github.com/scttfrdmn/aws-slurm-burst)
[![GitHub tag](https://img.shields.io/github/tag/scttfrdmn/aws-slurm-burst.svg)](https://github.com/scttfrdmn/aws-slurm-burst/tags)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Build Status](https://github.com/scttfrdmn/aws-slurm-burst/workflows/CI/badge.svg)](https://github.com/scttfrdmn/aws-slurm-burst/actions)
[![codecov](https://codecov.io/gh/scttfrdmn/aws-slurm-burst/branch/main/graph/badge.svg)](https://codecov.io/gh/scttfrdmn/aws-slurm-burst)

A production-ready, high-performance Go-based Slurm plugin for intelligent AWS workload bursting with advanced MPI support, EFA optimization, and cost-aware execution.

**ASBX** (AWS Slurm Burst eXecution) is the execution engine of the research computing ecosystem.

## 🌟 Research Computing Ecosystem

**ASBX** works seamlessly with companion projects:
- **[ASBA](https://github.com/scttfrdmn/aws-slurm-burst-advisor)** (Intelligence): Analyzes workloads and optimizes decisions
- **[ASBB](https://github.com/scttfrdmn/aws-slurm-burst-budget)** (Budget): Manages real grant money and cost enforcement
- **ASBX** (This Project): Executes optimized workloads with MPI and EFA support

## 🎯 Current Status: v0.2.0

**✅ Production Ready**: Complete AWS integration with gang scheduling
**✅ MPI Optimized**: EFA-aware provisioning with placement groups
**✅ Cost Intelligent**: Spot instance management with mixed pricing
**✅ Ecosystem Integrated**: Clean separation with ASBA intelligence and ASBB budget management

## Features

- 🚀 **MPI Gang Scheduling**: Atomic all-or-nothing provisioning for tightly-coupled workloads
- ⚡ **EFA Integration**: Automatic EFA-capable instance selection and configuration
- 📊 **Spot Intelligence**: Real-time spot pricing with MPI-aware allocation strategies
- 🔄 **Dual Mode Operation**: Standalone (static config) + ASBA (intelligent optimization)
- 🎯 **Cost Optimization**: Mixed spot/on-demand with automatic fallback
- 📈 **Production Ready**: Comprehensive validation, error handling, and observability

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Slurm Daemon  │    │  ASBA Advisor   │    │   AWS APIs      │
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

### Installation
```bash
# Download latest release
curl -L https://github.com/scttfrdmn/aws-slurm-burst/releases/latest/download/aws-slurm-burst-linux-amd64.tar.gz | tar xz

# Or build from source
git clone https://github.com/scttfrdmn/aws-slurm-burst.git
cd aws-slurm-burst
make build
```

### Configuration
```bash
# Validate your configuration
aws-slurm-burst-validate config examples/config.yaml

# Test standalone mode (like original plugin)
aws-slurm-burst-resume aws-cpu-[001-004] --config=config.yaml --dry-run
```

### ASBA Integration (Recommended)
```bash
# ASBA analyzes and optimizes
asba analyze job.sbatch --format=execution-plan --output=plan.json

# aws-slurm-burst executes the optimized plan
aws-slurm-burst-resume aws-hpc-[001-016] --execution-plan=plan.json
```

## Current Capabilities (v0.2.0-rc)

### ✅ Complete AWS Integration
- **EC2 Fleet API**: Real instance provisioning with AWS SDK v2
- **Instance Lifecycle**: Launch, terminate, and state management
- **Error Handling**: Retry logic, rollback, and graceful degradation

### ✅ MPI Optimization
- **Gang Scheduling**: Atomic all-or-nothing provisioning for MPI jobs
- **EFA Support**: Automatic EFA-capable instance selection
- **Placement Groups**: Cluster/partition/spread strategies for optimal networking
- **Performance Validation**: Pre-flight capacity checks and instance verification

### ✅ Cost Intelligence
- **Spot Management**: Real-time spot pricing with interruption monitoring
- **Mixed Pricing**: Intelligent spot/on-demand allocation strategies
- **Cost Constraints**: Budget limits and automatic cost estimation
- **MPI-Aware Pricing**: Different strategies for MPI vs embarrassingly parallel jobs

### ✅ Dual Operation Modes

**Standalone Mode** (Like Original Plugin):
```bash
# Uses static config.yaml instance types
aws-slurm-burst-resume aws-cpu-[001-004] --config=config.yaml
```

**ASBA Mode** (Intelligent Optimization):
```bash
# ASBA provides execution plan, aws-slurm-burst executes
aws-slurm-burst-resume aws-hpc-[001-016] --execution-plan=asba-plan.json
```

## Integration with ASBA

**Clean Architecture**: ASBA = Intelligence, aws-slurm-burst = Execution Engine

- **ASBA Analyzes**: MPI patterns, cost optimization, instance selection
- **aws-slurm-burst Executes**: AWS provisioning, gang scheduling, cost management

See [ASBA Integration Guide](docs/ASBA-INTEGRATION.md) for complete integration patterns.