package slurm

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/scttfrdmn/aws-slurm-burst/internal/config"
	"github.com/scttfrdmn/aws-slurm-burst/pkg/types"
	"go.uber.org/zap"
)

// Client provides Slurm integration functionality following original plugin patterns
type Client struct {
	logger *zap.Logger
	config *config.SlurmConfig
}

// NodeInfo represents information about a Slurm node
type NodeInfo struct {
	NodeName string
	State    string
	Reason   string
}

// NewClient creates a new Slurm client
func NewClient(logger *zap.Logger, slurmConfig *config.SlurmConfig) *Client {
	return &Client{
		logger: logger,
		config: slurmConfig,
	}
}

// ParseNodeList expands a Slurm hostlist using scontrol show hostnames (following original plugin)
func (c *Client) ParseNodeList(hostlist string) ([]string, error) {
	cmd := exec.Command(c.config.BinPath+"scontrol", "show", "hostnames", hostlist)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to expand hostlist '%s': %w", hostlist, err)
	}

	var nodes []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		node := strings.TrimSpace(scanner.Text())
		if node != "" {
			nodes = append(nodes, node)
		}
	}

	c.logger.Debug("Expanded node list",
		zap.String("hostlist", hostlist),
		zap.Strings("nodes", nodes))

	return nodes, nil
}

// ParseNodeNames parses node names into partition/nodegroup structure (following original plugin pattern)
func (c *Client) ParseNodeNames(nodeNames []string) map[string]map[string][]string {
	result := make(map[string]map[string][]string)

	// Pattern matches: partition-nodegroup-id (e.g., aws-gpu-001)
	pattern := regexp.MustCompile(`^([a-zA-Z0-9]+)-([a-zA-Z0-9]+)-([0-9]+)$`)

	for _, nodeName := range nodeNames {
		matches := pattern.FindStringSubmatch(nodeName)
		if len(matches) != 4 {
			c.logger.Warn("Invalid node name format", zap.String("node", nodeName))
			continue
		}

		partitionName := matches[1]
		nodeGroupName := matches[2]
		nodeID := matches[3]

		if result[partitionName] == nil {
			result[partitionName] = make(map[string][]string)
		}
		result[partitionName][nodeGroupName] = append(result[partitionName][nodeGroupName], nodeID)
	}

	c.logger.Debug("Parsed node names", zap.Any("result", result))
	return result
}

// GetJobForNodes attempts to find the job associated with the given nodes
func (c *Client) GetJobForNodes(ctx context.Context, nodeIds []string) (*types.SlurmJob, error) {
	// Try to get job information from squeue
	cmd := exec.CommandContext(ctx, c.config.BinPath+"squeue", "-w", strings.Join(nodeIds, ","), "-o", "%i,%j,%P,%D,%C,%m,%t,%S,%L", "--noheader")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs for nodes: %w", err)
	}

	if strings.TrimSpace(string(output)) == "" {
		return nil, fmt.Errorf("no job found for nodes")
	}

	// Parse first job found (format: jobid,name,partition,nodes,cpus,memory,state,start_time,time_limit)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	fields := strings.Split(lines[0], ",")
	if len(fields) < 9 {
		return nil, fmt.Errorf("invalid squeue output format")
	}

	jobID := strings.TrimSpace(fields[0])
	jobName := strings.TrimSpace(fields[1])
	partition := strings.TrimSpace(fields[2])

	nodes, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
	cpus, _ := strconv.Atoi(strings.TrimSpace(fields[4]))
	memory := c.parseMemory(strings.TrimSpace(fields[5]))

	job := &types.SlurmJob{
		JobID:     jobID,
		Name:      jobName,
		Partition: partition,
		NodeList:  nodeIds,
		Resources: types.ResourceSpec{
			Nodes:       nodes,
			CPUsPerNode: cpus / nodes, // Approximate
			MemoryMB:    memory,
		},
		Constraints: types.JobConstraints{},
		IsMPIJob:    false, // Will be determined by MPI analyzer
		MPITopology: types.TopologyAny,
		SubmitTime:  time.Now(), // Approximation
	}

	// Try to get job script for better analysis
	if script, err := c.getJobScript(ctx, jobID); err == nil {
		job.Script = script
		c.parseJobScript(job)
	}

	c.logger.Debug("Retrieved job information",
		zap.String("job_id", jobID),
		zap.String("name", jobName),
		zap.String("partition", partition),
		zap.Int("nodes", nodes),
		zap.Int("cpus", cpus))

	return job, nil
}

// getJobScript attempts to retrieve the job script
func (c *Client) getJobScript(ctx context.Context, jobID string) (string, error) {
	cmd := exec.CommandContext(ctx, c.config.BinPath+"scontrol", "show", "job", jobID)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Extract command from scontrol output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Command=") {
			parts := strings.SplitN(line, "Command=", 2)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}

	return "", fmt.Errorf("job script not found")
}

// parseJobScript extracts information from job script
func (c *Client) parseJobScript(job *types.SlurmJob) {
	if job.Script == "" {
		return
	}

	script := strings.ToLower(job.Script)

	// Parse SBATCH directives
	sbatchPattern := regexp.MustCompile(`#SBATCH\s+--([a-zA-Z-]+)(?:[=\s]+([^\s]+))?`)
	matches := sbatchPattern.FindAllStringSubmatch(job.Script, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		directive := match[1]
		value := ""
		if len(match) > 2 {
			value = match[2]
		}

		switch directive {
		case "constraint", "C":
			if value != "" {
				job.Constraints.Features = strings.Split(value, "&")
			}
		case "exclude":
			if value != "" {
				job.Constraints.ExcludeNodes = strings.Split(value, ",")
			}
		case "mem", "mem-per-node":
			if memory := c.parseMemory(value); memory > 0 {
				job.Resources.MemoryMB = memory
			}
		case "gres":
			if strings.Contains(value, "gpu") {
				parts := strings.Split(value, ":")
				if len(parts) >= 2 {
					if gpus, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
						job.Resources.GPUs = gpus
					}
				}
				if len(parts) >= 3 {
					job.Resources.GPUType = parts[1]
				}
			}
		case "time":
			if duration, err := c.parseDuration(value); err == nil {
				job.TimeLimit = types.Duration(duration)
			}
		}
	}

	// Check for MPI indicators in script content
	if strings.Contains(script, "mpirun") || strings.Contains(script, "mpiexec") {
		c.logger.Debug("MPI indicators found in job script", zap.String("job_id", job.JobID))
	}
}

// parseMemory parses memory specification like "16GB", "2048M", etc.
func (c *Client) parseMemory(memStr string) int {
	if memStr == "" {
		return 0
	}

	memStr = strings.ToUpper(strings.TrimSpace(memStr))

	// Extract numeric part
	re := regexp.MustCompile(`^(\d+)([KMGT]?B?)`)
	matches := re.FindStringSubmatch(memStr)
	if len(matches) < 2 {
		return 0
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}

	// Convert to MB
	switch unit {
	case "KB", "K":
		return value / 1024
	case "MB", "M", "":
		return value
	case "GB", "G":
		return value * 1024
	case "TB", "T":
		return value * 1024 * 1024
	}

	return value
}

// parseDuration parses time specification like "02:00:00", "120", etc.
func (c *Client) parseDuration(timeStr string) (time.Duration, error) {
	if timeStr == "" {
		return 0, fmt.Errorf("empty time string")
	}

	// Try different formats
	if strings.Contains(timeStr, ":") {
		// Format like "02:00:00" or "120:00"
		parts := strings.Split(timeStr, ":")
		var hours, minutes, seconds int

		switch len(parts) {
		case 2:
			if h, err := strconv.Atoi(parts[0]); err == nil {
				hours = h
			}
			if m, err := strconv.Atoi(parts[1]); err == nil {
				minutes = m
			}
		case 3:
			if h, err := strconv.Atoi(parts[0]); err == nil {
				hours = h
			}
			if m, err := strconv.Atoi(parts[1]); err == nil {
				minutes = m
			}
			if s, err := strconv.Atoi(parts[2]); err == nil {
				seconds = s
			}
		}

		return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second, nil
	}

	// Try as minutes
	if mins, err := strconv.Atoi(timeStr); err == nil {
		return time.Duration(mins) * time.Minute, nil
	}

	return 0, fmt.Errorf("unable to parse time: %s", timeStr)
}

// UpdateNodesWithInstanceInfo updates Slurm nodes with AWS instance information
func (c *Client) UpdateNodesWithInstanceInfo(ctx context.Context, instances []types.InstanceInfo) error {
	for _, instance := range instances {
		// Update node with instance information
		parameters := fmt.Sprintf("NodeAddr=%s NodeHostname=%s", instance.PrivateIP, instance.PrivateIP)

		if err := c.UpdateNode(instance.NodeName, parameters); err != nil {
			c.logger.Error("Failed to update node",
				zap.String("node", instance.NodeName),
				zap.String("instance_id", instance.InstanceID),
				zap.Error(err))
			continue
		}

		c.logger.Info("Updated node with instance info",
			zap.String("node", instance.NodeName),
			zap.String("instance_id", instance.InstanceID),
			zap.String("private_ip", instance.PrivateIP))
	}

	return nil
}

// UpdateNode updates a Slurm node using scontrol (following original plugin pattern)
func (c *Client) UpdateNode(nodeName, parameters string) error {
	args := []string{"update", "nodename=" + nodeName}
	args = append(args, strings.Split(parameters, " ")...)

	cmd := exec.Command(c.config.BinPath+"scontrol", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update node %s: %w", nodeName, err)
	}

	c.logger.Debug("Updated node",
		zap.String("node", nodeName),
		zap.String("parameters", parameters))

	return nil
}

// GetNodeState retrieves the state of specified nodes
func (c *Client) GetNodeState(nodeNames []string) ([]NodeInfo, error) {
	if len(nodeNames) == 0 {
		return nil, nil
	}

	cmd := exec.Command(c.config.BinPath+"scontrol", "show", "node", strings.Join(nodeNames, ","), "-o")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get node state: %w", err)
	}

	var nodes []NodeInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse scontrol output format
		nodeInfo := NodeInfo{}
		fields := strings.Split(line, " ")

		for _, field := range fields {
			if strings.Contains(field, "=") {
				parts := strings.SplitN(field, "=", 2)
				if len(parts) != 2 {
					continue
				}

				key := parts[0]
				value := parts[1]

				switch key {
				case "NodeName":
					nodeInfo.NodeName = value
				case "State":
					nodeInfo.State = value
				case "Reason":
					nodeInfo.Reason = value
				}
			}
		}

		if nodeInfo.NodeName != "" {
			nodes = append(nodes, nodeInfo)
		}
	}

	return nodes, nil
}

// SetNodeState sets the state of a node (following original change_state.py patterns)
func (c *Client) SetNodeState(nodeName, state, reason string) error {
	parameters := fmt.Sprintf("state=%s", state)
	if reason != "" {
		parameters += fmt.Sprintf(" reason=%s", reason)
	}

	if err := c.UpdateNode(nodeName, parameters); err != nil {
		return fmt.Errorf("failed to set node %s to state %s: %w", nodeName, state, err)
	}

	c.logger.Info("Set node state",
		zap.String("node", nodeName),
		zap.String("state", state),
		zap.String("reason", reason))

	return nil
}