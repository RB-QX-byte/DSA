#!/bin/bash
set -e

echo "Starting Judge Worker with Dual-Layer Sandbox..."

# Initialize isolate environment
echo "Initializing isolate sandbox environment..."

# Check if running as root (for initialization)
if [ "$EUID" -eq 0 ]; then
    echo "Running as root - setting up sandbox environment..."
    
    # Set up cgroups if not already done
    if [ ! -d "/sys/fs/cgroup/memory/judge" ]; then
        mkdir -p /sys/fs/cgroup/memory/judge
        mkdir -p /sys/fs/cgroup/cpuacct/judge  
        mkdir -p /sys/fs/cgroup/pids/judge
    fi
    
    # Set cgroup limits
    echo "256M" > /sys/fs/cgroup/memory/judge/memory.limit_in_bytes 2>/dev/null || true
    echo "64" > /sys/fs/cgroup/pids/judge/pids.max 2>/dev/null || true
    
    # Initialize isolate boxes
    for i in {0..15}; do
        /usr/bin/isolate --box-id=$i --init 2>/dev/null || true
    done
    
    # Switch to judge user
    exec su-exec judge "$0" "$@"
fi

# Running as judge user
echo "Setting up judge worker environment..."

# Set resource limits for the judge process
ulimit -t 30      # CPU time limit: 30 seconds
ulimit -v 1048576 # Virtual memory limit: 1GB
ulimit -u 64      # Process limit: 64 processes
ulimit -f 32768   # File size limit: 32MB

# Verify isolate is working
echo "Testing isolate sandbox..."
if ! /usr/bin/isolate --version > /dev/null 2>&1; then
    echo "ERROR: isolate is not available or not working"
    exit 1
fi

# Test sandbox initialization
if ! /usr/bin/isolate --box-id=0 --init > /dev/null 2>&1; then
    echo "ERROR: Cannot initialize isolate sandbox"
    exit 1
fi

echo "Cleaning up test sandbox..."
/usr/bin/isolate --box-id=0 --cleanup > /dev/null 2>&1 || true

# Set up work directories
mkdir -p ${JUDGE_WORK_DIR}/temp
mkdir -p ${JUDGE_WORK_DIR}/logs

# Apply seccomp profile if available
if [ -f "${SECCOMP_PROFILE}" ]; then
    echo "Seccomp profile found at ${SECCOMP_PROFILE}"
    export SECCOMP_PROFILE_PATH="${SECCOMP_PROFILE}"
fi

# Verify required environment variables
if [ -z "$DATABASE_URL" ]; then
    echo "WARNING: DATABASE_URL not set"
fi

if [ -z "$REDIS_URL" ]; then
    echo "WARNING: REDIS_URL not set" 
fi

# Start health check server in background
echo "Starting health check server..."
nohup /opt/judge/health-server > ${JUDGE_WORK_DIR}/logs/health.log 2>&1 &

# Wait a moment for health server to start
sleep 2

echo "Judge Worker initialization complete. Starting main process..."
echo "Arguments: $@"

# Execute the main command
exec "$@"