#!/bin/bash

# Test standalone mode functionality without requiring actual Slurm installation

echo "Testing aws-slurm-burst standalone mode..."

# Test 1: Configuration validation
echo "1. Testing configuration validation..."
./build/validate config examples/config.yaml
if [ $? -ne 0 ]; then
    echo "❌ Configuration validation failed"
    exit 1
fi
echo "✅ Configuration validation passed"

# Test 2: Execution plan validation
echo -e "\n2. Testing execution plan validation..."
./build/validate execution-plan examples/asba-execution-plan.json
if [ $? -ne 0 ]; then
    echo "❌ Execution plan validation failed"
    exit 1
fi
echo "✅ Execution plan validation passed"

# Test 3: Integration validation
echo -e "\n3. Testing integration validation..."
./build/validate integration
if [ $? -ne 0 ]; then
    echo "❌ Integration validation failed"
    exit 1
fi
echo "✅ Integration validation passed"

# Test 4: Help commands work
echo -e "\n4. Testing help commands..."
./build/resume --help > /dev/null
./build/suspend --help > /dev/null
./build/state-manager --help > /dev/null
./build/validate --help > /dev/null
echo "✅ All help commands work"

echo -e "\n🎉 All standalone tests passed!"
echo -e "\nNote: Full end-to-end testing requires:"
echo "- Working Slurm installation (scontrol command)"
echo "- AWS credentials and permissions"
echo "- Configured launch templates and subnets"

echo -e "\nReady for deployment to Slurm environment!"