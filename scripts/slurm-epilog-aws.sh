#!/bin/bash

# SLURM Epilog Script for AWS Burst Performance Data Collection
# Place this in /etc/slurm/epilog.d/aws-burst-metadata.sh

# Exit if not an AWS partition
if [[ "$SLURM_JOB_PARTITION" != aws-* ]]; then
    exit 0
fi

# Configuration
PLUGIN_DIR="/usr/local/bin"
CONFIG_FILE="/etc/slurm/aws-burst.yaml"
OUTPUT_DIR="/var/spool/asba/learning"
LOG_FILE="/var/log/slurm/aws-burst-epilog.log"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Log epilog execution
echo "$(date): Starting AWS metadata collection for job $SLURM_JOB_ID" >> "$LOG_FILE"

# Export performance data for ASBA learning
"$PLUGIN_DIR/aws-slurm-burst-export-performance" \
    --job-id="$SLURM_JOB_ID" \
    --config="$CONFIG_FILE" \
    --output-dir="$OUTPUT_DIR" \
    --format=asba-learning \
    >> "$LOG_FILE" 2>&1

EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "$(date): Successfully exported performance data for job $SLURM_JOB_ID" >> "$LOG_FILE"

    # Also export Slurm comment format for budget bank integration
    "$PLUGIN_DIR/aws-slurm-burst-export-performance" \
        --job-id="$SLURM_JOB_ID" \
        --config="$CONFIG_FILE" \
        --output-dir="$OUTPUT_DIR" \
        --format=slurm-comment \
        >> "$LOG_FILE" 2>&1
else
    echo "$(date): Failed to export performance data for job $SLURM_JOB_ID (exit code: $EXIT_CODE)" >> "$LOG_FILE"
fi

# Update job comment with basic AWS metadata (if we have the data)
COMMENT_FILE="$OUTPUT_DIR/job-$SLURM_JOB_ID-comment.txt"
if [ -f "$COMMENT_FILE" ]; then
    COMMENT_DATA=$(cat "$COMMENT_FILE")

    # Update Slurm job comment
    if command -v scontrol >/dev/null 2>&1; then
        scontrol update job="$SLURM_JOB_ID" comment="$COMMENT_DATA" >> "$LOG_FILE" 2>&1
        if [ $? -eq 0 ]; then
            echo "$(date): Updated job comment for $SLURM_JOB_ID" >> "$LOG_FILE"
        else
            echo "$(date): Failed to update job comment for $SLURM_JOB_ID" >> "$LOG_FILE"
        fi
    fi
fi

# Clean up old performance data (older than 30 days)
find "$OUTPUT_DIR" -name "job-*-performance.json" -mtime +30 -delete 2>/dev/null

exit 0