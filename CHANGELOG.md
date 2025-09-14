# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project structure with Go 1.23+ support
- MPI-aware job analysis and scheduling
- EFA (Elastic Fabric Adapter) support with automatic detection
- HPC instance family optimization (hpc7a, hpc6id, hpc6a)
- Integration with aws-slurm-burst-advisor (ABSA) for cost optimization
- Dynamic instance selection based on job requirements
- Placement group management for MPI workloads
- Spot instance support with MPI-aware interruption handling
- Comprehensive logging with structured zap logger
- Prometheus metrics for observability
- CLI tools for resume, suspend, and state management operations
- Configuration management with YAML support

### Changed
- Complete rewrite from Python to Go for improved performance
- Modern architecture with clean separation of concerns
- Enhanced error handling and retry mechanisms

### Deprecated

### Removed

### Fixed

### Security
- IAM role-based authentication (no embedded credentials)
- Secure instance communication via VPC
- Regular security scanning in CI pipeline

## [0.1.0] - 2025-09-13

### Added
- Initial release candidate
- Core Slurm integration functionality
- Basic AWS EC2 instance management
- MPI job detection and optimization
- Documentation and examples