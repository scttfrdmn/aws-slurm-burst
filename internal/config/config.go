package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config represents the complete application configuration
type Config struct {
	AWS       AWSConfig     `mapstructure:"aws"`
	Slurm     SlurmConfig   `mapstructure:"slurm"`
	ASBA      ASBAConfig    `mapstructure:"asba"`
	ASBB      ASBBConfig    `mapstructure:"asbb"`
	MPI       MPIConfig     `mapstructure:"mpi"`
	Logging   LoggingConfig `mapstructure:"logging"`
	Ecosystem EcosystemConfig `mapstructure:"ecosystem"`
}

// EcosystemConfig contains ecosystem-wide configuration
type EcosystemConfig struct {
	AutoDetect         bool   `mapstructure:"auto_detect"`
	SharedConfigPath   string `mapstructure:"shared_config_path"`   // Optional shared config across all tools
	DataExchangeDir    string `mapstructure:"data_exchange_dir"`    // Directory for inter-tool communication
	EnableCrossProject bool   `mapstructure:"enable_cross_project"` // Enable cross-project features
}

// AWSConfig contains AWS-specific configuration
type AWSConfig struct {
	Region           string `mapstructure:"region"`
	Profile          string `mapstructure:"profile"`
	RetryMaxAttempts int    `mapstructure:"retry_max_attempts"`
	RetryMode        string `mapstructure:"retry_mode"`

	// Modern authentication configuration
	AuthenticationMethod string                 `mapstructure:"authentication_method"`
	AssumeRole          *AssumeRoleConfig      `mapstructure:"assume_role"`
	SSO                 *SSOConfig             `mapstructure:"sso"`
	WebIdentity         *WebIdentityConfig     `mapstructure:"web_identity"`
	CrossAccount        *CrossAccountConfig    `mapstructure:"cross_account"`
	AccessKeys          *AccessKeysConfig      `mapstructure:"access_keys"`
	TokenRefresh        *TokenRefreshConfig    `mapstructure:"token_refresh"`
}

// AccessKeysConfig contains static access key configuration (DISCOURAGED)
type AccessKeysConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
}

// AssumeRoleConfig contains STS AssumeRole configuration
type AssumeRoleConfig struct {
	RoleARN         string `mapstructure:"role_arn"`
	SessionName     string `mapstructure:"session_name"`
	DurationSeconds int32  `mapstructure:"duration_seconds"`
	ExternalID      string `mapstructure:"external_id"`
	Policy          string `mapstructure:"policy"`
}

// SSOConfig contains AWS IAM Identity Center configuration
type SSOConfig struct {
	ProfileName string `mapstructure:"profile_name"`
	StartURL    string `mapstructure:"start_url"`
	AccountID   string `mapstructure:"account_id"`
	RoleName    string `mapstructure:"role_name"`
}

// WebIdentityConfig contains Web Identity Federation configuration
type WebIdentityConfig struct {
	RoleARN     string `mapstructure:"role_arn"`
	TokenFile   string `mapstructure:"token_file"`
	SessionName string `mapstructure:"session_name"`
}

// CrossAccountConfig contains cross-account role assumption configuration
type CrossAccountConfig struct {
	SourceProfile string `mapstructure:"source_profile"`
	TargetRoleARN string `mapstructure:"target_role_arn"`
	ExternalID    string `mapstructure:"external_id"`
	SessionName   string `mapstructure:"session_name"`
}

// TokenRefreshConfig contains token refresh configuration
type TokenRefreshConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	RefreshInterval int  `mapstructure:"refresh_interval_minutes"`
	PreExpireBuffer int  `mapstructure:"pre_expire_buffer_minutes"`
}

// SlurmConfig contains Slurm integration configuration
type SlurmConfig struct {
	BinPath        string            `mapstructure:"bin_path"`
	ConfigPath     string            `mapstructure:"config_path"`
	PrivateData    string            `mapstructure:"private_data"`
	ResumeProgram  string            `mapstructure:"resume_program"`
	SuspendProgram string            `mapstructure:"suspend_program"`
	ResumeRate     int               `mapstructure:"resume_rate"`
	SuspendRate    int               `mapstructure:"suspend_rate"`
	ResumeTimeout  int               `mapstructure:"resume_timeout"`
	SuspendTime    int               `mapstructure:"suspend_time"`
	TreeWidth      int               `mapstructure:"tree_width"`
	Partitions     []PartitionConfig `mapstructure:"partitions"`
}

// PartitionConfig defines Slurm partition configuration
type PartitionConfig struct {
	PartitionName    string            `mapstructure:"partition_name"`
	NodeGroups       []NodeGroupConfig `mapstructure:"node_groups"`
	PartitionOptions map[string]string `mapstructure:"partition_options"`
}

// NodeGroupConfig defines node group configuration within a partition
type NodeGroupConfig struct {
	NodeGroupName           string                   `mapstructure:"node_group_name"`
	MaxNodes                int                      `mapstructure:"max_nodes"`
	Region                  string                   `mapstructure:"region"`
	SlurmSpecifications     map[string]string        `mapstructure:"slurm_specifications"`
	PurchasingOption        string                   `mapstructure:"purchasing_option"` // "spot" or "on-demand"
	OnDemandOptions         map[string]interface{}   `mapstructure:"on_demand_options"`
	SpotOptions             map[string]interface{}   `mapstructure:"spot_options"`
	LaunchTemplateSpec      LaunchTemplateSpec       `mapstructure:"launch_template_specification"`
	LaunchTemplateOverrides []LaunchTemplateOverride `mapstructure:"launch_template_overrides"`
	SubnetIds               []string                 `mapstructure:"subnet_ids"`
	SecurityGroupIds        []string                 `mapstructure:"security_group_ids"`
	IAMInstanceProfile      string                   `mapstructure:"iam_instance_profile"`
	Tags                    []AWSTag                 `mapstructure:"tags"`
}

// LaunchTemplateSpec defines EC2 launch template specification
type LaunchTemplateSpec struct {
	LaunchTemplateName string `mapstructure:"launch_template_name"`
	LaunchTemplateID   string `mapstructure:"launch_template_id"`
	Version            string `mapstructure:"version"`
}

// LaunchTemplateOverride defines instance type overrides for EC2 Fleet
type LaunchTemplateOverride struct {
	InstanceType     string  `mapstructure:"instance_type"`
	SpotPrice        string  `mapstructure:"spot_price"`
	SubnetID         string  `mapstructure:"subnet_id"`
	WeightedCapacity float64 `mapstructure:"weighted_capacity"`
}

// AWSTag represents an AWS resource tag
type AWSTag struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

// ASBAConfig contains configuration for ASBA integration
type ASBAConfig struct {
	Enabled    string `mapstructure:"enabled"`    // "auto-detect", "true", "false"
	Command    string `mapstructure:"command"`
	ConfigPath string `mapstructure:"config_path"`
	Timeout    int    `mapstructure:"timeout_seconds"`
}

// ASBBConfig contains configuration for ASBB integration
type ASBBConfig struct {
	Enabled           string `mapstructure:"enabled"`             // "auto-detect", "true", "false"
	Command           string `mapstructure:"command"`
	ReconciliationDir string `mapstructure:"reconciliation_dir"`
	Timeout           int    `mapstructure:"timeout_seconds"`
}

// MPIConfig contains MPI-specific configuration
type MPIConfig struct {
	EFADefault               string `mapstructure:"efa_default"` // "required", "preferred", "optional", "disabled"
	HPCInstancesThreshold    int    `mapstructure:"hpc_instances_threshold"`
	PlacementGroupThreshold  int    `mapstructure:"placement_group_threshold"`
	ForceClusterPG           bool   `mapstructure:"force_cluster_placement_group"`
	EnableEnhancedNetworking bool   `mapstructure:"enable_enhanced_networking"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"` // "json" or "text"
	File     string `mapstructure:"file"`
	MaxSize  int    `mapstructure:"max_size_mb"`
	MaxAge   int    `mapstructure:"max_age_days"`
	Compress bool   `mapstructure:"compress"`
}

// Load loads configuration from the specified file path
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)

	// Set defaults based on original AWS plugin patterns
	setDefaults()

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate and normalize configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	normalize(&config)

	return &config, nil
}

// setDefaults sets default configuration values following original plugin patterns
func setDefaults() {
	// AWS defaults (no default region - it's required)
	viper.SetDefault("aws.retry_max_attempts", 3)
	viper.SetDefault("aws.retry_mode", "adaptive")
	viper.SetDefault("aws.authentication_method", "instance_profile") // Secure default for AWS deployments

	// Slurm defaults (from original plugin)
	viper.SetDefault("slurm.bin_path", "/usr/bin")
	viper.SetDefault("slurm.config_path", "/etc/slurm/slurm.conf")
	viper.SetDefault("slurm.private_data", "CLOUD")
	viper.SetDefault("slurm.resume_rate", 100)
	viper.SetDefault("slurm.suspend_rate", 100)
	viper.SetDefault("slurm.resume_timeout", 300)
	viper.SetDefault("slurm.suspend_time", 350)
	viper.SetDefault("slurm.tree_width", 60000)

	// ASBA defaults
	viper.SetDefault("asba.enabled", "auto-detect")
	viper.SetDefault("asba.command", "asba")
	viper.SetDefault("asba.timeout_seconds", 30)

	// ASBB defaults
	viper.SetDefault("asbb.enabled", "auto-detect")
	viper.SetDefault("asbb.command", "asbb")
	viper.SetDefault("asbb.reconciliation_dir", "/var/spool/asbb/costs")
	viper.SetDefault("asbb.timeout_seconds", 30)

	// Ecosystem defaults
	viper.SetDefault("ecosystem.auto_detect", true)
	viper.SetDefault("ecosystem.data_exchange_dir", "/var/spool/asbx/ecosystem")
	viper.SetDefault("ecosystem.enable_cross_project", true)

	// MPI defaults
	viper.SetDefault("mpi.efa_default", "preferred")
	viper.SetDefault("mpi.hpc_instances_threshold", 8)
	viper.SetDefault("mpi.placement_group_threshold", 2)
	viper.SetDefault("mpi.force_cluster_placement_group", false)
	viper.SetDefault("mpi.enable_enhanced_networking", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.file", "/var/log/slurm/aws-burst.log")
	viper.SetDefault("logging.max_size_mb", 100)
	viper.SetDefault("logging.max_age_days", 30)
	viper.SetDefault("logging.compress", true)
}

// validate performs comprehensive configuration validation following original plugin patterns
func validate(config *Config) error {
	if err := validateAWS(&config.AWS); err != nil {
		return err
	}
	if err := validateSlurm(&config.Slurm); err != nil {
		return err
	}
	if err := validateMPI(&config.MPI); err != nil {
		return err
	}
	if err := validateLogging(&config.Logging); err != nil {
		return err
	}
	return nil
}

// validateAWS validates AWS configuration
func validateAWS(aws *AWSConfig) error {
	if aws.Region == "" {
		return fmt.Errorf("aws.region is required")
	}
	return nil
}

// validateSlurm validates Slurm configuration
func validateSlurm(slurm *SlurmConfig) error {
	if slurm.BinPath == "" {
		return fmt.Errorf("slurm.bin_path is required")
	}
	if slurm.PrivateData != "CLOUD" {
		return fmt.Errorf("slurm.private_data must be 'CLOUD' for power save nodes to be visible")
	}
	if err := validateSlurmRates(slurm); err != nil {
		return err
	}
	return validatePartitions(slurm.Partitions)
}

// validateSlurmRates validates Slurm rate configurations
func validateSlurmRates(slurm *SlurmConfig) error {
	if slurm.ResumeRate <= 0 || slurm.ResumeRate > 1000 {
		return fmt.Errorf("slurm.resume_rate must be between 1 and 1000")
	}
	if slurm.SuspendRate <= 0 || slurm.SuspendRate > 1000 {
		return fmt.Errorf("slurm.suspend_rate must be between 1 and 1000")
	}
	if slurm.ResumeTimeout <= 0 {
		return fmt.Errorf("slurm.resume_timeout must be positive")
	}
	if slurm.SuspendTime <= 0 {
		return fmt.Errorf("slurm.suspend_time must be positive")
	}
	return nil
}

// validatePartitions validates partition configurations
func validatePartitions(partitions []PartitionConfig) error {
	if len(partitions) == 0 {
		return fmt.Errorf("at least one partition must be configured")
	}
	for i, partition := range partitions {
		if err := validatePartition(partition, i); err != nil {
			return err
		}
	}
	return nil
}

// validateMPI validates MPI configuration
func validateMPI(mpi *MPIConfig) error {
	validEFAOptions := []string{"required", "preferred", "optional", "disabled"}
	for _, option := range validEFAOptions {
		if mpi.EFADefault == option {
			return nil
		}
	}
	return fmt.Errorf("mpi.efa_default must be one of: %s", strings.Join(validEFAOptions, ", "))
}

// validateLogging validates logging configuration
func validateLogging(logging *LoggingConfig) error {
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	for _, level := range validLogLevels {
		if logging.Level == level {
			return nil
		}
	}
	return fmt.Errorf("logging.level must be one of: %s", strings.Join(validLogLevels, ", "))
}

// validatePartition validates partition configuration following original plugin patterns
func validatePartition(partition PartitionConfig, index int) error {
	if partition.PartitionName == "" {
		return fmt.Errorf("partitions[%d].partition_name is required", index)
	}

	// Validate partition name format (alphanumeric only, like original plugin)
	for _, r := range partition.PartitionName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("partitions[%d].partition_name must contain only alphanumeric characters", index)
		}
	}

	if len(partition.NodeGroups) == 0 {
		return fmt.Errorf("partitions[%d].node_groups cannot be empty", index)
	}

	for j, nodeGroup := range partition.NodeGroups {
		if err := validateNodeGroup(nodeGroup, index, j); err != nil {
			return err
		}
	}

	return nil
}

// validateNodeGroup validates node group configuration following original plugin patterns
func validateNodeGroup(nodeGroup NodeGroupConfig, partitionIndex, nodeGroupIndex int) error {
	if nodeGroup.NodeGroupName == "" {
		return fmt.Errorf("partitions[%d].node_groups[%d].node_group_name is required", partitionIndex, nodeGroupIndex)
	}

	// Validate node group name format (alphanumeric only, like original plugin)
	for _, r := range nodeGroup.NodeGroupName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("partitions[%d].node_groups[%d].node_group_name must contain only alphanumeric characters", partitionIndex, nodeGroupIndex)
		}
	}

	if nodeGroup.MaxNodes <= 0 {
		return fmt.Errorf("partitions[%d].node_groups[%d].max_nodes must be positive", partitionIndex, nodeGroupIndex)
	}

	if nodeGroup.Region == "" {
		return fmt.Errorf("partitions[%d].node_groups[%d].region is required", partitionIndex, nodeGroupIndex)
	}

	if nodeGroup.PurchasingOption != "spot" && nodeGroup.PurchasingOption != "on-demand" {
		return fmt.Errorf("partitions[%d].node_groups[%d].purchasing_option must be 'spot' or 'on-demand'", partitionIndex, nodeGroupIndex)
	}

	if len(nodeGroup.LaunchTemplateOverrides) == 0 {
		return fmt.Errorf("partitions[%d].node_groups[%d].launch_template_overrides cannot be empty", partitionIndex, nodeGroupIndex)
	}

	if len(nodeGroup.SubnetIds) == 0 {
		return fmt.Errorf("partitions[%d].node_groups[%d].subnet_ids cannot be empty", partitionIndex, nodeGroupIndex)
	}

	return nil
}

// normalize performs configuration normalization following original plugin patterns
func normalize(config *Config) {
	// Ensure bin path ends with slash (like original plugin)
	if config.Slurm.BinPath != "" && !strings.HasSuffix(config.Slurm.BinPath, "/") {
		config.Slurm.BinPath += "/"
	}

	// Ensure log file directory exists
	if config.Logging.File != "" {
		dir := filepath.Dir(config.Logging.File)
		if err := os.MkdirAll(dir, 0755); err != nil {
			// Log directory creation failure, but don't fail configuration loading
			fmt.Printf("Warning: failed to create log directory %s: %v\n", dir, err)
		}
	}
}

// GetNodeName generates node names following original plugin pattern: [partition]-[nodegroup]-[id]
func (c *Config) GetNodeName(partitionName, nodeGroupName string, nodeID string) string {
	if nodeID == "" {
		return fmt.Sprintf("%s-%s", partitionName, nodeGroupName)
	}
	return fmt.Sprintf("%s-%s-%s", partitionName, nodeGroupName, nodeID)
}

// GetNodeRange generates node ranges following original plugin pattern: [partition]-[nodegroup]-[0-N]
func (c *Config) GetNodeRange(partitionName, nodeGroupName string, maxNodes int) string {
	if maxNodes > 1 {
		return fmt.Sprintf("%s-%s-[0-%d]", partitionName, nodeGroupName, maxNodes-1)
	}
	return fmt.Sprintf("%s-%s-0", partitionName, nodeGroupName)
}

// FindNodeGroup finds a node group configuration by partition and node group name
func (c *Config) FindNodeGroup(partitionName, nodeGroupName string) *NodeGroupConfig {
	for _, partition := range c.Slurm.Partitions {
		if partition.PartitionName == partitionName {
			for _, nodeGroup := range partition.NodeGroups {
				if nodeGroup.NodeGroupName == nodeGroupName {
					return &nodeGroup
				}
			}
		}
	}
	return nil
}

// SetupLogger creates a zap logger with the configured settings
func (c *Config) SetupLogger() (*zap.Logger, error) {
	var logger *zap.Logger
	var err error

	switch c.Logging.Format {
	case "json":
		if c.Logging.Level == "debug" {
			logger, err = zap.NewDevelopment()
		} else {
			logger, err = zap.NewProduction()
		}
	case "text":
		config := zap.NewDevelopmentConfig()
		config.Encoding = "console"
		logger, err = config.Build()
	default:
		logger, err = zap.NewProduction()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return logger, nil
}
