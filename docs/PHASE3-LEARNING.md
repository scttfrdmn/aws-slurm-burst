# Phase 3: Performance Learning Integration

## Overview

Phase 3 establishes the performance feedback loop between aws-slurm-burst and ASBA, enabling adaptive learning and continuous improvement of research computing recommendations.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Job Execution │    │ Performance     │    │ ASBA Learning   │
│                 │    │ Data Export     │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │EC2 Instances│ │───▶│ │Epilog Script│ │───▶│ │Model Update │ │
│ │MPI Execution│ │    │ │Performance  │ │    │ │Accuracy Imp │ │
│ │Cost Tracking│ │    │ │Export       │ │    │ │Domain Learn │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Performance Data Export

### Comprehensive Metrics Collection

**Job Execution Data**:
- Instance types actually used vs requested
- Real AWS costs vs ASBA predictions
- Execution duration vs time estimates
- Success/failure rates and error details

**AWS Performance Metrics**:
- EFA utilization and effectiveness
- Placement group performance impact
- Spot interruption rates and recovery
- Network throughput and latency

**MPI Optimization Results** (for MPI jobs):
- Communication overhead percentages
- Scaling efficiency across nodes
- Load balance distribution
- Collective operation efficiency

### CLI Commands

**Performance Export**:
```bash
# Export comprehensive performance data
aws-slurm-burst-export-performance --job-id=12345 --format=asba-learning

# Export for budget systems
aws-slurm-burst-export-performance --job-id=12345 --format=slurm-comment

# Anonymized export for institutional sharing
aws-slurm-burst-export-performance --job-id=12345 --anonymize --output-dir=/tmp
```

## SLURM Integration

### Automatic Data Collection via Epilog

**Installation**:
```bash
# Install epilog script
sudo cp scripts/slurm-epilog-aws.sh /etc/slurm/epilog.d/aws-burst-metadata.sh
sudo chmod +x /etc/slurm/epilog.d/aws-burst-metadata.sh
```

**Epilog Script Functionality**:
- Automatically runs after every AWS partition job completion
- Exports performance data in ASBA-compatible format
- Updates Slurm job comments with cost/performance metadata
- Handles errors gracefully without failing job completion

### Job Comment Integration

**Enhanced Job Comments**:
```bash
# Before (standard Slurm)
sacct --format=JobID,Comment
12345   user_comment

# After (with aws-slurm-burst)
sacct --format=JobID,Comment
12345   aws_meta:{"cost":12.45,"instances":["c5n.xlarge"],"mpi_eff":0.87}
```

## ASBA Learning Integration

### Data Format for ASBA

**Performance Feedback Structure**:
```json
{
  "job_metadata": {
    "job_id": "12345",
    "original_asba_prediction": { /* original execution plan */ },
    "actual_execution": {
      "instance_types_used": ["c5n.xlarge"],
      "actual_cost_usd": 12.45,
      "execution_duration": "2h15m",
      "success": true
    }
  },
  "prediction_validation": {
    "cost_accuracy": 0.94,
    "runtime_accuracy": 0.91,
    "instance_type_optimal": true,
    "overall_accuracy_score": 0.92
  },
  "aws_performance_metrics": {
    "efa_utilization": 0.89,
    "placement_group_effectiveness": 0.95,
    "network_throughput_gbps": 45.2
  },
  "mpi_optimization_results": {
    "communication_overhead": 0.12,
    "scaling_efficiency": 0.87,
    "load_balance": 0.93
  }
}
```

### Learning Workflow

**Step 1: Job Execution**
```bash
# ASBA generates plan
asba analyze job.sbatch --format=execution-plan --output=plan.json

# aws-slurm-burst executes with monitoring
aws-slurm-burst-resume nodes --execution-plan=plan.json
```

**Step 2: Automatic Data Collection**
```bash
# Epilog script runs automatically
# Exports performance data to /var/spool/asba/learning/
```

**Step 3: ASBA Learning**
```bash
# ASBA processes performance feedback
asba learn-from-execution --data-dir=/var/spool/asba/learning/

# Update models based on feedback
asba retrain-models --domain=all --accuracy-threshold=0.8
```

## Performance Monitoring

### Real-time Monitoring (Future v0.4.0)

**Monitoring During Execution**:
```bash
# Enable performance monitoring
aws-slurm-burst-resume nodes --enable-performance-monitoring

# Monitor job in real-time
aws-slurm-burst monitor-performance --job-id=12345
```

**MPI Communication Profiling**:
- Hook into MPI runtime for communication pattern analysis
- Network bandwidth and latency monitoring
- Load balance detection across processes
- Bottleneck identification and reporting

## Academic Research Benefits

### Institutional Analytics

**Research Efficiency Metrics**:
- Cost savings through optimization
- Performance improvements over time
- Domain-specific best practices
- Resource utilization patterns

**Grant Management Support**:
- Accurate cost tracking for budget reporting
- Performance metrics for grant renewals
- Reproducibility data for research validation
- Institutional resource optimization

### Multi-Institutional Learning

**Anonymized Data Sharing**:
```bash
# Export anonymized performance data
aws-slurm-burst-export-performance --anonymize --format=institutional-sharing

# Aggregate across institutions for pattern learning
asba learn-institutional-patterns --data-source=anonymized-exports
```

## Implementation Timeline

### v0.3.0 (Current) - Basic Learning Integration
- ✅ Performance data export CLI
- ✅ SLURM epilog script integration
- ✅ Basic cost and performance tracking
- ✅ ASBA learning data format

### v0.4.0 (1-2 months) - Advanced Monitoring
- Real-time performance monitoring during job execution
- Advanced MPI communication profiling
- Spot interruption learning and recovery optimization
- Enhanced cost tracking with detailed breakdowns

### v0.5.0 (2-3 months) - Institutional Features
- Multi-user aggregated analytics
- Cross-institutional anonymized learning
- Predictive performance monitoring
- Advanced research workflow optimization

## Success Metrics

**ASBA Learning Targets**:
- Domain detection accuracy >80% (supported by execution validation)
- Performance prediction <20% error (validated by actual runtime data)
- Cost prediction <15% error (validated by actual AWS costs)

**Research Productivity**:
- 25% improvement in computational efficiency
- 30% cost savings through optimization
- 50% reduction in manual parameter tuning

This Phase 3 implementation establishes aws-slurm-burst as the **definitive performance data source** for adaptive research computing intelligence.