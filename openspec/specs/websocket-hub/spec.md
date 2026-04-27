# websocket-hub

## ADDED Requirements

### Requirement: WebSocket Upgrade
Server SHALL accept WebSocket connections at `/ws` endpoint.

```go
GET /ws
Upgrade: websocket
```

#### Scenario: Successful upgrade
- **WHEN** client sends GET /ws with Upgrade: websocket header
- **THEN** connection is upgraded to WebSocket
- **AND** client receives connection confirmation message

#### Scenario: Invalid upgrade request
- **WHEN** client sends regular HTTP to /ws
- **THEN** 400 Bad Request is returned

### Requirement: Client Registration
Clients SHALL be assigned unique ID and registered with hub on connect.

```go
type Client struct {
    ID      string
    Conn    *websocket.Conn
    Send    chan []byte
    Hub     *Hub
   Mu      sync.RWMutex
}
```

#### Scenario: Client registration
- **WHEN** WebSocket connection is established
- **THEN** client is assigned unique ID
- **AND** client is registered with hub
- **AND** client starts read/write goroutines

### Requirement: Channel Subscription
Clients SHALL be able to subscribe to channels.

```json
{
    "action": "subscribe",
    "channel": "log"
}
```

#### Scenario: Subscribe to channel
- **WHEN** client sends subscribe message
- **THEN** client is added to channel subscription list
- **AND** confirmation message is sent to client

#### Scenario: Unsubscribe from channel
- **WHEN** client sends unsubscribe message
- **THEN** client is removed from channel subscription list

### Requirement: Supported Channels
System SHALL support the following channels: log, metric, device_event, pipeline_update, alert, container_log.

#### Scenario: Valid channel
- **WHEN** client subscribes to "log"
- **THEN** subscription succeeds

#### Scenario: Valid container_log channel
- **WHEN** client subscribes to "container_log"
- **THEN** subscription succeeds

#### Scenario: Invalid channel
- **WHEN** client subscribes to "invalid_channel"
- **THEN** error message is sent to client

### Requirement: Message Format
All broadcast messages SHALL follow standard format.

```json
{
    "channel": "log",
    "type": "log_entry",
    "data": {
        "id": "uuid",
        "level": "error",
        "message": "Something went wrong",
        "timestamp": "2026-04-27T10:00:00Z"
    },
    "timestamp": "2026-04-27T10:00:00Z"
}
```

### Requirement: Broadcast to Channel
Messages SHALL be broadcast to all clients subscribed to the channel.

#### Scenario: Broadcast log event
- **WHEN** new log entry is created
- **THEN** message is broadcast to all "log" channel subscribers

#### Scenario: Multiple channel subscribers
- **WHEN** message is broadcast to channel
- **THEN** ALL clients in that channel receive the message

### Requirement: Concurrent Safety
Hub operations SHALL be thread-safe using mutex.

#### Scenario: Concurrent subscribe
- **WHEN** multiple clients subscribe simultaneously
- **THEN** no race condition occurs
- **AND** all subscriptions are recorded correctly

### Requirement: Client Disconnect
Clients SHALL be properly unregistered on disconnect.

#### Scenario: Clean disconnect
- **WHEN** client closes WebSocket connection
- **THEN** client is removed from hub
- **AND** client is removed from all channel subscriptions
- **AND** read/write goroutines are terminated

### Requirement: Ping/Pong Heartbeat
Server SHALL send periodic ping messages to keep connection alive.

#### Scenario: Ping interval
- **WHEN** no message is sent for ping_interval seconds
- **THEN** server sends ping message
- **AND** expects pong response

### Requirement: Message Queue
Server SHALL use buffered channel for outgoing messages.

```go
const OutBufferSize = 256

type Client struct {
    Send chan []byte  // buffered channel
}
```

#### Scenario: Slow client
- **WHEN** client is slow to read
- **THEN** messages queue in Send channel
- **AND** client is disconnected if queue overflows

### Requirement: Hub Broadcast Channel
Hub SHALL use dedicated broadcast channel for message distribution.

```go
type Hub struct {
    broadcast  chan *Message
    register   chan *Client
    unregister chan *Client
}
```

#### Scenario: Message flow
- **WHEN** module calls hub.Broadcast(channel, message)
- **THEN** message is sent to broadcast channel
- **AND** hub routes to appropriate channel subscribers
