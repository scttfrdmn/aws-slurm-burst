package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.uber.org/zap"
)

// AuthenticationMethod represents different AWS authentication approaches
type AuthenticationMethod string

const (
	AuthMethodDefault       AuthenticationMethod = "default"        // Default credential chain
	AuthMethodInstanceProfile AuthenticationMethod = "instance_profile" // EC2 instance profile (recommended)
	AuthMethodAssumeRole    AuthenticationMethod = "assume_role"    // STS AssumeRole
	AuthMethodSSO           AuthenticationMethod = "sso"            // AWS IAM Identity Center
	AuthMethodWebIdentity   AuthenticationMethod = "web_identity"   // Web Identity Federation
	AuthMethodCrossAccount  AuthenticationMethod = "cross_account"  // Cross-account role assumption
	AuthMethodProfile       AuthenticationMethod = "profile"        // Named AWS profile
	AuthMethodAccessKeys    AuthenticationMethod = "access_keys"    // Static access keys (DISCOURAGED)
)

// AuthenticationConfig contains authentication configuration
type AuthenticationConfig struct {
	Method          AuthenticationMethod  `mapstructure:"authentication_method"`
	Profile         string               `mapstructure:"profile"`
	AssumeRole      *AssumeRoleConfig    `mapstructure:"assume_role"`
	SSO             *SSOConfig           `mapstructure:"sso"`
	WebIdentity     *WebIdentityConfig   `mapstructure:"web_identity"`
	CrossAccount    *CrossAccountConfig  `mapstructure:"cross_account"`
	AccessKeys      *AccessKeysConfig    `mapstructure:"access_keys"`
	TokenRefresh    *TokenRefreshConfig  `mapstructure:"token_refresh"`
}

// AccessKeysConfig contains static access key configuration (DISCOURAGED)
type AccessKeysConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token,omitempty"`
}

// AssumeRoleConfig contains STS AssumeRole configuration
type AssumeRoleConfig struct {
	RoleARN         string `mapstructure:"role_arn"`
	SessionName     string `mapstructure:"session_name"`
	DurationSeconds int32  `mapstructure:"duration_seconds"`
	ExternalID      string `mapstructure:"external_id,omitempty"`
	Policy          string `mapstructure:"policy,omitempty"`
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
	RoleARN   string `mapstructure:"role_arn"`
	TokenFile string `mapstructure:"token_file"`
	SessionName string `mapstructure:"session_name"`
}

// CrossAccountConfig contains cross-account role assumption configuration
type CrossAccountConfig struct {
	SourceProfile   string `mapstructure:"source_profile"`
	TargetRoleARN   string `mapstructure:"target_role_arn"`
	ExternalID      string `mapstructure:"external_id"`
	SessionName     string `mapstructure:"session_name"`
}

// TokenRefreshConfig contains token refresh configuration
type TokenRefreshConfig struct {
	Enabled         bool  `mapstructure:"enabled"`
	RefreshInterval int   `mapstructure:"refresh_interval_minutes"`
	PreExpireBuffer int   `mapstructure:"pre_expire_buffer_minutes"`
}

// AuthenticationProvider handles various AWS authentication methods
type AuthenticationProvider struct {
	logger *zap.Logger
	config *AuthenticationConfig
}

// NewAuthenticationProvider creates a new authentication provider
func NewAuthenticationProvider(logger *zap.Logger, authConfig *AuthenticationConfig) *AuthenticationProvider {
	return &AuthenticationProvider{
		logger: logger,
		config: authConfig,
	}
}

// GetAWSConfig returns an AWS config with the specified authentication method
func (a *AuthenticationProvider) GetAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	a.logger.Info("Configuring AWS authentication",
		zap.String("method", string(a.config.Method)),
		zap.String("region", region))

	switch a.config.Method {
	case AuthMethodInstanceProfile:
		return a.getInstanceProfileConfig(ctx, region)
	case AuthMethodAssumeRole:
		return a.getAssumeRoleConfig(ctx, region)
	case AuthMethodSSO:
		return a.getSSOConfig(ctx, region)
	case AuthMethodWebIdentity:
		return a.getWebIdentityConfig(ctx, region)
	case AuthMethodCrossAccount:
		return a.getCrossAccountConfig(ctx, region)
	case AuthMethodProfile:
		return a.getProfileConfig(ctx, region)
	case AuthMethodAccessKeys:
		return a.getAccessKeysConfig(ctx, region)
	case AuthMethodDefault:
		return a.getDefaultConfig(ctx, region)
	default:
		return aws.Config{}, fmt.Errorf("unsupported authentication method: %s", a.config.Method)
	}
}

// getInstanceProfileConfig uses EC2 instance profile for authentication
func (a *AuthenticationProvider) getInstanceProfileConfig(ctx context.Context, region string) (aws.Config, error) {
	a.logger.Info("Using EC2 instance profile authentication")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithEC2IMDSRegion(), // Use IMDS for region if not specified
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load instance profile config: %w", err)
	}

	// Validate instance profile access
	if err := a.validateCredentials(ctx, cfg); err != nil {
		return aws.Config{}, fmt.Errorf("instance profile validation failed: %w", err)
	}

	a.logger.Info("EC2 instance profile authentication configured successfully")
	return cfg, nil
}

// getAssumeRoleConfig uses STS AssumeRole for authentication
func (a *AuthenticationProvider) getAssumeRoleConfig(ctx context.Context, region string) (aws.Config, error) {
	if a.config.AssumeRole == nil {
		return aws.Config{}, fmt.Errorf("assume_role configuration required")
	}

	a.logger.Info("Using STS AssumeRole authentication",
		zap.String("role_arn", a.config.AssumeRole.RoleARN),
		zap.String("session_name", a.config.AssumeRole.SessionName))

	// Load base config
	baseCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load base config: %w", err)
	}

	// Create STS client for role assumption
	stsClient := sts.NewFromConfig(baseCfg)

	// Configure assume role credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(stscreds.NewAssumeRoleProvider(stsClient, a.config.AssumeRole.RoleARN, func(options *stscreds.AssumeRoleOptions) {
			options.RoleSessionName = a.config.AssumeRole.SessionName
			if a.config.AssumeRole.DurationSeconds > 0 {
				options.Duration = time.Duration(a.config.AssumeRole.DurationSeconds) * time.Second
			}
			if a.config.AssumeRole.ExternalID != "" {
				options.ExternalID = aws.String(a.config.AssumeRole.ExternalID)
			}
			if a.config.AssumeRole.Policy != "" {
				options.Policy = aws.String(a.config.AssumeRole.Policy)
			}
		})),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to configure assume role: %w", err)
	}

	a.logger.Info("STS AssumeRole authentication configured successfully")
	return cfg, nil
}

// getSSOConfig uses AWS IAM Identity Center (SSO) for authentication
func (a *AuthenticationProvider) getSSOConfig(ctx context.Context, region string) (aws.Config, error) {
	if a.config.SSO == nil {
		return aws.Config{}, fmt.Errorf("sso configuration required")
	}

	a.logger.Info("Using AWS IAM Identity Center (SSO) authentication",
		zap.String("profile", a.config.SSO.ProfileName),
		zap.String("account_id", a.config.SSO.AccountID))

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(a.config.SSO.ProfileName),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load SSO config: %w", err)
	}

	a.logger.Info("AWS IAM Identity Center authentication configured successfully")
	return cfg, nil
}

// getWebIdentityConfig uses Web Identity Federation (for Kubernetes/containers)
func (a *AuthenticationProvider) getWebIdentityConfig(ctx context.Context, region string) (aws.Config, error) {
	if a.config.WebIdentity == nil {
		return aws.Config{}, fmt.Errorf("web_identity configuration required")
	}

	a.logger.Info("Using Web Identity Federation authentication",
		zap.String("role_arn", a.config.WebIdentity.RoleARN),
		zap.String("token_file", a.config.WebIdentity.TokenFile))

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithWebIdentityRoleCredentialOptions(func(options *stscreds.WebIdentityRoleOptions) {
			options.RoleARN = a.config.WebIdentity.RoleARN
			options.TokenRetriever = stscreds.IdentityTokenFile(a.config.WebIdentity.TokenFile)
			options.RoleSessionName = a.config.WebIdentity.SessionName
		}),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to configure web identity: %w", err)
	}

	a.logger.Info("Web Identity Federation authentication configured successfully")
	return cfg, nil
}

// getCrossAccountConfig uses cross-account role assumption
func (a *AuthenticationProvider) getCrossAccountConfig(ctx context.Context, region string) (aws.Config, error) {
	if a.config.CrossAccount == nil {
		return aws.Config{}, fmt.Errorf("cross_account configuration required")
	}

	a.logger.Info("Using cross-account role assumption",
		zap.String("source_profile", a.config.CrossAccount.SourceProfile),
		zap.String("target_role", a.config.CrossAccount.TargetRoleARN))

	// Load source account credentials
	sourceCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(a.config.CrossAccount.SourceProfile),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load source profile: %w", err)
	}

	// Create STS client with source credentials
	stsClient := sts.NewFromConfig(sourceCfg)

	// Configure cross-account role assumption
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(stscreds.NewAssumeRoleProvider(stsClient, a.config.CrossAccount.TargetRoleARN, func(options *stscreds.AssumeRoleOptions) {
			options.RoleSessionName = a.config.CrossAccount.SessionName
			if a.config.CrossAccount.ExternalID != "" {
				options.ExternalID = aws.String(a.config.CrossAccount.ExternalID)
			}
		})),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to configure cross-account role: %w", err)
	}

	a.logger.Info("Cross-account role assumption configured successfully")
	return cfg, nil
}

// getProfileConfig uses named AWS profile
func (a *AuthenticationProvider) getProfileConfig(ctx context.Context, region string) (aws.Config, error) {
	profile := a.config.Profile
	if profile == "" {
		profile = "default"
	}

	a.logger.Info("Using AWS profile authentication", zap.String("profile", profile))

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithSharedConfigProfile(profile),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load profile config: %w", err)
	}

	a.logger.Info("AWS profile authentication configured successfully")
	return cfg, nil
}

// getAccessKeysConfig uses static access keys (DISCOURAGED)
func (a *AuthenticationProvider) getAccessKeysConfig(ctx context.Context, region string) (aws.Config, error) {
	if a.config.AccessKeys == nil {
		return aws.Config{}, fmt.Errorf("access_keys configuration required")
	}

	// Security warning for access keys
	a.logger.Warn("⚠️  Using static access keys - SECURITY RISK",
		zap.String("recommendation", "Use instance_profile, assume_role, or sso instead"))
	a.logger.Warn("⚠️  Access keys should only be used for development or legacy compatibility")

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     a.config.AccessKeys.AccessKeyID,
				SecretAccessKey: a.config.AccessKeys.SecretAccessKey,
				SessionToken:    a.config.AccessKeys.SessionToken,
			}, nil
		}))),
	)

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to configure access keys: %w", err)
	}

	// Validate credentials work
	if err := a.validateCredentials(ctx, cfg); err != nil {
		return aws.Config{}, fmt.Errorf("access key validation failed: %w", err)
	}

	a.logger.Warn("Static access key authentication configured - consider migrating to IAM roles")
	return cfg, nil
}

// getDefaultConfig uses default AWS credential chain
func (a *AuthenticationProvider) getDefaultConfig(ctx context.Context, region string) (aws.Config, error) {
	a.logger.Info("Using default AWS credential chain")

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load default config: %w", err)
	}

	a.logger.Info("Default credential chain configured successfully")
	return cfg, nil
}

// validateCredentials validates that credentials work and have required permissions
func (a *AuthenticationProvider) validateCredentials(ctx context.Context, cfg aws.Config) error {
	stsClient := sts.NewFromConfig(cfg)

	// Get caller identity to validate credentials
	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("credential validation failed: %w", err)
	}

	a.logger.Info("AWS credentials validated",
		zap.String("account", aws.ToString(result.Account)),
		zap.String("arn", aws.ToString(result.Arn)),
		zap.String("user_id", aws.ToString(result.UserId)))

	return nil
}

// RefreshCredentials refreshes temporary credentials if needed
func (a *AuthenticationProvider) RefreshCredentials(ctx context.Context, cfg aws.Config) (aws.Config, error) {
	// Check if token refresh is enabled and needed
	if a.config.TokenRefresh == nil || !a.config.TokenRefresh.Enabled {
		return cfg, nil
	}

	// For methods that use temporary credentials, implement refresh logic
	switch a.config.Method {
	case AuthMethodAssumeRole, AuthMethodCrossAccount:
		a.logger.Debug("Refreshing temporary credentials")
		return a.GetAWSConfig(ctx, cfg.Region)
	default:
		// No refresh needed for instance profiles or SSO
		return cfg, nil
	}
}

// GetCredentialInfo returns information about current credentials (for debugging/monitoring)
func (a *AuthenticationProvider) GetCredentialInfo(ctx context.Context, cfg aws.Config) (*CredentialInfo, error) {
	stsClient := sts.NewFromConfig(cfg)

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get credential info: %w", err)
	}

	return &CredentialInfo{
		Account:    aws.ToString(result.Account),
		ARN:        aws.ToString(result.Arn),
		UserID:     aws.ToString(result.UserId),
		Method:     string(a.config.Method),
		ValidatedAt: time.Now(),
	}, nil
}

// CredentialInfo contains information about current AWS credentials
type CredentialInfo struct {
	Account     string    `json:"account"`
	ARN         string    `json:"arn"`
	UserID      string    `json:"user_id"`
	Method      string    `json:"method"`
	ValidatedAt time.Time `json:"validated_at"`
}