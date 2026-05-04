#!/bin/bash
# Device log simulation script
# Generates periodic logs for physical hosts and network devices, sends to Loki via devops-toolkit

DEVOPS_URL="http://localhost:3000"
LOG_INTERVAL=10  # seconds between log entries

# Device types
declare -a DEVICES=(
    "physical-host-01:physical_host:host01 datacenter rack A"
    "physical-host-02:physical_host:host02 datacenter rack B"
    "physical-host-03:physical_host:host03 datacenter rack C"
    "switch-dc1-sw1:network_device:dc1-sw1 datacenter-east row1"
    "switch-dc1-sw2:network_device:dc1-sw2 datacenter-east row2"
    "switch-dc2-sw1:network_device:dc2-sw1 datacenter-west row1"
    "switch-dc2-sw2:network_device:dc2-sw2 datacenter-west row2"
)

# Log levels
declare -a LEVELS=("info" "info" "info" "warning" "debug")

# Log messages by device type
declare -A MESSAGES_HOST=(
    ["info"]="CPU usage normal|Memory usage normal|Disk I/O normal|Network traffic normal|Health check passed"
    ["warning"]="High CPU temperature detected|Disk space low|CPU usage spike|Memory pressure detected"
    ["debug"]="Metric collection completed|Sensor data updated|Configuration sync completed"
)

declare -A MESSAGES_SWITCH=(
    ["info"]="Port eth1/0/1 up|Port eth1/0/2 up|VLAN configuration applied|Spanning tree stable"
    ["warning"]="Port eth1/0/3 error rate high|Interface utilization high|Broadcast storm detected"
    ["debug"]="MAC address table updated|Forwarding table updated|CAM table lookup performed"
)

get_message() {
    local type=$1
    local level=$2
    local key="${type}[${level}]"
    local msgs="${!key}"
    local msg_array=(${msgs//|/ })
    echo "${msg_array[$((RANDOM % ${#msg_array[@]}))]}"
}

echo "Starting device log simulation..."
echo "Sending logs to $DEVOPS_URL every ${LOG_INTERVAL}s"
echo "Press Ctrl+C to stop"
echo ""

# Register devices first
for device_info in "${DEVICES[@]}"; do
    IFS=':' read -r name type location <<< "$device_info"
    echo "Registering device: $name ($type) at $location"

    # Use dev username for auth bypass
    curl -s -X POST "$DEVOPS_URL/api/devices" \
        -H "Content-Type: application/json" \
        -u dev:dev \
        -d "{\"name\":\"$name\",\"type\":\"$type\",\"environment\":\"test\",\"labels\":{\"location\":\"$location\"}}" \
        > /dev/null 2>&1 || true
done

echo ""
echo "Devices registered. Starting log generation..."
echo ""

# Generate logs in a loop
count=0
while true; do
    count=$((count + 1))

    # Pick a random device
    idx=$((RANDOM % ${#DEVICES[@]}))
    device_info="${DEVICES[$idx]}"
    IFS=':' read -r name type location <<< "$device_info"

    # Pick log level (80% info, 15% warning, 5% debug)
    rand=$((RANDOM % 100))
    if [ $rand -lt 80 ]; then
        level="info"
    elif [ $rand -lt 95 ]; then
        level="warning"
    else
        level="debug"
    fi

    # Get message based on device type
    if [[ "$type" == "physical_host" ]]; then
        message=$(get_message "MESSAGES_HOST" "$level")
    else
        message=$(get_message "MESSAGES_SWITCH" "$level")
    fi

    # Add some variation with random values
    case $((RANDOM % 5)) in
        0) message="$message (value=$((RANDOM % 100)))" ;;
        1) message="$message [cpu=$((RANDOM % 100))%]" ;;
        2) message="$message [temp=$((35 + RANDOM % 20))C]" ;;
    esac

    # Send log via API (this goes to Loki when backend=loki)
    response=$(curl -s -X POST "$DEVOPS_URL/api/logs" \
        -H "Content-Type: application/json" \
        -u dev:dev \
        -d "{\"level\":\"$level\",\"message\":\"$message\",\"source\":\"$type\",\"metadata\":{\"device\":\"$name\",\"location\":\"$location\",\"seq\":\"$count\"}}")

    echo "[$(date '+%H:%M:%S')] $name: $level - $message"

    sleep $LOG_INTERVAL
done