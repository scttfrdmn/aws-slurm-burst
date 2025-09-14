# Production Deployment Guide

## Prerequisites

### AWS Setup
- AWS account with EC2 permissions
- VPC with public/private subnets
- Launch templates configured for Slurm compute nodes
- IAM roles for compute instances

### Slurm Environment
- Slurm 20.02+ cluster (head node)
- Python 3 and standard utilities
- Network connectivity to AWS VPC

## Installation Steps

### 1. Download and Install Binaries

```bash
# Download latest release
cd /opt/slurm/etc
wget -q https://github.com/scttfrdmn/aws-slurm-burst/releases/latest/download/aws-slurm-burst-linux-amd64.tar.gz
tar xzf aws-slurm-burst-linux-amd64.tar.gz

# Install binaries
sudo cp resume suspend state-manager validate /usr/local/bin/
sudo chmod +x /usr/local/bin/aws-slurm-burst-*

# Verify installation
aws-slurm-burst-validate --help
```

### 2. Configure AWS Credentials

```bash
# Option A: IAM role (recommended for AWS-hosted head nodes)
# Attach IAM role to head node instance

# Option B: AWS credentials file
aws configure
# Enter access key, secret key, region

# Test AWS access
aws sts get-caller-identity
```

### 3. Create Configuration File

```bash
# Create configuration directory
sudo mkdir -p /etc/slurm/aws-burst

# Copy example configuration
cp examples/config.yaml /etc/slurm/aws-burst/config.yaml

# Edit configuration for your environment
sudo vi /etc/slurm/aws-burst/config.yaml
```

**Required Configuration Changes**:
- `aws.region`: Your target AWS region
- `subnet_ids`: Your VPC subnet IDs
- `security_group_ids`: Security groups allowing Slurm traffic
- `launch_template_name`: Your compute node launch template
- `iam_instance_profile`: IAM role for compute instances

### 4. Validate Configuration

```bash
# Validate configuration
aws-slurm-burst-validate config /etc/slurm/aws-burst/config.yaml

# Test with dry run
aws-slurm-burst-resume test-node-001 --config=/etc/slurm/aws-burst/config.yaml --dry-run
```

### 5. Update Slurm Configuration

Add to `/etc/slurm/slurm.conf`:

```bash
# AWS burst partition configuration
PrivateData=cloud
ResumeProgram=/usr/local/bin/aws-slurm-burst-resume --config=/etc/slurm/aws-burst/config.yaml
SuspendProgram=/usr/local/bin/aws-slurm-burst-suspend --config=/etc/slurm/aws-burst/config.yaml
ResumeRate=100
SuspendRate=100
ResumeTimeout=600
SuspendTime=300
TreeWidth=60000

# Node definitions (generated from config)
NodeName=aws-cpu-[001-020] State=CLOUD CPUs=4 RealMemory=8192
NodeName=aws-gpu-[001-010] State=CLOUD CPUs=8 RealMemory=32768 Gres=gpu:4

# Partition definitions
PartitionName=aws-cpu Nodes=aws-cpu-[001-020] Default=YES MaxTime=INFINITE State=UP
PartitionName=aws-gpu Nodes=aws-gpu-[001-010] MaxTime=INFINITE State=UP
```

### 6. Setup State Management

```bash
# Add cron job for state management
sudo crontab -e

# Add this line:
* * * * * /usr/local/bin/aws-slurm-burst-state-manager --config=/etc/slurm/aws-burst/config.yaml >/dev/null 2>&1
```

### 7. Restart Slurm Services

```bash
# Reload Slurm configuration
sudo scontrol reconfigure

# Verify nodes are visible
sinfo -p aws-cpu
sinfo -p aws-gpu
```

## Testing Your Deployment

### Basic Functionality Test

```bash
# Test CPU partition
srun -p aws-cpu hostname

# Test GPU partition
srun -p aws-gpu nvidia-smi

# Test MPI job
sbatch examples/mpi-job.sbatch
```

### MPI Performance Test

```bash
# Submit MPI job to test gang scheduling
sbatch --partition=aws-cpu --nodes=4 --ntasks-per-node=8 examples/mpi-test.sbatch

# Monitor job execution
squeue
watch sinfo
```

### Cost Optimization Test

```bash
# Test spot instance usage
aws-slurm-burst-resume aws-cpu-[001-004] --config=config.yaml --dry-run

# Check estimated costs
grep "Estimated Total Cost" /var/log/slurm/aws-burst.log
```

## Integration with ASBA (Optional)

### Install ASBA
```bash
# Install aws-slurm-burst-advisor
go install github.com/scttfrdmn/aws-slurm-burst-advisor/cmd/asba@latest

# Verify installation
asba --version
```

### Test ASBA Integration
```bash
# Generate execution plan
asba analyze examples/mpi-job.sbatch --format=execution-plan --output=plan.json

# Execute optimized plan
aws-slurm-burst-resume aws-hpc-[001-008] --execution-plan=plan.json --dry-run
```

## Monitoring and Troubleshooting

### Log Files
- **Plugin logs**: `/var/log/slurm/aws-burst.log`
- **Slurm logs**: `/var/log/slurm/slurmctld.log`
- **Node logs**: `/var/log/slurm/slurmd.log`

### Common Issues

**Nodes don't start**:
- Check AWS permissions and launch template
- Verify subnet connectivity to head node
- Check security group rules for Slurm traffic

**MPI jobs fail**:
- Verify EFA is enabled in launch template
- Check placement group creation
- Monitor gang scheduling logs

**High costs**:
- Review spot instance usage
- Check instance type selection
- Monitor cost estimation logs

### Performance Monitoring

```bash
# Monitor AWS resource usage
aws ec2 describe-instances --filters "Name=tag:ManagedBy,Values=aws-slurm-burst"

# Check placement group effectiveness
aws ec2 describe-placement-groups

# Monitor spot instance pricing
aws ec2 describe-spot-price-history --instance-types c5n.large
```

## Security Considerations

### IAM Permissions

Minimum required permissions for head node:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateFleet",
        "ec2:TerminateInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceTypes",
        "ec2:DescribeSpotPriceHistory",
        "ec2:CreatePlacementGroup",
        "ec2:DescribePlacementGroups",
        "ec2:CreateTags",
        "iam:PassRole"
      ],
      "Resource": "*"
    }
  ]
}
```

### Network Security
- Use private subnets for compute nodes
- Restrict security groups to necessary Slurm ports
- Enable VPC Flow Logs for network monitoring

### Cost Controls
- Set up billing alerts for unexpected charges
- Use cost allocation tags for tracking
- Monitor spot instance usage and interruptions

This deployment guide ensures secure, reliable, and cost-effective operation of aws-slurm-burst in production environments.