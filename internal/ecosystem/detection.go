package ecosystem

import (
	"context"
	"os/exec"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// EcosystemStatus represents the availability of companion tools
type EcosystemStatus struct {
	ASBA ASBAStatus `json:"asba"`
	ASBB ASBBStatus `json:"asbb"`
}

// ASBAStatus represents ASBA availability and capabilities
type ASBAStatus struct {
	Available        bool   `json:"available"`
	Version          string `json:"version,omitempty"`
	Command          string `json:"command,omitempty"`
	SupportsExecPlan bool   `json:"supports_execution_plan"`
	SupportsBurst    bool   `json:"supports_burst_command"`
	LastChecked      string `json:"last_checked"`
}

// ASBBStatus represents ASBB availability and capabilities
type ASBBStatus struct {
	Available           bool   `json:"available"`
	Version             string `json:"version,omitempty"`
	Command             string `json:"command,omitempty"`
	SupportsReconciliation bool `json:"supports_reconciliation"`
	ReconciliationDir   string `json:"reconciliation_dir,omitempty"`
	LastChecked         string `json:"last_checked"`
}

// EcosystemDetector handles detection of companion tools
type EcosystemDetector struct {
	logger *zap.Logger
}

// NewEcosystemDetector creates a new ecosystem detector
func NewEcosystemDetector(logger *zap.Logger) *EcosystemDetector {
	return &EcosystemDetector{
		logger: logger,
	}
}

// DetectEcosystem checks for availability of companion tools
func (e *EcosystemDetector) DetectEcosystem(ctx context.Context) *EcosystemStatus {
	status := &EcosystemStatus{
		ASBA: e.detectASBA(ctx),
		ASBB: e.detectASBB(ctx),
	}

	e.logger.Info("Ecosystem detection complete",
		zap.Bool("asba_available", status.ASBA.Available),
		zap.Bool("asbb_available", status.ASBB.Available),
		zap.String("asba_version", status.ASBA.Version),
		zap.String("asbb_version", status.ASBB.Version))

	return status
}

// detectASBA checks for ASBA availability and capabilities
func (e *EcosystemDetector) detectASBA(ctx context.Context) ASBAStatus {
	status := ASBAStatus{
		LastChecked: time.Now().Format(time.RFC3339),
	}

	// Check common ASBA command locations
	commands := []string{"asba", "/usr/local/bin/asba", "/usr/bin/asba"}

	for _, cmd := range commands {
		if e.commandExists(cmd) {
			status.Available = true
			status.Command = cmd

			// Get version
			if version := e.getASBAVersion(ctx, cmd); version != "" {
				status.Version = version
			}

			// Check capabilities
			status.SupportsExecPlan = e.checkASBAExecutionPlanSupport(ctx, cmd)
			status.SupportsBurst = e.checkASBABurstSupport(ctx, cmd)

			e.logger.Debug("ASBA detected",
				zap.String("command", cmd),
				zap.String("version", status.Version),
				zap.Bool("exec_plan", status.SupportsExecPlan),
				zap.Bool("burst", status.SupportsBurst))

			break
		}
	}

	if !status.Available {
		e.logger.Debug("ASBA not found - operating in standalone mode")
	}

	return status
}

// detectASBB checks for ASBB availability and capabilities
func (e *EcosystemDetector) detectASBB(ctx context.Context) ASBBStatus {
	status := ASBBStatus{
		LastChecked: time.Now().Format(time.RFC3339),
	}

	// Check common ASBB command locations
	commands := []string{"asbb", "/usr/local/bin/asbb", "/usr/bin/asbb"}

	for _, cmd := range commands {
		if e.commandExists(cmd) {
			status.Available = true
			status.Command = cmd

			// Get version
			if version := e.getASBBVersion(ctx, cmd); version != "" {
				status.Version = version
			}

			// Check capabilities
			status.SupportsReconciliation = e.checkASBBReconciliationSupport(ctx, cmd)

			// Find reconciliation directory
			if reconcileDir := e.findASBBReconciliationDir(); reconcileDir != "" {
				status.ReconciliationDir = reconcileDir
			}

			e.logger.Debug("ASBB detected",
				zap.String("command", cmd),
				zap.String("version", status.Version),
				zap.Bool("reconciliation", status.SupportsReconciliation),
				zap.String("reconcile_dir", status.ReconciliationDir))

			break
		}
	}

	if !status.Available {
		e.logger.Debug("ASBB not found - budget features disabled")
	}

	return status
}

// commandExists checks if a command is available in the system
func (e *EcosystemDetector) commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// getASBAVersion gets the ASBA version
func (e *EcosystemDetector) getASBAVersion(ctx context.Context, command string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(output)
}

// getASBBVersion gets the ASBB version
func (e *EcosystemDetector) getASBBVersion(ctx context.Context, command string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(output)
}

// checkASBAExecutionPlanSupport checks if ASBA supports execution plan generation
func (e *EcosystemDetector) checkASBAExecutionPlanSupport(ctx context.Context, command string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "execution-plan", "--help")
	err := cmd.Run()
	return err == nil
}

// checkASBABurstSupport checks if ASBA supports burst command
func (e *EcosystemDetector) checkASBABurstSupport(ctx context.Context, command string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "burst", "--help")
	err := cmd.Run()
	return err == nil
}

// checkASBBReconciliationSupport checks if ASBB supports cost reconciliation
func (e *EcosystemDetector) checkASBBReconciliationSupport(ctx context.Context, command string) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "reconcile", "--help")
	err := cmd.Run()
	return err == nil
}

// findASBBReconciliationDir finds the ASBB reconciliation directory
func (e *EcosystemDetector) findASBBReconciliationDir() string {
	// Check common ASBB reconciliation directories
	commonDirs := []string{
		"/var/spool/asbb/costs",
		"/var/spool/asbb/reconciliation",
		"/tmp/asbb/reconciliation",
		"/opt/asbb/data/reconciliation",
	}

	for _, dir := range commonDirs {
		if e.directoryExists(dir) {
			return dir
		}
	}

	return ""
}

// directoryExists checks if a directory exists and is accessible
func (e *EcosystemDetector) directoryExists(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	if stat, err := filepath.Glob(absPath); err == nil && len(stat) > 0 {
		return true
	}

	return false
}

// GetEnhancementRecommendations suggests ecosystem improvements based on what's available
func (e *EcosystemDetector) GetEnhancementRecommendations(status *EcosystemStatus) []string {
	var recommendations []string

	if !status.ASBA.Available {
		recommendations = append(recommendations,
			"Install ASBA for intelligent job analysis and cost optimization",
			"See: https://github.com/scttfrdmn/aws-slurm-burst-advisor")
	} else if !status.ASBA.SupportsExecPlan {
		recommendations = append(recommendations,
			"Upgrade ASBA to v0.3.0+ for execution plan support")
	}

	if !status.ASBB.Available {
		recommendations = append(recommendations,
			"Install ASBB for real grant money budget management",
			"See: https://github.com/scttfrdmn/aws-slurm-burst-budget")
	}

	if status.ASBA.Available && status.ASBB.Available {
		recommendations = append(recommendations,
			"Complete ecosystem available - enable integrated workflows",
			"Try: asba burst job.sbatch gpu-aws nodes --account=NSF-ABC123")
	}

	return recommendations
}