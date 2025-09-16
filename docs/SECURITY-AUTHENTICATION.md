# AWS Authentication Security Guide

## 🔐 Modern Authentication Methods for Academic Research Computing

ASBX supports multiple AWS authentication methods, prioritizing security while maintaining flexibility for different academic deployment scenarios.

## 🏆 Recommended Authentication Methods (Best → Acceptable)

### 1. **EC2 Instance Profile** (🥇 Most Secure for AWS-hosted)
```yaml
aws:
  authentication_method: instance_profile
  region: us-east-1
```

**Benefits**:
- ✅ **No stored credentials** - Uses IAM role attached to EC2 instance
- ✅ **Automatic credential management** - AWS handles rotation
- ✅ **Fine-grained permissions** - Precise IAM policy controls
- ✅ **Audit trails** - Complete CloudTrail logging

**Best For**: AWS-hosted Slurm head nodes

### 2. **AWS IAM Identity Center (SSO)** (🥇 Best for Universities)
```yaml
aws:
  authentication_method: sso
  sso:
    profile_name: "university-research"
    start_url: "https://university.awsapps.com/start"
    account_id: "123456789012"
    role_name: "ResearchComputingRole"
```

**Benefits**:
- ✅ **University identity integration** - SAML/Active Directory
- ✅ **Temporary credentials** - Automatically refreshed
- ✅ **Centralized management** - IT controls access
- ✅ **Compliance ready** - Audit trails and access reviews

**Best For**: Universities with centralized identity management

### 3. **STS AssumeRole** (🥈 Excellent for Temporary Access)
```yaml
aws:
  authentication_method: assume_role
  assume_role:
    role_arn: "arn:aws:iam::123456789012:role/SlurmBurstRole"
    session_name: "slurm-burst-session"
    duration_seconds: 3600
    external_id: "university-shared-secret"
```

**Benefits**:
- ✅ **Temporary credentials** - Expire automatically
- ✅ **External ID security** - Additional authentication factor
- ✅ **Session tracking** - Named sessions for audit
- ✅ **Cross-account support** - Multi-account academic setups

**Best For**: Cross-account research setups, temporary access

### 4. **Web Identity Federation** (🥈 Modern for Containers)
```yaml
aws:
  authentication_method: web_identity
  web_identity:
    role_arn: "arn:aws:iam::123456789012:role/EKSSlurmRole"
    token_file: "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
    session_name: "slurm-k8s-pod"
```

**Benefits**:
- ✅ **Container-native** - Kubernetes service account integration
- ✅ **Short-lived tokens** - Automatic rotation
- ✅ **Pod-level isolation** - Per-workload credentials
- ✅ **Cloud-native security** - Modern container patterns

**Best For**: Kubernetes/container-based Slurm deployments

### 5. **AWS Profile** (🥉 Acceptable for Development)
```yaml
aws:
  authentication_method: profile
  profile: "research-development"
```

**Benefits**:
- ✅ **Credential isolation** - Separate profiles per environment
- ✅ **AWS CLI integration** - Uses standard AWS configuration
- ✅ **Development friendly** - Easy local testing

**Best For**: Development environments, local testing

### 6. **Static Access Keys** (⚠️ DISCOURAGED)
```yaml
aws:
  authentication_method: access_keys  # ⚠️ SECURITY RISK
  access_keys:
    access_key_id: "AKIA..."          # ⚠️ NEVER commit to version control
    secret_access_key: "..."          # ⚠️ Store in environment variables
```

**Security Warnings**:
- ⚠️ **Long-lived credentials** - Security risk if compromised
- ⚠️ **No automatic rotation** - Manual key management required
- ⚠️ **Accidental exposure risk** - Can be committed to version control
- ⚠️ **Compliance issues** - Not suitable for academic grant requirements

**Only Acceptable For**:
- Legacy system migration (temporary)
- Development/testing environments
- Emergency access scenarios

## 🎓 Academic Institution Recommendations

### Large Universities (>10,000 students)
**Recommended**: **AWS IAM Identity Center (SSO)**
- Integrate with existing Active Directory/SAML
- Centralized identity management
- Compliance with institutional security policies
- Support for multi-PI research projects

### Small Colleges/Research Centers
**Recommended**: **EC2 Instance Profile** or **AssumeRole**
- Simpler setup and management
- Cost-effective security
- Easy integration with existing AWS accounts

### Multi-Institution Collaborations
**Recommended**: **Cross-Account AssumeRole**
- Clear security boundaries between institutions
- Shared research projects with separate billing
- Compliance with each institution's policies

### Container-Based HPC Centers
**Recommended**: **Web Identity Federation**
- Modern Kubernetes/container security
- Cloud-native credential management
- Scalable for large container deployments

## 🔒 Security Best Practices

### Credential Management
```bash
# ✅ DO: Use IAM roles whenever possible
aws:
  authentication_method: instance_profile

# ✅ DO: Use temporary credentials
aws:
  authentication_method: assume_role
  assume_role:
    duration_seconds: 3600  # 1 hour maximum

# ❌ DON'T: Store credentials in configuration files
# aws:
#   access_key_id: "AKIA..."     # ❌ NEVER DO THIS
#   secret_access_key: "..."     # ❌ SECURITY RISK
```

### Permission Principles
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:CreateFleet",
        "ec2:TerminateInstances",
        "ec2:DescribeInstances"
      ],
      "Resource": "*",
      "Condition": {
        "StringEquals": {
          "ec2:Region": ["us-east-1", "us-west-2"]
        }
      }
    }
  ]
}
```

**Principles**:
- ✅ **Least Privilege**: Only required permissions
- ✅ **Resource Constraints**: Limit to specific regions/resources
- ✅ **Time Boundaries**: Use temporary credentials
- ✅ **Audit Trails**: Enable CloudTrail logging

### Multi-User Environments
```yaml
# Separate roles for different user classes
research_faculty:
  role_arn: "arn:aws:iam::123456789012:role/FacultyComputeRole"
  max_instances: 50

graduate_students:
  role_arn: "arn:aws:iam::123456789012:role/StudentComputeRole"
  max_instances: 10

# Budget constraints per user class
cost_controls:
  faculty_monthly_limit: 1000.00
  student_monthly_limit: 100.00
```

## 🛡️ Security Monitoring

### Credential Validation
```bash
# ASBX automatically validates credentials and permissions
aws-slurm-burst-validate integration

# Output shows credential info:
# ✅ AWS Credentials Valid
#    Account: 123456789012
#    ARN: arn:aws:sts::123456789012:assumed-role/SlurmBurstRole/session
#    Method: assume_role
```

### Audit and Monitoring
```bash
# Enable comprehensive AWS API logging
aws cloudtrail create-trail --name slurm-burst-audit

# Monitor ASBX API usage
aws logs filter-log-events --log-group-name aws-slurm-burst

# Check for unauthorized access attempts
aws iam get-account-authorization-details
```

## 🎯 Migration Guide

### From Access Keys to Modern Authentication

**Step 1: Assessment**
```bash
# Check current authentication method
aws-slurm-burst-validate integration
# Shows: "Using access_keys authentication ⚠️ SECURITY RISK"
```

**Step 2: Choose Target Method**
- **AWS-hosted**: Migrate to `instance_profile`
- **University SSO**: Migrate to `sso`
- **Multi-account**: Migrate to `assume_role`
- **Kubernetes**: Migrate to `web_identity`

**Step 3: Gradual Migration**
```yaml
# Phase 1: Test new method in development
aws:
  authentication_method: instance_profile

# Phase 2: Update production configuration
# Phase 3: Remove access keys from old configuration
```

### Validation
```bash
# Test new authentication method
aws-slurm-burst-resume test-node-001 --dry-run

# Verify permissions
aws-slurm-burst-validate integration
```

## 🚨 Security Incidents

### Credential Compromise Response
1. **Immediate**: Disable compromised credentials
2. **Assessment**: Check CloudTrail for unauthorized usage
3. **Rotation**: Create new credentials with new method
4. **Prevention**: Migrate to IAM roles to prevent future compromise

### Access Key Best Practices (If Required)
```bash
# Environment variables (better than config files)
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."

# Temporary credentials only
export AWS_SESSION_TOKEN="..."  # Include session token for temporary keys

# Regular rotation
aws iam rotate-access-key --access-key-id AKIA...
```

## 🎓 Compliance Considerations

### Academic Grant Requirements
- **NSF**: Requires secure credential management
- **NIH**: Mandates audit trails and access controls
- **DOE**: Requires encryption and access monitoring

### Institutional Policies
- **FERPA**: Student data protection requirements
- **HIPAA**: Healthcare research data security
- **SOX**: Financial data handling for business schools

**Recommendation**: Use **IAM Identity Center (SSO)** for comprehensive compliance coverage.

---

**ASBX provides secure, flexible authentication while maintaining academic institution compliance requirements!** 🔒