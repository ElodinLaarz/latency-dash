# Technical Specification - Latency Dashboard

## Purpose
This document provides technical details for implementing the core algorithms and data structures of the latency monitoring system.

---

## 1. Data Structures

### 1.1 Backend (Go)

#### Hub
```go
type Hub struct {
    // Active client connections
    clients map[*Client]bool
    
    // Inbound messages to broadcast
    broadcast chan []byte
    
    // Register requests from clients
    register chan *Client
    
    // Unregister requests from clients
    unregister chan *Client
    
    // Mutex for thread-safe operations
    mu sync.RWMutex
}
```

**Thread Safety**: All operations on `clients` map must be protected by `mu` mutex.

**Channels**:
- `broadcast`: Buffered channel (e.g., 256) for message queuing
- `register/unregister`: Unbuffered for immediate processing

#### Client
```go
type Client struct {
    // Reference to hub
    hub *Hub
    
    // WebSocket connection
    conn *websocket.Conn
    
    // Buffered channel for outbound messages
    send chan []byte
}
```

**Send Buffer**: Recommended size 256 messages. If buffer fills, disconnect client to prevent memory issues.

#### Event Message (Protocol Buffers - Multi-Target)
```protobuf
syntax = "proto3";

package latency;

message Event {
    string target_id = 1;         // Target stream identifier
    string key = 2;               // Service/component key
    int64 server_timestamp = 3;   // Unix nanoseconds when server sent
    bytes payload = 4;            // Random data for throughput testing
    uint32 payload_size = 5;      // Size in bytes
}

message InitMessage {
    string message = 1;
    int64 server_time = 2;
    repeated string available_targets = 3;  // List of targets client can subscribe to
}

message SubscriptionMessage {
    enum Action {
        SUBSCRIBE = 0;
        UNSUBSCRIBE = 1;
    }
    Action action = 1;
    string target_id = 2;
}
```

```go
// Generated Go code from protobuf
type Event struct {
    TargetId        string
    Key             string
    ServerTimestamp int64
    Payload         []byte
    PayloadSize     uint32
}

type InitMessage struct {
    Message           string
    ServerTime        int64
    AvailableTargets  []string
}

type SubscriptionMessage struct {
    Action   SubscriptionMessage_Action
    TargetId string
}
```

### 1.2 Frontend (TypeScript)

#### KeyMetrics
```typescript
interface KeyMetrics {
    key: string;                    // Unique identifier
    lastTimestamp: number | null;   // Previous client receive timestamp
    lastServerTimestamp: number | null; // Previous server send timestamp
    latencies: number[];            // Network latency history (server→client)
    intervalLatencies: number[];    // Interval between server sends
    processingTimes: number[];      // Browser processing time history
    payloadSizes: number[];         // Payload size history for throughput
    payloadTimestamps: number[];    // Timestamps for throughput calculation
    min: number;                    // Minimum interval latency (ms)
    max: number;                    // Maximum interval latency (ms)
    average: number;                // Mean interval latency (ms)
    p90: number;                    // 90th percentile latency (ms)
    avgProcessingTime: number;      // Average browser processing time (ms)
    throughput: number;             // Bytes per second
    count: number;                  // Number of measurements
    lastUpdate: number;             // Last update time for UI highlighting
    lastPayloadSize: number;        // Most recent payload size (bytes)
}
```

**Latency Array**: Fixed-size circular buffer (max 1000) to prevent unbounded growth.

#### Protobuf Types
```typescript
// Using protobufjs runtime types
import { Event, InitMessage, SubscriptionMessage } from './proto/latency';

interface ProcessedEvent {
    targetId: string;              // Which target this event belongs to
    key: string;
    serverTimestamp: number;
    clientReceiveTimestamp: number;
    payloadSize: number;
    processingStartTime: number;
    processingEndTime: number;
}

interface UserSettings {
    visibleColumns: Set<ColumnId>;
    latencyThreshold: number;  // Max acceptable latency in ms
    thresholdWarningPercent: number;  // When to show yellow (e.g., 80% of threshold)
}

type ColumnId = 'key' | 'min' | 'max' | 'avg' | 'p90' | 
                'processing' | 'throughput' | 'payload' | 'count';
```

#### Multi-Target Data Structures
```typescript
// Per-target metrics store
interface TargetMetrics {
    targetId: string;                       // Target identifier
    metrics: Map<string, KeyMetrics>;       // key -> metrics for this target
    messageCount: number;                   // Total messages for this target
    sortColumn: ColumnId;                   // Current sort column
    sortDirection: 'asc' | 'desc';          // Current sort direction
    settings: UserSettings;                 // Target-specific settings (when unlinked)
}

// Global application state
interface AppState {
    targets: Map<string, TargetMetrics>;    // targetId -> target metrics
    subscribedTargets: Set<string>;         // Currently subscribed targets
    availableTargets: string[];             // Targets available from server
    linkedMode: boolean;                    // Are targets linked for comparison?
    globalSettings: UserSettings;           // Settings when in linked mode
}

// Linked view display row (combines multiple targets)
interface LinkedRow {
    key: string;                            // The key (service name)
    targets: Map<string, KeyMetrics | null>; // targetId -> metrics (null if no data)
}

// Subscription request
interface SubscriptionRequest {
    action: 'SUBSCRIBE' | 'UNSUBSCRIBE';
    targetId: string;
}
```

**Key Design Decisions**:
- **Separate metrics per target**: Each target has its own `Map<string, KeyMetrics>`
- **Linked mode flag**: Determines display mode (linked vs unlinked)
- **Settings hierarchy**: 
  - Linked mode: Use `globalSettings` for all targets
  - Unlinked mode: Each target has its own `settings`
- **LinkedRow**: Union of keys across all targets for aligned display

---

## 2. Core Algorithms

### 2.1 Latency Calculations

#### Network Latency (Server → Client)
**Input**: 
- Server send timestamp: `server_timestamp`
- Client receive timestamp: `client_receive_timestamp`

**Output**: Network latency in nanoseconds

**Algorithm**:
```typescript
function calculateNetworkLatency(
    serverTimestamp: number, 
    clientReceiveTimestamp: number
): number {
    return clientReceiveTimestamp - serverTimestamp;
}
```

**Notes**:
- Requires synchronized clocks or accounts for drift
- Measures one-way network + serialization time
- Should always be positive (negative indicates clock skew)

#### Interval Latency (Between Events)
**Input**: 
- Current server timestamp: `t_current_server`
- Previous server timestamp: `t_previous_server`

**Output**: Interval latency in nanoseconds

**Algorithm**:
```typescript
function calculateIntervalLatency(
    currentServerTimestamp: number,
    previousServerTimestamp: number
): number {
    return currentServerTimestamp - previousServerTimestamp;
}
```

**Notes**:
- This is the primary metric for "time between events"
- First event for a key has no previous timestamp → no latency calculated
- Negative latencies indicate clock skew or out-of-order delivery (should log/handle)

#### Processing Time (Browser)
**Input**: 
- Processing start timestamp: `t_start`
- Processing end timestamp: `t_end`

**Output**: Processing time in nanoseconds

**Algorithm**:
```typescript
function measureProcessingTime(operation: () => void): number {
    const start = performance.now();
    operation();
    const end = performance.now();
    return end - start;
}
```

**Notes**:
- Uses `performance.now()` for high-resolution timing
- Measures time to deserialize protobuf + update state
- Separate from network latency

### 2.2 Percentile Calculation (P90)

**Approach**: Sort and index

**Algorithm**:
```typescript
function calculateP90(latencies: number[]): number {
    if (latencies.length === 0) {
        return 0;
    }
    
    // Create sorted copy (don't mutate original)
    const sorted = [...latencies].sort((a, b) => a - b);
    
    // Calculate 90th percentile index
    const index = Math.floor(sorted.length * 0.9);
    
    // Return value at index (0-indexed, so subtract 1)
    return sorted[Math.max(0, index - 1)];
}
```

**Complexity**: O(n log n) due to sort, where n ≤ 1000

### 2.5 Throughput Calculation

**Problem**: Calculate bytes per second from received messages

**Algorithm** (sliding window):
```typescript
function calculateThroughput(
    payloadSizes: number[],
    timestamps: number[],
    windowMs: number = 10000  // 10 second window
): number {
    if (payloadSizes.length === 0) return 0;
    
    const now = Date.now();
    const cutoff = now - windowMs;
    
    // Find messages within window
    let totalBytes = 0;
    let oldestTimestamp = now;
    
    for (let i = timestamps.length - 1; i >= 0; i--) {
        if (timestamps[i] < cutoff) break;
        totalBytes += payloadSizes[i];
        oldestTimestamp = timestamps[i];
    }
    
    const timeSpanMs = now - oldestTimestamp;
    if (timeSpanMs === 0) return 0;
    
    // Convert to bytes per second
    return (totalBytes / timeSpanMs) * 1000;
}
```

**Complexity**: O(n) where n is history size

**Display formatting**:
```typescript
function formatThroughput(bytesPerSecond: number): string {
    if (bytesPerSecond < 1024) {
        return `${bytesPerSecond.toFixed(0)} B/s`;
    }
    if (bytesPerSecond < 1024 * 1024) {
        return `${(bytesPerSecond / 1024).toFixed(2)} KiB/s`;
    }
    return `${(bytesPerSecond / (1024 * 1024)).toFixed(2)} MiB/s`;
}
```

### 2.6 Color Coding by Threshold

**Problem**: Map latency value to color based on user threshold

**Algorithm**:
```typescript
function getLatencyColor(
    latency: number,
    threshold: number,
    warningPercent: number = 80
): string {
    const warningThreshold = threshold * (warningPercent / 100);
    
    if (latency <= warningThreshold) {
        return 'text-green-600';  // Good
    }
    if (latency <= threshold) {
        // Interpolate between yellow and orange
        return 'text-yellow-600';  // Warning
    }
    return 'text-red-600';  // Critical
    }
}

// For smooth gradient (using HSL)
function getLatencyColorGradient(
    latency: number,
    threshold: number
): string {
    // 0 = green (120deg), threshold = red (0deg)
    const ratio = Math.min(latency / threshold, 1.5);
    const hue = 120 * (1 - ratio);  // 120 → 0
    return `hsl(${hue}, 70%, 50%)`;
}
```

**Complexity**: O(1)

**Optimization for production**:
- Maintain sorted structure (e.g., binary search tree)
- Use approximate algorithms for large datasets (t-digest, P² algorithm)
- Trade-off: Accuracy vs. performance

**Alternative - Efficient Implementation**:
```typescript
// Keep latencies sorted during insertion
function insertSorted(arr: number[], value: number): number[] {
    const index = arr.findIndex(x => x > value);
    if (index === -1) {
        arr.push(value);
    } else {
        arr.splice(index, 0, value);
    }
    return arr;
}

function calculateP90Efficient(sortedLatencies: number[]): number {
    if (sortedLatencies.length === 0) return 0;
    const index = Math.floor(sortedLatencies.length * 0.9);
    return sortedLatencies[Math.max(0, index - 1)];
}
```

**Complexity**: O(n) insertion, O(1) percentile query

### 2.3 Rolling Average (Mean)

**Note**: This applies to all metrics: interval latency, processing time, etc.

**Problem**: Efficiently update average without storing all values

**Algorithm** (incremental mean):
```typescript
function updateAverage(
    currentAvg: number,
    currentCount: number,
    newValue: number
): number {
    const newCount = currentCount + 1;
    return (currentAvg * currentCount + newValue) / newCount;
}
```

**Derivation**:
```
avg_new = (sum_all_values + new_value) / (count + 1)
        = (avg_old * count + new_value) / (count + 1)
```

**Complexity**: O(1)

**Precision**: May accumulate floating-point errors over millions of updates. Consider periodic recalculation from array if needed.

### 2.4 Min/Max Update

**Algorithm**:
```typescript
function updateMinMax(
    currentMin: number,
    currentMax: number,
    newValue: number
): { min: number; max: number } {
    return {
        min: Math.min(currentMin, newValue),
        max: Math.max(currentMax, newValue)
    };
}
```

**Initialization**: 
- `min`: Initialize to `Infinity` or first value
- `max`: Initialize to `0` or first value

**Complexity**: O(1)

### 2.7 Animation Algorithms

#### Flash Animation on Update
**Purpose**: Visual feedback when a row receives new data

**Algorithm** (CSS transition):
```typescript
function triggerFlashAnimation(rowElement: HTMLElement) {
    // Add flash class
    rowElement.classList.add('flash-update');
    
    // Remove after animation completes
    setTimeout(() => {
        rowElement.classList.remove('flash-update');
    }, 500);
}
```

**CSS**:
```css
@keyframes flash {
    0%, 100% { background-color: transparent; }
    50% { background-color: rgba(59, 130, 246, 0.3); }
}

.flash-update {
    animation: flash 500ms ease-in-out;
}
```

#### Bubble Sort Animation
**Purpose**: Smooth row position change when re-sorting

**Using Framer Motion**:
```typescript
import { motion, AnimatePresence } from 'framer-motion';

function AnimatedTableRow({ row, index }: Props) {
    return (
        <motion.tr
            layout  // Automatically animate position changes
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 20 }}
            transition={{
                layout: { duration: 0.3, ease: 'easeOut' },
                opacity: { duration: 0.2 }
            }}
            className={getRowClassName(row)}
        >
            {/* ... cell content */}
        </motion.tr>
    );
}
```

**Complexity**: O(1) per row, handled by GPU

### 2.8 Complete Metrics Update

**Full algorithm** combining all operations:

```typescript
function updateMetrics(
    current: KeyMetrics,
    newIntervalLatency: number,
    newProcessingTime: number,
    newPayloadSize: number,
    timestamp: number
): KeyMetrics {
    // 1. Add to histories
    const intervalLatencies = [...current.intervalLatencies, newIntervalLatency];
    const processingTimes = [...current.processingTimes, newProcessingTime];
    const payloadSizes = [...current.payloadSizes, newPayloadSize];
    const payloadTimestamps = [...current.payloadTimestamps, timestamp];
    
    // 2. Maintain fixed size (circular buffer)
    const maxHistory = 1000;
    if (intervalLatencies.length > maxHistory) {
        intervalLatencies.shift();
        processingTimes.shift();
        payloadSizes.shift();
        payloadTimestamps.shift();
    }
    
    // 3. Update count
    const count = current.count + 1;
    
    // 4. Update min/max (interval latency)
    const min = Math.min(current.min, newIntervalLatency);
    const max = Math.max(current.max, newIntervalLatency);
    
    // 5. Update averages (incremental)
    const average = (current.average * current.count + newIntervalLatency) / count;
    const avgProcessingTime = (current.avgProcessingTime * current.count + newProcessingTime) / count;
    
    // 6. Calculate P90 (from interval latency history)
    const p90 = calculateP90(intervalLatencies);
    
    // 7. Calculate throughput (10 second window)
    const throughput = calculateThroughput(payloadSizes, payloadTimestamps);
    
    // 8. Update timestamp for UI
    const lastUpdate = Date.now();
    
    return {
        ...current,
        intervalLatencies,
        processingTimes,
        payloadSizes,
        payloadTimestamps,
        min,
        max,
        average,
        p90,
        avgProcessingTime,
        throughput,
        count,
        lastUpdate,
        lastPayloadSize: newPayloadSize
    };
}
```

**Complexity**: O(n log n) due to P90 calculation, where n ≤ 1000

**Optimization**: Pre-sort latencies array, use insertion sort for new values

---

## 3. WebSocket Protocol Details

### 3.1 Connection Lifecycle

#### Client → Server (Handshake)
```http
GET /ws HTTP/1.1
Host: localhost:8080
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==
Sec-WebSocket-Version: 13
```

#### Server → Client (Upgrade)
```http
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: HSmrc0sMlYUkAGmm5OPpG2HaGWk=
```

### 3.2 Message Frames

All messages are JSON-encoded text frames.

#### Event Frame
```json
{
  "type": "event",
  "data": {
    "key": "service-A",
    "timestamp": 1699534860123
  }
}
```

#### Init Frame (on connect)
```json
{
  "type": "init",
  "data": {
    "message": "Connected to latency monitor",
    "serverTime": 1699534860123
  }
}
```

#### Error Frame
```json
{
  "type": "error",
  "data": {
    "error": "Internal server error",
    "code": "ERR_INTERNAL"
  }
}
```

### 3.3 Keepalive (Ping/Pong)

**Server behavior**:
- Send PING frame every 30 seconds
- Expect PONG within 10 seconds
- Close connection if no PONG received

**Client behavior**:
- Respond to PING with PONG (automatic in most WS libraries)
- Send PING if no message received in 45 seconds (optional)

**Go Implementation**:
```go
const (
    writeWait = 10 * time.Second
    pongWait = 60 * time.Second
    pingPeriod = (pongWait * 9) / 10  // 54 seconds
)

// In writePump goroutine
ticker := time.NewTicker(pingPeriod)
defer ticker.Stop()

for {
    select {
    case message := <-client.send:
        // ... send message
    case <-ticker.C:
        if err := client.conn.WriteControl(
            websocket.PingMessage,
            []byte{},
            time.Now().Add(writeWait),
        ); err != nil {
            return
        }
    }
}
```

**JavaScript Implementation**:
```typescript
// Automatic in browser WebSocket API
// Manually send ping if needed:
ws.send(JSON.stringify({ type: 'ping' }));
```

---

## 4. Concurrency & Threading

### 4.1 Backend (Go)

#### Hub Goroutine
Single goroutine manages all clients.

```go
func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.mu.Lock()
            h.clients[client] = true
            h.mu.Unlock()
            
        case client := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
            h.mu.Unlock()
            
        case message := <-h.broadcast:
            h.mu.RLock()
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    // Client send buffer full, disconnect
                    close(client.send)
                    delete(h.clients, client)
                }
            }
            h.mu.RUnlock()
        }
    }
}
```

**Concurrency model**:
- One goroutine per concern (register, unregister, broadcast)
- Channels for communication (no shared memory)
- Mutex protects `clients` map during iteration

#### Client Goroutines
Two goroutines per client.

**readPump**: Reads from WebSocket
```go
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })
    
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        // Process message if needed
    }
}
```

**writePump**: Writes to WebSocket
```go
func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    
    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                // Hub closed channel
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            
            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

#### Generator Goroutine
Single goroutine generates events.

```go
func Start(cfg Config, hub *Hub) {
    go func() {
        rand.Seed(time.Now().UnixNano())
        
        for {
            // Random key
            key := cfg.Keys[rand.Intn(len(cfg.Keys))]
            
            // Random interval
            minNS := cfg.MinInterval.Nanoseconds()
            maxNS := cfg.MaxInterval.Nanoseconds()
            intervalNS := minNS + rand.Int63n(maxNS-minNS)
            
            time.Sleep(time.Duration(intervalNS) * time.Nanosecond)
            
            // Create event
            event := EventMessage{
                Type: "event",
                Data: EventData{
                    Key:       key,
                    Timestamp: time.Now().UnixNano(),
                },
            }
            
            message, _ := json.Marshal(event)
            hub.broadcast <- message
        }
    }()
}
```

### 4.2 Frontend (JavaScript/React)

**Single-threaded** event loop with async operations.

#### WebSocket Event Handlers
```typescript
useEffect(() => {
    const ws = new WebSocket(url);
    
    ws.onopen = () => {
        console.log('Connected');
        setConnected(true);
    };
    
    ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        handleMessage(message); // React state update
    };
    
    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
    
    ws.onclose = () => {
        console.log('Disconnected');
        setConnected(false);
        scheduleReconnect(); // Exponential backoff
    };
    
    return () => ws.close();
}, [url]);
```

**State Updates**: Batched by React for performance

---

## 4.3 Multi-Target Handling

### Backend: Target Subscriptions

**Track subscriptions per client**:
```go
type Client struct {
    hub           *Hub
    conn          *websocket.Conn
    send          chan []byte
    subscriptions map[string]bool   // targetId -> subscribed
    mu            sync.RWMutex      // Protect subscriptions map
}

// Subscribe client to target
func (c *Client) Subscribe(targetId string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.subscriptions[targetId] = true
}

// Unsubscribe client from target
func (c *Client) Unsubscribe(targetId string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.subscriptions, targetId)
}

// Check if client is subscribed to target
func (c *Client) IsSubscribed(targetId string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.subscriptions[targetId]
}
```

**Broadcast only to subscribed clients**:
```go
func (h *Hub) BroadcastToTarget(targetId string, message []byte) {
    for client := range h.clients {
        if client.IsSubscribed(targetId) {
            select {
            case client.send <- message:
            default:
                // Buffer full, disconnect client
                close(client.send)
                delete(h.clients, client)
            }
        }
    }
}
```

**Handle subscription messages**:
```go
func (c *Client) readPump() {
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        
        // Try to decode as SubscriptionMessage
        var subMsg pb.SubscriptionMessage
        if err := proto.Unmarshal(message, &subMsg); err == nil {
            if subMsg.Action == pb.SubscriptionMessage_SUBSCRIBE {
                c.Subscribe(subMsg.TargetId)
            } else {
                c.Unsubscribe(subMsg.TargetId)
            }
        }
    }
}
```

### Frontend: Multi-Target State Management

**Update metrics for specific target**:
```typescript
function handleEvent(event: Event) {
    const { targetId, key, serverTimestamp, payloadSize } = event;
    const clientReceiveTimestamp = Date.now();
    
    // Get or create target metrics
    setAppState(prev => {
        const targets = new Map(prev.targets);
        
        if (!targets.has(targetId)) {
            targets.set(targetId, {
                targetId,
                metrics: new Map(),
                messageCount: 0,
                sortColumn: 'avg',
                sortDirection: 'desc',
                settings: { ...prev.globalSettings }
            });
        }
        
        const target = targets.get(targetId)!;
        
        // Update metrics for this key within this target
        const updatedMetrics = new Map(target.metrics);
        const current = updatedMetrics.get(key) || createEmptyMetrics(key);
        
        // Calculate latencies
        const processingStart = performance.now();
        // ... process event ...
        const processingEnd = performance.now();
        const processingTime = processingEnd - processingStart;
        
        if (current.lastServerTimestamp !== null) {
            const intervalLatency = serverTimestamp - current.lastServerTimestamp;
            const updated = updateMetrics(
                current, 
                intervalLatency, 
                processingTime, 
                payloadSize, 
                clientReceiveTimestamp
            );
            updatedMetrics.set(key, updated);
        } else {
            // First message for this key
            updatedMetrics.set(key, {
                ...current,
                lastTimestamp: clientReceiveTimestamp,
                lastServerTimestamp: serverTimestamp,
                lastPayloadSize: payloadSize
            });
        }
        
        // Update target
        target.metrics = updatedMetrics;
        target.messageCount++;
        targets.set(targetId, target);
        
        return { ...prev, targets };
    });
}
```

**Generate linked view rows**:
```typescript
function generateLinkedRows(
    targets: Map<string, TargetMetrics>,
    subscribedTargets: Set<string>
): LinkedRow[] {
    // Collect all unique keys across all subscribed targets
    const allKeys = new Set<string>();
    for (const targetId of subscribedTargets) {
        const target = targets.get(targetId);
        if (target) {
            for (const key of target.metrics.keys()) {
                allKeys.add(key);
            }
        }
    }
    
    // Build linked rows
    const rows: LinkedRow[] = [];
    for (const key of allKeys) {
        const row: LinkedRow = {
            key,
            targets: new Map()
        };
        
        for (const targetId of subscribedTargets) {
            const target = targets.get(targetId);
            const metrics = target?.metrics.get(key) || null;
            row.targets.set(targetId, metrics);
        }
        
        rows.push(row);
    }
    
    return rows;
}
```

**Sorting in linked vs unlinked mode**:
```typescript
// Unlinked: Each target sorts independently
function sortTarget(
    targetMetrics: TargetMetrics,
    column: ColumnId,
    direction: 'asc' | 'desc'
): KeyMetrics[] {
    const rows = Array.from(targetMetrics.metrics.values());
    return rows.sort((a, b) => {
        const aVal = a[column];
        const bVal = b[column];
        return direction === 'asc' ? aVal - bVal : bVal - aVal;
    });
}

// Linked: Sort by one target's values, align others
function sortLinkedRows(
    rows: LinkedRow[],
    primaryTargetId: string,
    column: ColumnId,
    direction: 'asc' | 'desc'
): LinkedRow[] {
    return rows.sort((a, b) => {
        const aMetrics = a.targets.get(primaryTargetId);
        const bMetrics = b.targets.get(primaryTargetId);
        
        // Handle null metrics
        if (!aMetrics && !bMetrics) return 0;
        if (!aMetrics) return direction === 'asc' ? 1 : -1;
        if (!bMetrics) return direction === 'asc' ? -1 : 1;
        
        const aVal = aMetrics[column];
        const bVal = bMetrics[column];
        return direction === 'asc' ? aVal - bVal : bVal - aVal;
    });
}
```

---

## 5. Performance Considerations

### 5.1 Time Complexity

| Operation | Backend | Frontend |
|-----------|---------|----------|
| Broadcast message | O(n) clients | - |
| Broadcast to target | O(n) clients (check subscriptions) | - |
| Add client | O(1) | - |
| Remove client | O(1) | - |
| Subscribe/unsubscribe | O(1) | - |
| Receive message | - | O(1) |
| Update metrics | - | O(m log m)* |
| Sort table (single target) | - | O(k log k)** |
| Generate linked rows | - | O(t × k)*** |
| Sort linked view | - | O(k log k) |

*m = history size (max 1000)  
**k = number of keys  
***t = number of subscribed targets

### 5.2 Space Complexity

**Backend**:
- Hub: O(n) for n clients
- Each client: O(1) + buffer size + O(t) for subscriptions
- Total: O(n × (buffer_size + t))

**Frontend (Single Target)**:
- Metrics map: O(k) for k keys
- History per key: O(k × m) for m measurements
- Total: O(k × m) ≈ O(k × 1000)

**Frontend (Multi-Target)**:
- Metrics per target: O(t × k × m) where t = subscribed targets
- Linked view rows: O(k) rows × O(t) target pointers = O(k × t)
- Total: O(t × k × m)
- Example: 3 targets × 20 keys × 1000 history = 60k entries

**Key Insight**: Memory scales linearly with number of subscribed targets

### 5.3 Bottlenecks & Optimizations

#### Backend Bottlenecks
1. **Broadcast to many clients**: O(n) iteration
   - **Mitigation**: Use goroutines for parallel send (with semaphore)
   - **Trade-off**: Complexity vs. throughput

2. **JSON marshaling**: Repeated per message
   - **Mitigation**: Marshal once, broadcast bytes
   - **Already implemented** in example code

3. **Client send buffer full**
   - **Mitigation**: Disconnect slow clients
   - **Already implemented** with non-blocking send

#### Frontend Bottlenecks
1. **P90 calculation**: O(n log n) per update
   - **Mitigation**: 
     - Keep latencies sorted during insertion
     - Use approximate algorithms (P² estimator)
     - Calculate P90 only on-demand (user clicks column)

2. **React re-renders**: Full table re-render per message
   - **Mitigation**:
     - Use `React.memo` on table rows
     - Batch state updates with `unstable_batchedUpdates`
     - Use keys correctly for list reconciliation

3. **Large number of keys**: 1000+ keys in table
   - **Mitigation**:
     - Virtual scrolling (react-window)
     - Pagination
     - Filter/search to reduce visible items

---

## 6. Error Handling

### 6.1 Backend Errors

**WebSocket upgrade failure**:
```go
conn, err := upgrader.Upgrade(w, r, nil)
if err != nil {
    log.Printf("Upgrade failed: %v", err)
    http.Error(w, "Could not open websocket", 400)
    return
}
```

**Broadcast failure** (client disconnected):
```go
select {
case client.send <- message:
default:
    // Buffer full or closed, disconnect client
    close(client.send)
    delete(h.clients, client)
}
```

**Generator panic** (should never happen, but protect):
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Generator panic: %v", r)
            // Restart generator
            Start(cfg, hub)
        }
    }()
    // ... generator logic
}()
```

### 6.2 Frontend Errors

**WebSocket connection failure**:
```typescript
ws.onerror = (error) => {
    console.error('WebSocket error:', error);
    setError('Connection failed');
};

ws.onclose = () => {
    // Exponential backoff reconnection
    const delay = Math.min(1000 * Math.pow(2, attempts), 30000);
    setTimeout(reconnect, delay);
};
```

**Message parsing failure**:
```typescript
ws.onmessage = (event) => {
    try {
        const message = JSON.parse(event.data);
        handleMessage(message);
    } catch (error) {
        console.error('Failed to parse message:', error);
        // Don't crash, continue processing
    }
};
```

**Invalid latency** (negative, NaN):
```typescript
function calculateLatency(current: number, previous: number): number | null {
    const latency = current - previous;
    
    if (latency < 0) {
        console.warn('Negative latency detected (clock skew?)');
        return null; // Skip this measurement
    }
    
    if (!isFinite(latency)) {
        console.warn('Invalid latency value');
        return null;
    }
    
    return latency;
}
```

---

## 7. Testing Strategy

### 7.1 Unit Tests

**Backend**:
- Hub register/unregister logic
- Broadcast to multiple clients
- Event generator frequency distribution
- Message serialization/deserialization

**Frontend**:
- Percentile calculation correctness
- Average calculation with edge cases
- Min/max update logic
- Latency history circular buffer

### 7.2 Integration Tests

- WebSocket connection establishment
- Message delivery end-to-end
- Reconnection after disconnect
- Multiple concurrent clients
- High-frequency message handling

### 7.3 Load Tests

- 100+ concurrent clients
- 1000+ messages per second
- 1000+ unique keys
- 24-hour stability test
- Memory leak detection

---

## 8. Security Considerations

### 8.1 WebSocket Security

**Origin checking** (CORS):
```go
upgrader := websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        return origin == "https://yourdomain.com"
    },
}
```

**Rate limiting**:
- Limit connections per IP
- Limit messages per client
- Disconnect abusive clients

**Authentication** (future):
- Token-based auth in upgrade request
- Validate token before upgrade
- Associate client with user/tenant

### 8.2 Input Validation

**Backend**:
- Validate message format
- Sanitize key names (prevent injection)
- Limit message size

**Frontend**:
- Validate timestamp ranges
- Handle unexpected message types gracefully
- Sanitize display values (prevent XSS)

---

## 9. Monitoring & Observability

### 9.1 Metrics to Track

**Backend**:
- Active WebSocket connections
- Messages broadcast per second
- Client connect/disconnect rate
- Message queue depth
- Goroutine count
- Memory usage

**Frontend**:
- WebSocket connection uptime
- Messages received per second
- Number of active keys
- Latency calculation errors
- UI render time

### 9.2 Logging

**Structured logging** (JSON):
```go
log.Printf(`{"level":"info","msg":"Client connected","remote":"%s","clients":%d}`,
    r.RemoteAddr, len(hub.clients))
```

**Log levels**:
- ERROR: Connection failures, panics
- WARN: Client disconnect, buffer full
- INFO: Client connect, startup
- DEBUG: Individual messages (disable in prod)

---

## 10. Deployment Checklist

- [ ] Environment variables configured
- [ ] CORS/Origin checking enabled
- [ ] TLS/SSL certificates configured (wss://)
- [ ] Rate limiting implemented
- [ ] Logging configured
- [ ] Monitoring/metrics enabled
- [ ] Health check endpoint working
- [ ] Graceful shutdown implemented
- [ ] Load tested with expected traffic
- [ ] Memory profiled for leaks

---

## References

- **WebSocket RFC**: https://tools.ietf.org/html/rfc6455
- **Gorilla WebSocket**: https://github.com/gorilla/websocket
- **Percentile algorithms**: https://en.wikipedia.org/wiki/Percentile
- **TanStack Table**: https://tanstack.com/table/latest
