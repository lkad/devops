# Tasks: K8s Pod Log Streaming Implementation

## 1. Log Streamer Service

- [ ] 1.1 Create internal/k8s/log_streamer.go
- [ ] 1.2 Implement WatchPodLogs(clusterID, namespace, pod, container) (*LogStream, error)
- [ ] 1.3 Implement LogStream struct with channel-based log output
- [ ] 1.4 Implement ParseLogLevel(message string) LogLevel
- [ ] 1.5 Add context cancellation support
- [ ] 1.6 Implement resource cleanup on stream close

## 2. WebSocket Hub Integration

- [ ] 2.1 Add "container_log" channel type to internal/websocket/channel.go
- [ ] 2.2 Implement BroadcastToChannel(channel string, msg *Message) error
- [ ] 2.3 Update log_streamer to send logs to hub.BroadcastToChannel

## 3. Integration with Existing Logs Service

- [ ] 3.1 Inject logsService into K8s log_streamer
- [ ] 3.2 Call logsService.CreateLogEntry() for each log line (async/batch)
- [ ] 3.3 Pass ContainerMetadata in LogEntry.Metadata

## 4. API Endpoints

- [ ] 4.1 Add GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/stream
  - WebSocket endpoint for real-time log streaming
  - Subscribe to container_log channel
- [ ] 4.2 Add GET /api/k8s/clusters/:id/namespaces/:ns/pods/:pod/logs/link
  - Returns URL to log management platform with filters
  - Query params: cluster_id, namespace, pod, container

## 5. Frontend Integration

- [ ] 5.1 Create frontend/hooks/useK8sLogStream.ts
- [ ] 5.2 Implement WebSocket subscription to container_log channel
- [ ] 5.3 Implement log level color coding
- [ ] 5.4 Add auto-scroll and pause functionality
- [ ] 5.5 "View History" button navigates to /logs with pre-filled filters

## 6. Testing

- [ ] 6.1 Write unit tests for ParseLogLevel
- [ ] 6.2 Write unit tests for log_streamer.go with mock K8s client
- [ ] 6.3 Write integration tests verifying logs appear in log_entries with source=container

## 7. Configuration

- [ ] 7.1 Add k8s.log_streaming.enabled to config.yaml
- [ ] 7.2 Add k8s.log_streaming.batch_size for async writes
