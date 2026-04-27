package k8s

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/devops-toolkit/internal/logs"
	"github.com/devops-toolkit/internal/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ContainerLogSubscription represents an active log streaming subscription
type ContainerLogSubscription struct {
	ClusterID   string
	ClusterName string
	Namespace   string
	Pod         string
	Container   string // empty string means all containers
	Since       time.Time
	Hub         *websocket.Hub
	LogsManager *logs.Manager
	stopChan    chan struct{}
	mu          sync.Mutex
	stopped     bool
}

// ContainerLogSubscriptions holds all active container log subscriptions
type ContainerLogSubscriptions struct {
	subscriptions map[string]*ContainerLogSubscription
	mu            sync.RWMutex
}

var subs = &ContainerLogSubscriptions{
	subscriptions: make(map[string]*ContainerLogSubscription),
}

// subscriptionKey generates a unique key for a subscription
func subscriptionKey(clusterID, namespace, pod, container string) string {
	return fmt.Sprintf("%s/%s/%s/%s", clusterID, namespace, pod, container)
}

// SubscribeToContainerLogs starts streaming logs for a container
func SubscribeToContainerLogs(hub *websocket.Hub, logMgr *logs.Manager, clusterID, clusterName, namespace, pod, container string, since time.Time) (*ContainerLogSubscription, error) {
	sub := &ContainerLogSubscription{
		ClusterID:   clusterID,
		ClusterName: clusterName,
		Namespace:   namespace,
		Pod:         pod,
		Container:   container,
		Since:       since,
		Hub:         hub,
		LogsManager: logMgr,
		stopChan:    make(chan struct{}),
	}

	key := subscriptionKey(clusterID, namespace, pod, container)
	subs.mu.Lock()
	// Stop existing subscription for this pod/container
	if existing, ok := subs.subscriptions[key]; ok {
		existing.Stop()
	}
	subs.subscriptions[key] = sub
	subs.mu.Unlock()

	// Start streaming in background
	go sub.stream()

	return sub, nil
}

// Stop stops the log streaming
func (s *ContainerLogSubscription) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.stopChan)
	}
}

// stream streams logs from the pod
func (s *ContainerLogSubscription) stream() {
	defer func() {
		key := subscriptionKey(s.ClusterID, s.Namespace, s.Pod, s.Container)
		subs.mu.Lock()
		delete(subs.subscriptions, key)
		subs.mu.Unlock()
	}()

	kubeconfig := s.getKubeconfig()

	for {
		select {
		case <-s.stopChan:
			return
		default:
			if err := s.watchLogs(kubeconfig); err != nil {
				log.Printf("Error watching logs for %s/%s/%s: %v", s.Namespace, s.Pod, s.Container, err)
				time.Sleep(5 * time.Second)
			}
			// If watchLogs returns, it might be a temporary error, retry
			select {
			case <-s.stopChan:
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// getKubeconfig returns the kubeconfig path for the cluster
func (s *ContainerLogSubscription) getKubeconfig() string {
	return fmt.Sprintf("%s/.kube/config-%s", getEnv("HOME", "/root"), s.ClusterID)
}

// getEnv returns environment variable or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// watchLogs watches logs from the pod using kubectl
func (s *ContainerLogSubscription) watchLogs(kubeconfig string) error {
	args := []string{
		"--kubeconfig", kubeconfig,
		"logs",
		"-n", s.Namespace,
		s.Pod,
		"--follow",
		"--timestamps",
	}

	if s.Container != "" {
		args = append(args, "-c", s.Container)
	}

	if !s.Since.IsZero() {
		args = append(args, fmt.Sprintf("--since=%s", s.Since.Format(time.RFC3339)))
	}

	cmd := exec.CommandContext(context.Background(), "kubectl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start kubectl: %w", err)
	}

	// Read logs line by line
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopChan:
			cmd.Process.Kill()
			return nil
		default:
			n, err := stdout.Read(buf)
			if err != nil {
				cmd.Process.Kill()
				return err
			}
			if n > 0 {
				lines := bytes.Split(buf[:n], []byte("\n"))
				for _, line := range lines {
					if len(line) > 0 {
						s.processLogLine(string(line))
					}
				}
			}
		}
	}
}

// processLogLine processes a single log line
func (s *ContainerLogSubscription) processLogLine(line string) {
	// Parse timestamp from kubectl --timestamps output (format: 2026-04-27T10:00:00.000000000Z ...)
	timestamp := time.Now()
	message := line

	if ts, msg, ok := parseTimestamp(line); ok {
		timestamp = ts
		message = msg
	}

	level := InferLogLevel(message)

	entry := &websocket.ContainerLogEntry{
		ClusterID:   s.ClusterID,
		ClusterName: s.ClusterName,
		Namespace:   s.Namespace,
		Pod:         s.Pod,
		Container:   s.Container,
		Message:     message,
		Level:       level,
		Timestamp:   timestamp.Format(time.RFC3339),
	}

	// Broadcast to WebSocket subscribers
	if s.Hub != nil {
		if err := s.Hub.BroadcastContainerLog(entry); err != nil {
			log.Printf("Failed to broadcast container log: %v", err)
		}
	}

	// Persist via logs service
	if s.LogsManager != nil {
		meta := map[string]interface{}{
			"cluster_id": s.ClusterID,
			"cluster":    s.ClusterName,
			"namespace":  s.Namespace,
			"pod":        s.Pod,
			"container":  s.Container,
		}
		if _, err := s.LogsManager.AddLog(level, message, "container", meta); err != nil {
			log.Printf("Failed to persist container log: %v", err)
		}
	}
}

// parseTimestamp tries to extract timestamp from log line
func parseTimestamp(line string) (time.Time, string, bool) {
	// kubectl --timestamps outputs: 2026-04-27T10:00:00.000000000Z message
	// Try RFC3339Nano format first

	// First, try to parse the whole line as RFC3339Nano (timestamp-only case)
	ts, err := time.Parse(time.RFC3339Nano, line)
	if err == nil {
		return ts, "", true
	}

	// Otherwise, split on space to get timestamp and message
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return time.Time{}, line, false
	}

	ts, err = time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, line, false
	}

	return ts, parts[1], true
}

// InferLogLevel infers the log level from message content
func InferLogLevel(message string) string {
	upper := strings.ToUpper(message)

	// Error level patterns
	errorPatterns := []string{
		"ERROR",
		"FAILED",
		"FATAL",
		"PANIC",
	}
	for _, pattern := range errorPatterns {
		if strings.Contains(upper, pattern) {
			return "error"
		}
	}

	// Warn level patterns
	warnPatterns := []string{
		"WARN",
		"WARNING",
	}
	for _, pattern := range warnPatterns {
		if strings.Contains(upper, pattern) {
			return "warn"
		}
	}

	// Debug level patterns
	debugPatterns := []string{
		"DEBUG",
		"TRACE",
	}
	for _, pattern := range debugPatterns {
		if strings.Contains(upper, pattern) {
			return "debug"
		}
	}

	return "info"
}

// GetPodContainers returns all container names for a pod
func (m *ClusterManager) GetPodContainers(clusterName, namespace, podName string) ([]string, error) {
	ctx := context.Background()
	kubeconfig := m.getKubeconfig(clusterName)

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var containers []string
	for _, c := range pod.Spec.Containers {
		containers = append(containers, c.Name)
	}

	return containers, nil
}

// SubscribeToAllContainers subscribes to logs from all containers in a pod
func SubscribeToAllContainers(hub *websocket.Hub, logMgr *logs.Manager, clusterID, clusterName, namespace, pod string, since time.Time) ([]*ContainerLogSubscription, error) {
	m := NewClusterManager()
	containers, err := m.GetPodContainers(clusterID, namespace, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod containers: %w", err)
	}

	var subscriptions []*ContainerLogSubscription
	for _, container := range containers {
		sub, err := SubscribeToContainerLogs(hub, logMgr, clusterID, clusterName, namespace, pod, container, since)
		if err != nil {
			log.Printf("Failed to subscribe to container %s: %v", container, err)
			continue
		}
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions, nil
}

// UnsubscribeFromContainerLogs stops streaming logs for a container
func UnsubscribeFromContainerLogs(clusterID, namespace, pod, container string) {
	key := subscriptionKey(clusterID, namespace, pod, container)
	subs.mu.Lock()
	if sub, ok := subs.subscriptions[key]; ok {
		sub.Stop()
		delete(subs.subscriptions, key)
	}
	subs.mu.Unlock()
}

// GetActiveSubscriptions returns all active container log subscriptions
func GetActiveSubscriptions() []*ContainerLogSubscription {
	subs.mu.RLock()
	defer subs.mu.RUnlock()

	var result []*ContainerLogSubscription
	for _, sub := range subs.subscriptions {
		result = append(result, sub)
	}
	return result
}

// HandleContainerLogSubscription creates a new container log subscription
func HandleContainerLogSubscription(hub *websocket.Hub, logMgr *logs.Manager, req *websocket.ContainerLogSubscribeRequest) (*ContainerLogSubscription, error) {
	var since time.Time
	if req.Since != "" {
		var err error
		since, err = time.Parse(time.RFC3339, req.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid since timestamp: %w", err)
		}
	}

	if req.Container == "" {
		// Subscribe to all containers
		subs, err := SubscribeToAllContainers(hub, logMgr, req.ClusterID, req.ClusterName, req.Namespace, req.Pod, since)
		if err != nil {
			return nil, err
		}
		if len(subs) > 0 {
			return subs[0], nil
		}
		return nil, fmt.Errorf("no containers found in pod")
	}

	return SubscribeToContainerLogs(hub, logMgr, req.ClusterID, req.ClusterName, req.Namespace, req.Pod, req.Container, since)
}