#!/bin/bash
# Register containerlab physical hosts to devops-toolkit
# Discovers running clab containers and registers them as physical_host devices

DEVOPS_URL="${DEVOPS_URL:-http://localhost:3000}"
AUTH="${AUTH:-dev:dev}"
LOG_INTERVAL=30

echo "=== ContainerLab Device Registration ==="
echo "DevOps URL: $DEVOPS_URL"
echo ""

# Discover physical host containers
# Note: Loki sends logs with device label format "physical-host-001" (with leading zeros)
# so we use the same format here for consistency
declare -A CONTAINERS=(
    ["clab-devops-physical-hosts-host01"]="physical-host-001"
    ["clab-devops-physical-hosts-host02"]="physical-host-002"
    ["clab-devops-physical-hosts-host03"]="physical-host-003"
)

# Check if log generator is running in container, if not deploy it
ensure_log_generator() {
    local container=$1
    local device=$2

    # Check if log-gen.sh is running
    if ! docker exec $container ps aux | grep -q "log-gen.sh" 2>/dev/null; then
        echo "  Deploying log generator to $container..."

        # Create log generator script
        cat << 'LOGSCRIPT' | docker exec -i $container sh -c 'cat > /tmp/log-gen.sh && chmod +x /tmp/log-gen.sh'
#!/bin/sh
LOKI_URL="${LOKI_URL:-http://192.168.121.111:3100/loki/api/v1/push}"
DEVICE_NAME="${DEVICE_NAME:-unknown}"
LOG_INTERVAL="${LOG_INTERVAL:-30}"

HOST_MSGS="CPU temperature nominal|Memory utilization normal|Disk I/O latency OK|Network interface eth0 up|Health check passed|Bootstrap complete|System time synchronized"

log_to_loki() {
    level="$1"
    message="$2"
    ts=$(date +%s)000000000
    stream="{\"streams\":[{\"stream\":{\"source\":\"physical_host\",\"level\":\"$level\",\"device\":\"$DEVICE_NAME\"},\"values\":[[\"$ts\",\"$message\"]]}]}"
    wget -q -O /dev/null --post-data="$stream" --header="Content-Type: application/json" "$LOKI_URL" 2>/dev/null || true
}

while true; do
    rand=$((RANDOM % 100))
    [ $rand -lt 90 ] && level="info" || { [ $rand -lt 98 ] && level="warning" || level="error"; }

    msg_array=$(echo "$HOST_MSGS" | tr '|' '\n')
    total=$(echo "$msg_array" | wc -l)
    idx=$((RANDOM % total + 1))
    message=$(echo "$msg_array" | sed -n "${idx}p")

    case $((RANDOM % 5)) in
        0) message="$message [cpu=$((RANDOM % 80))%]" ;;
        1) message="$message [mem=$((30 + RANDOM % 50))%]" ;;
        2) message="$message [temp=$((35 + RANDOM % 25))C]" ;;
    esac

    log_to_loki "$level" "$message"
    echo "[$(date '+%H:%M:%S')] $DEVICE_NAME: $level - $message"
    sleep $LOG_INTERVAL
done
LOGSCRIPT

        # Start it
        docker exec -d $container sh -c "DEVICE_NAME=$device LOG_INTERVAL=$LOG_INTERVAL nohup /tmp/log-gen.sh > /tmp/log-gen.log 2>&1 &"
        sleep 1
    else
        echo "  Log generator already running in $container"
    fi
}

# Register each containerlab physical host
for container in "${!CONTAINERS[@]}"; do
    device_name="${CONTAINERS[$container]}"
    echo "=== Processing $container ($device_name) ==="

    # Check if container is running
    if ! docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
        echo "  Container not running, skipping..."
        continue
    fi

    # Ensure log generator is running
    ensure_log_generator "$container" "$device_name"

    # Check if already registered by checking for existing device with same name
    existing=$(curl -s -u "$AUTH" "$DEVOPS_URL/api/devices?type=physical_host" 2>/dev/null | python3 -c "
import sys,json
try:
    data=json.load(sys.stdin)
    for d in data.get('data',[]):
        if d.get('name')=='$device_name':
            print(d['id'])
except: pass
" 2>/dev/null)

    if [ -n "$existing" ]; then
        echo "  Device $device_name already registered (ID: $existing)"
    else
        # Get container IP
        ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $container 2>/dev/null)

        # Register new device
        echo "  Registering $device_name (IP: $ip)..."
        result=$(curl -s -X POST -u "$AUTH" "$DEVOPS_URL/api/devices" \
            -H "Content-Type: application/json" \
            -d "{
                \"name\": \"$device_name\",
                \"type\": \"physical_host\",
                \"environment\": \"test\",
                \"labels\": {
                    \"source\": \"containerlab\",
                    \"container\": \"$container\",
                    \"ip\": \"$ip\"
                }
            }" 2>/dev/null)

        if echo "$result" | python3 -c "import sys,json; json.load(sys.stdin); print('OK')" 2>/dev/null; then
            echo "  Registered successfully"
        else
            echo "  Registration result: $result"
        fi
    fi

    # Show log generator status
    log_status=$(docker exec $container sh -c 'tail -2 /tmp/log-gen.log 2>/dev/null' 2>/dev/null || echo "no logs")
    echo "  Log status: $log_status"
    echo ""
done

echo "=== Done ==="
echo ""
echo "Verifying device registration..."
curl -s -u "$AUTH" "$DEVOPS_URL/api/devices?type=physical_host" | python3 -c "
import sys,json
data=json.load(sys.stdin)
print(f\"Total physical hosts: {len(data.get('data',[]))}\")
for d in data.get('data',[]):
    print(f\"  - {d['name']} (status: {d['status']})\")
" 2>/dev/null