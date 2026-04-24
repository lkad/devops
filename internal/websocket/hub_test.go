package websocket

import (
	"encoding/json"
	"testing"
	"time"
)

func TestHub_NewHub(t *testing.T) {
	h := NewHub()
	if h.clients == nil {
		t.Fatal("expected clients to be initialized")
	}
	if h.broadcast == nil {
		t.Fatal("expected broadcast channel to be initialized")
	}
	if h.register == nil {
		t.Fatal("expected register channel to be initialized")
	}
	if h.unregister == nil {
		t.Fatal("expected unregister channel to be initialized")
	}
	if h.subscriptions == nil {
		t.Fatal("expected subscriptions to be initialized")
	}
}

func TestMessage_Struct(t *testing.T) {
	msg := Message{
		Type:    "log",
		Channel: "log",
		Payload: map[string]string{"message": "hello"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.Type != "log" {
		t.Errorf("expected type 'log', got '%s'", decoded.Type)
	}
	if decoded.Channel != "log" {
		t.Errorf("expected channel 'log', got '%s'", decoded.Channel)
	}
}

func TestHub_BroadcastLog(t *testing.T) {
	h := NewHub()

	// Start the hub goroutine
	go h.Run()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not panic
	h.BroadcastLog(map[string]string{"msg": "test"})
}

func TestHub_BroadcastMetric(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	h.BroadcastMetric(map[string]string{"metric": "cpu"})
}

func TestHub_BroadcastAlert(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	h.BroadcastAlert(map[string]string{"alert": "high-cpu"})
}

func TestHub_BroadcastPipelineUpdate(t *testing.T) {
	h := NewHub()
	go h.Run()
	time.Sleep(10 * time.Millisecond)

	h.BroadcastPipelineUpdate(map[string]string{"pipeline": "deploy"})
}

func TestClient_Struct(t *testing.T) {
	// Client is created by Hub, just verify structure
	c := &Client{
		id: 1,
	}
	if c.id != 1 {
		t.Errorf("expected id 1, got %d", c.id)
	}
}
