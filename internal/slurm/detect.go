package slurm

import (
	"context"
	"os/exec"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"go.uber.org/zap"
)

// DetectSlurmAvailability checks if Slurm is available and working
func DetectSlurmAvailability(logger *zap.Logger, slurmConfig *config.SlurmConfig) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if scontrol command exists and is executable
	scontrolPath := slurmConfig.BinPath + "scontrol"
	cmd := exec.CommandContext(ctx, scontrolPath, "--version")

	if err := cmd.Run(); err != nil {
		logger.Debug("Slurm not available",
			zap.String("scontrol_path", scontrolPath),
			zap.Error(err))
		return false
	}

	logger.Debug("Slurm availability confirmed", zap.String("scontrol_path", scontrolPath))
	return true
}

// DetectSlurmDaemon checks if Slurm daemon is running and accessible
func DetectSlurmDaemon(logger *zap.Logger, slurmConfig *config.SlurmConfig) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to get cluster information
	scontrolPath := slurmConfig.BinPath + "scontrol"
	cmd := exec.CommandContext(ctx, scontrolPath, "show", "config")

	if err := cmd.Run(); err != nil {
		logger.Debug("Slurm daemon not accessible",
			zap.String("scontrol_path", scontrolPath),
			zap.Error(err))
		return false
	}

	logger.Info("Slurm daemon is accessible")
	return true
}
