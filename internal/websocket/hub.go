package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	subscriptions map[*Client]map[string]bool
	// ContainerLogSubscribeHandler is called when a client subscribes to container_log channel
	// It receives the subscription parameters and should start streaming logs
	ContainerLogSubscribeHandler func(*Client, *ContainerLogSubscribeRequest)
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	id     int
}

type Message struct {
	Type    string      `json:"type"`
	Channel string      `json:"channel,omitempty"`
	Payload interface{} `json:"payload"`
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		subscriptions: make(map[*Client]map[string]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			client.id = len(h.clients) + 1
			h.clients[client] = true
			h.subscriptions[client] = make(map[string]bool)
			h.mu.Unlock()
			log.Printf("WebSocket client %d connected", client.id)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.subscriptions, client)
				close(client.send)
				log.Printf("WebSocket client %d disconnected", client.id)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
					delete(h.subscriptions, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastLog(entry interface{}) {
	msg := Message{
		Type:    "log",
		Channel: "log",
		Payload: entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastMetric(entry interface{}) {
	msg := Message{
		Type:    "metric",
		Channel: "metric",
		Payload: entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastDeviceEvent(entry interface{}) {
	msg := Message{
		Type:    "device_event",
		Channel: "device_event",
		Payload: entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastAlert(entry interface{}) {
	msg := Message{
		Type:    "alert",
		Channel: "alert",
		Payload: entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastPipelineUpdate(entry interface{}) {
	msg := Message{
		Type:    "pipeline_update",
		Channel: "pipeline_update",
		Payload: entry,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.broadcast <- data
}

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			c.hub.mu.Lock()
			if c.hub.subscriptions[c] == nil {
				c.hub.subscriptions[c] = make(map[string]bool)
			}
			c.hub.subscriptions[c][msg.Channel] = true
			c.hub.mu.Unlock()

			// Handle container_log subscription with payload
			if msg.Channel == "container_log" && c.hub.ContainerLogSubscribeHandler != nil {
				if payloadBytes, ok := msg.Payload.(json.RawMessage); ok {
					var req ContainerLogSubscribeRequest
					if err := json.Unmarshal(payloadBytes, &req); err == nil {
						c.hub.ContainerLogSubscribeHandler(c, &req)
					}
				}
			}

		case "unsubscribe":
			c.hub.mu.Lock()
			if c.hub.subscriptions[c] != nil {
				delete(c.hub.subscriptions[c], msg.Channel)
			}
			c.hub.mu.Unlock()

		case "ping":
			response := Message{Type: "pong"}
			data, _ := json.Marshal(response)
			c.send <- data
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

// BroadcastToChannel sends a message only to clients subscribed to the specified channel
func (h *Hub) BroadcastToChannel(channel string, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if h.subscriptions[client][channel] {
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(h.clients, client)
				delete(h.subscriptions, client)
			}
		}
	}
	return nil
}

// ContainerLogEntry represents a K8s container log message
type ContainerLogEntry struct {
	ClusterID   string `json:"clusterId"`
	ClusterName string `json:"clusterName"`
	Namespace   string `json:"namespace"`
	Pod         string `json:"pod"`
	Container   string `json:"container"`
	Message     string `json:"message"`
	Level       string `json:"level"`
	Timestamp   string `json:"timestamp"`
}

// ContainerLogSubscribeRequest represents subscription params for container_log
type ContainerLogSubscribeRequest struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Namespace   string `json:"namespace"`
	Pod         string `json:"pod"`
	Container   string `json:"container,omitempty"`
	Since       string `json:"since,omitempty"`
}

// BroadcastContainerLog sends a container log to all subscribers of the container_log channel
func (h *Hub) BroadcastContainerLog(entry *ContainerLogEntry) error {
	msg := Message{
		Type:    "container_log",
		Channel: "container_log",
		Payload: entry,
	}
	return h.BroadcastToChannel("container_log", &msg)
}

// ContainerLogSubscribeHandler is called when a client subscribes to container_log channel
// It receives the subscription parameters and should start streaming logs
type ContainerLogSubscribeHandler func(*Client, *ContainerLogSubscribeRequest)

// SetContainerLogSubscribeHandler sets the handler for container_log subscriptions
func (h *Hub) SetContainerLogSubscribeHandler(handler ContainerLogSubscribeHandler) {
	h.ContainerLogSubscribeHandler = handler
}
