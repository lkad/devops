#!/bin/sh
# Device log generator - runs inside containerlab node
# Pushes logs directly to Loki via HTTP push API

LOKI_URL="${LOKI_URL:-http://192.168.121.111:3100/loki/api/v1/push}"
DEVICE_NAME="${DEVICE_NAME:-unknown}"
LOG_INTERVAL="${LOG_INTERVAL:-30}"

# Log messages
HOST_MSGS="CPU temperature nominal|Memory utilization normal|Disk I/O latency OK|Network interface eth0 up|Health check passed|Bootstrap complete|System time synchronized"

log_to_loki() {
    level="$1"
    message="$2"
    ts=$(date +%s)000000000
    # Correct Loki push API format with streams array
    stream="{\"streams\":[{\"stream\":{\"source\":\"physical_host\",\"level\":\"$level\",\"device\":\"$DEVICE_NAME\"},\"values\":[[\"$ts\",\"$message\"]]}]}"
    wget -q -O /dev/null --post-data="$stream" --header="Content-Type: application/json" "$LOKI_URL" 2>/dev/null || true
}

echo "Starting log generator for device: $DEVICE_NAME"
echo "Loki endpoint: $LOKI_URL"
echo "Log interval: ${LOG_INTERVAL}s"

count=0
while true; do
    count=$((count + 1))

    # Select random log level (90% info, 8% warning, 2% error)
    rand=$((RANDOM % 100))
    if [ $rand -lt 90 ]; then
        level="info"
    elif [ $rand -lt 98 ]; then
        level="warning"
    else
        level="error"
    fi

    # Pick random message
    msg_array=$(echo "$HOST_MSGS" | tr '|' '\n')
    total=$(echo "$msg_array" | wc -l)
    idx=$((RANDOM % total + 1))
    message=$(echo "$msg_array" | sed -n "${idx}p")

    # Add some variation
    case $((RANDOM % 5)) in
        0) message="$message [cpu=$((RANDOM % 80))%]" ;;
        1) message="$message [mem=$((30 + RANDOM % 50))%]" ;;
        2) message="$message [temp=$((35 + RANDOM % 25))C]" ;;
    esac

    log_to_loki "$level" "$message"
    echo "[$(date '+%H:%M:%S')] $DEVICE_NAME: $level - $message"

    sleep $LOG_INTERVAL
done