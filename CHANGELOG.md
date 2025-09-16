# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

## [0.4.0] - 2025-09-15

### Added
- **Modern AWS Authentication**: Comprehensive security architecture with 6 authentication methods
- **IAM Identity Center (SSO)**: Perfect integration for university identity management
- **STS AssumeRole**: Temporary credentials for secure cross-account research
- **Web Identity Federation**: Kubernetes/container-native authentication
- **Cross-Account Support**: Multi-institution research collaboration security
- **Operational Independence**: Auto-detection with graceful enhancement when ecosystem available
- **Security Documentation**: Complete authentication guide with academic compliance considerations

### Changed
- **Authentication Architecture**: Modern AWS SDK v2 authentication patterns
- **Configuration Format**: Support for multiple authentication methods with auto-detection
- **Security Defaults**: Instance profile as secure default instead of access keys

### Security
- **Access Keys Discouraged**: Supported for compatibility but with prominent security warnings
- **Credential Validation**: Automatic validation and permission checking
- **Academic Compliance**: NSF, NIH, DOE grant requirement support
- **Audit Trails**: Complete credential usage tracking and monitoring

## [0.3.0] - 2025-09-13

### Added
- **ASBX Branding**: Rebranded as AWS Slurm Burst eXecution for ecosystem clarity
- **Performance Data Export**: Comprehensive aws-slurm-burst-export-performance CLI
- **ASBA Learning Integration**: Complete performance feedback loop for adaptive intelligence
- **ASBB Budget Integration**: Cost reconciliation data export for budget management
- **SLURM Epilog Scripts**: Automatic performance data collection via job epilog
- **Ecosystem Coordination**: Cross-project integration with ASBA v0.3.0 and ASBB
- **Enhanced Validation**: aws-slurm-burst-validate with execution plan support
- **README Badges**: Complete quality indicators including Go Report Card A+
- **Development Support**: Slurm detection with graceful mock fallback

### Changed
- **Project Identity**: Positioned as execution engine in three-project ecosystem
- **Documentation**: Complete ecosystem integration guides and coordination
- **CLI Interface**: Enhanced with multiple export formats and validation options

### Fixed
- **CI Pipeline**: Resolved all linting and security scanning issues
- **Go Report Card**: Achieved A+ grade with zero issues
- **Error Handling**: Comprehensive error checking and logging improvements
- **Security**: File permissions and proper error handling for production use

## [0.2.0] - 2025-09-13

### Added
- **Complete AWS Integration**: Real EC2 Fleet API with AWS SDK v2
- **MPI Gang Scheduling**: Atomic all-or-nothing provisioning for MPI workloads
- **EFA Optimization**: Automatic EFA-capable instance detection and filtering
- **Advanced Placement Groups**: Dynamic cluster/partition/spread placement strategies
- **Spot Instance Management**: Real-time spot pricing with interruption monitoring
- **Mixed Pricing Support**: Intelligent spot/on-demand allocation with MPI-aware strategies
- **ExecutionPlan Support**: Complete ASBA integration via JSON execution plans
- **Dual Operation Modes**: Standalone (static config) + ASBA (intelligent optimization)
- **Validation Commands**: aws-slurm-burst-validate for configs and execution plans
- **Development Support**: Graceful Slurm detection with mock fallback

### Changed
- **Architectural Refactor**: Clean separation of concerns with ASBA
- **ASBA Integration**: Pure execution engine - ASBA provides intelligence
- **Instance Selection**: ASBA-driven vs standalone mode logic
- **Error Handling**: Comprehensive AWS API error handling and retry logic

### Security
- **AWS SDK v2**: Latest security practices and authentication
- **IAM Integration**: Proper instance profile and role-based authentication
- **Network Security**: VPC, security group, and placement group management

## [0.1.0] - 2025-09-13

### Added
- Initial release candidate
- Core Slurm integration functionality
- Basic AWS EC2 instance management
- MPI job detection and optimization
- Documentation and examples