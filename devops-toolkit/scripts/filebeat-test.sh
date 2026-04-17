#!/bin/bash
# Filebeat Log Collection Test Script
# Tests sending logs to Elasticsearch and Loki via Filebeat

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_DIR/config/filebeat"
DATA_DIR="$PROJECT_DIR/data"

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() { echo -e "${CYAN}[FILEBEAT]${NC} $1"; }
ok() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

usage() {
    cat <<EOF
Usage: $0 <command> [options]

Commands:
    setup          Create Filebeat configuration directories
    elasticsearch  Configure Filebeat for Elasticsearch output
    loki           Configure Filebeat for Loki output
    start         Start Filebeat with current configuration
    stop          Stop Filebeat
    status        Check Filebeat status
    test          Run log generation and collection test
    tail          Tail Filebeat logs
    clean         Clean up test data

Options:
    --backend <backend>  Target backend: elasticsearch|loki (default: elasticsearch)
    --logs <path>       Log files to collect (default: $PROJECT_DIR/config/logs.json)
    --interval <sec>    Check interval in seconds (default: 5)

Examples:
    $0 setup
    $0 elasticsearch
    $0 start
    $0 test --backend elasticsearch
    $0 test --backend loki

Environment Variables:
    ELASTICSEARCH_URL   Elasticsearch URL (default: http://localhost:9200)
    LOKI_URL           Loki URL (default: http://localhost:3100)
    FILEBEAT_INTERVAL  Check interval (default: 5)
EOF
}

# ===========================================
# Configuration
# ===========================================
ELASTICSEARCH_URL="${ELASTICSEARCH_URL:-http://localhost:9200}"
LOKI_URL="${LOKI_URL:-http://localhost:3100}"
FILEBEAT_INTERVAL="${FILEBEAT_INTERVAL:-5}"

# ===========================================
# Directories
# ===========================================
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR/filebeat"
mkdir -p "$DATA_DIR/logs"

# ===========================================
# Filebeat Config Templates
# ===========================================

create_filebeat_config() {
    local backend="$1"
    local config_file="$CONFIG_DIR/filebeat.yml"

    if [ "$backend" = "elasticsearch" ]; then
        cat > "$config_file" <<EOF
# Filebeat Configuration for Elasticsearch
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - $PROJECT_DIR/config/logs.json
    json.keys_under_root: true
    json.add_error_key: true
    json.message_key: message
    fields:
      log_type: devops-toolkit
      environment: \${NODE_ENV:-development}
    fields_under_root: true

  - type: log
    enabled: true
    paths:
      - $DATA_DIR/logs/*.log
    fields:
      log_type: device-collector
    fields_under_root: true

processors:
  - add_host_metadata:
      when.not.contains.tags: forwarded
  - add_cloud_metadata: ~
  - add_docker_metadata: ~
  - timestamp:
      field: timestamp
      layouts:
        - '2006-01-02T15:04:05.999Z'
        - '2006-01-02T15:04:05Z'
      test:
        - '2026-04-17T08:00:00.000Z'

output.elasticsearch:
  hosts: ["$ELASTICSEARCH_URL"]
  index: "devops-logs-%{+yyyy.MM.dd}"
  pipeline: "devops-logs-pipeline"

setup.template:
  name: "devops-logs"
  pattern: "devops-logs-*"
  settings:
    index.number_of_shards: 1
    index.number_of_replicas: 0

setup.ilm:
  enabled: false

logging.level: info
logging.to_files: true
logging.files:
  path: $DATA_DIR/filebeat
  name: filebeat
  keepfiles: 7
  permissions: 0644

http.enabled: true
http.host: 0.0.0.0
http.port: 5066
EOF
    else
        cat > "$config_file" <<EOF
# Filebeat Configuration for Loki
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - $PROJECT_DIR/config/logs.json
    json.keys_under_root: true
    json.add_error_key: true
    json.message_key: message
    fields:
      log_type: devops-toolkit
      environment: \${NODE_ENV:-development}
    fields_under_root: true

  - type: log
    enabled: true
    paths:
      - $DATA_DIR/logs/*.log
    fields:
      log_type: device-collector
    fields_under_root: true

processors:
  - add_host_metadata:
      when.not.contains.tags: forwarded
  - add_cloud_metadata: ~
  - add_docker_metadata: ~
  - timestamp:
      field: timestamp
      layouts:
        - '2006-01-02T15:04:05.999Z'
        - '2006-01-02T15:04:05Z'
      test:
        - '2026-04-17T08:00:00.000Z'
  - dropsample:
      rate: 10

output.logstash:
  hosts: ["localhost:5044"]

# Loki via promtail-compatible HTTP
filebeat.outputs:
  - type: loki
    enabled: true
    host: "$LOKI_URL"
    port: 3100
    labels:
      app: devops-toolkit
      environment: \${NODE_ENV:-development}
      job: filebeat
    timeout: 10

# Alternative: plain http output to Loki
output.http:
  enabled: true
  hosts: ["$LOKI_URL"]
  path: "/loki/api/v1/push"
  method: POST
  content_type: "application/json"

logging.level: info
logging.to_files: true
logging.files:
  path: $DATA_DIR/filebeat
  name: filebeat
  keepfiles: 7
  permissions: 0644

http.enabled: true
http.host: 0.0.0.0
http.port: 5066
EOF
    fi

    ok "Created Filebeat config: $config_file"
}

# ===========================================
# Create Sample Log Files
# ===========================================
create_sample_logs() {
    local output_dir="$DATA_DIR/logs"
    mkdir -p "$output_dir"

    # Create device collector logs
    cat > "$output_dir/device-collector.log" <<EOF
{"timestamp":"2026-04-17T08:00:00.000Z","level":"info","message":"Device collector started","source":"collector","device_id":"device-1","metadata":{"collector_version":"1.0.0"}}
{"timestamp":"2026-04-17T08:00:01.000Z","level":"debug","message":"Scanning for devices","source":"collector","device_id":"device-1","metadata":{"devices_found":5}}
{"timestamp":"2026-04-17T08:00:02.000Z","level":"info","message":"Device device-1 online","source":"heartbeat","device_id":"device-1","status":"online"}
{"timestamp":"2026-04-17T08:00:03.000Z","level":"warn","message":"Device device-2 high memory","source":"metrics","device_id":"device-2","metadata":{"memory_usage":87}}
{"timestamp":"2026-04-17T08:00:04.000Z","level":"error","message":"Device device-3 unreachable","source":"healthcheck","device_id":"device-3","error":"connection_timeout"}
{"timestamp":"2026-04-17T08:00:05.000Z","level":"info","message":"Device device-4 configuration updated","source":"config-manager","device_id":"device-4"}
{"timestamp":"2026-04-17T08:00:06.000Z","level":"debug","message":"Metric batch sent","source":"metrics-collector","device_id":"device-1","batch_size":100}
EOF

    # Create container logs
    cat > "$output_dir/container.log" <<EOF
{"timestamp":"2026-04-17T08:00:00.000Z","level":"info","message":"Container started","source":"docker","container_id":"abc123","image":"nginx:latest"}
{"timestamp":"2026-04-17T08:00:01.000Z","level":"info","message":"Health check passed","source":"docker","container_id":"abc123","status":"healthy"}
{"timestamp":"2026-04-17T08:00:02.000Z","level":"warn","message":"High CPU usage detected","source":"docker","container_id":"abc123","cpu_percent":85}
{"timestamp":"2026-04-17T08:00:03.000Z","level":"info","message":"Container stopped","source":"docker","container_id":"abc123","exit_code":0}
EOF

    # Create system logs
    cat > "$output_dir/system.log" <<EOF
{"timestamp":"2026-04-17T08:00:00.000Z","level":"info","message":"System startup complete","source":"init","hostname":"devops-server"}
{"timestamp":"2026-04-17T08:00:01.000Z","level":"info","message":"Network interface eth0 up","source":"network","interface":"eth0"}
{"timestamp":"2026-04-17T08:00:02.000Z","level":"error","message":"Disk space critical on /data","source":"disk","path":"/data","usage_percent":95}
{"timestamp":"2026-04-17T08:00:03.000Z","level":"warn","message":"Memory usage high","source":"memory","usage_percent":82}
{"timestamp":"2026-04-17T08:00:04.000Z","level":"info","message":"Backup completed successfully","source":"backup","duration_seconds":120}
EOF

    ok "Created sample logs in $output_dir"
}

# ===========================================
# Check Backend Health
# ===========================================
check_backend() {
    local backend="$1"

    if [ "$backend" = "elasticsearch" ]; then
        if curl -sf "$ELASTICSEARCH_URL/_cluster/health" > /dev/null 2>&1; then
            ok "Elasticsearch is healthy at $ELASTICSEARCH_URL"
            return 0
        else
            fail "Elasticsearch not reachable at $ELASTICSEARCH_URL"
            return 1
        fi
    else
        if curl -sf "$LOKI_URL/loki/api/v1/health" > /dev/null 2>&1; then
            ok "Loki is healthy at $LOKI_URL"
            return 0
        else
            fail "Loki not reachable at $LOKI_URL"
            return 1
        fi
    fi
}

# ===========================================
# Query Backend for Logs
# ===========================================
query_logs() {
    local backend="$1"
    local query="${2:-*}"

    if [ "$backend" = "elasticsearch" ]; then
        echo "=== Elasticsearch Logs ==="
        curl -sf "$ELASTICSEARCH_URL/devops-logs-*/_search" -H "Content-Type: application/json" \
            -d "{\"query\":{\"match_all\":{}},\"sort\":[{\"timestamp\":\"desc\"}],\"size\":10}" 2>/dev/null | \
            python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"Total: {d['hits']['total']['value']}\"); [print(f\"  - {h['_source'].get('message','')[:80]}\") for h in d['hits']['hits'][:5]]" 2>/dev/null || \
            echo "  Query failed or no logs found"
    else
        echo "=== Loki Logs ==="
        curl -sf "$LOKI_URL/loki/api/v1/query" --data-urlencode "query={app=\"devops-toolkit\"}" 2>/dev/null | \
            python3 -c "import sys,json; d=json.load(sys.stdin); print(f\"Total: {len(d.get('data',{}).get('result',[]))}\"); [print(f\"  - {v['line'][:80]}\") for v in d.get('data',{}).get('result',[])[:5]]" 2>/dev/null || \
            echo "  Query failed or no logs found"
    fi
}

# ===========================================
# Main Commands
# ===========================================
cmd_setup() {
    info "Creating Filebeat test environment..."
    create_sample_logs
    ok "Setup complete"
}

cmd_elasticsearch() {
    info "Configuring Filebeat for Elasticsearch..."
    create_filebeat_config "elasticsearch"
    ok "Elasticsearch configuration ready"
    echo "To start: $0 start --backend elasticsearch"
}

cmd_loki() {
    info "Configuring Filebeat for Loki..."
    create_filebeat_config "loki"
    ok "Loki configuration ready"
    echo "To start: $0 start --backend loki"
}

cmd_start() {
    local backend="${1:-elasticsearch}"
    local config_file="$CONFIG_DIR/filebeat.yml"

    if [ ! -f "$config_file" ]; then
        warn "No configuration found. Creating default..."
        create_filebeat_config "$backend"
    fi

    check_backend "$backend" || exit 1

    info "Starting Filebeat..."

    # Check if filebeat is installed
    if command -v filebeat >/dev/null 2>&1; then
        filebeat -c "$config_file" -d "*" &
        ok "Filebeat started (PID: $!)"
    else
        warn "Filebeat not installed. Using Docker..."
        docker run \
            --rm \
            --name filebeat-test \
            -v "$config_file:$CONFIG_DIR/filebeat.yml:ro" \
            -v "$DATA_DIR:/data:ro" \
            docker.elastic.co/beats/filebeat:8.12.0 \
            filebeat -c "$CONFIG_DIR/filebeat.yml" &
        ok "Filebeat Docker started"
    fi

    echo "Waiting for Filebeat to initialize..."
    sleep 3
}

cmd_stop() {
    info "Stopping Filebeat..."
    pkill -f "filebeat.*filebeat.yml" 2>/dev/null || true
    docker stop filebeat-test 2>/dev/null || true
    ok "Filebeat stopped"
}

cmd_status() {
    info "Filebeat Status"
    if pgrep -f "filebeat.*filebeat.yml" > /dev/null 2>&1; then
        ok "Filebeat is running"
    else
        warn "Filebeat is not running"
    fi
}

cmd_test() {
    local backend="${1:-elasticsearch}"

    info "=== Filebeat Collection Test for $backend ==="
    echo ""

    # Check backend
    check_backend "$backend" || exit 1

    # Stop existing Filebeat
    cmd_stop

    # Create config
    create_filebeat_config "$backend"

    # Start Filebeat
    cmd_start "$backend"

    # Wait for collection
    info "Waiting for log collection ($FILEBEAT_INTERVAL seconds)..."
    sleep "$FILEBEAT_INTERVAL"

    # Query logs
    echo ""
    query_logs "$backend"

    # Show Filebeat status
    echo ""
    cmd_status
}

cmd_tail() {
    local log_file="$DATA_DIR/filebeat/filebeat"

    if [ -f "$log_file" ]; then
        tail -f "$log_file"
    else
        warn "No Filebeat log found at $log_file"
    fi
}

cmd_clean() {
    info "Cleaning up test data..."
    rm -rf "$DATA_DIR/logs/*.log"
    rm -rf "$DATA_DIR/filebeat/*"
    ok "Cleaned"
}

# ===========================================
# Main
# ===========================================
main() {
    if [ $# -lt 1 ]; then
        usage
        exit 1
    fi

    local cmd="$1"
    shift

    case "$cmd" in
        setup) cmd_setup ;;
        elasticsearch) cmd_elasticsearch ;;
        loki) cmd_loki ;;
        start)
            local backend="${1:-elasticsearch}"
            cmd_start "$backend" ;;
        stop) cmd_stop ;;
        status) cmd_status ;;
        test)
            local backend="${1:-elasticsearch}"
            cmd_test "$backend" ;;
        tail) cmd_tail ;;
        clean) cmd_clean ;;
        help|--help|-h) usage ;;
        *)
            echo "Error: Unknown command '$cmd'"
            usage
            exit 1
            ;;
    esac
}

main "$@"