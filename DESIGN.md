# Latency Dashboard - Design Document

## Overview
A real-time latency monitoring dashboard that tracks time intervals between keyed updates from a server. The system displays statistical metrics (min, max, average, p90) for each key and provides dynamic sorting capabilities as new data arrives.

## Architecture

### High-Level Design
```
┌─────────────────┐         WebSocket            ┌─────────────────┐
│                 │◄─────────────────────────────┤                 │
│   React SPA     │    Subscribe to targets      │   Go Backend    │
│   (Frontend)    │─────────────────────────────►│   (Server)      │
│                 │         HTTP/REST            │                 │
└─────────────────┘                              └─────────────────┘
        │                                                 │
        │  Multiple target displays                       │
        │  (side-by-side or tiled)                        │
        ▼                                                 ▼
  Browser State                                   Event Generators
  - Target A metrics                              - Target A (keys: svc-A1, svc-A2...)
  - Target B metrics                              - Target B (keys: svc-B1, svc-B2...)
  - Linked view state                             - Target C (keys: svc-C1, svc-C2...)
  (ephemeral)                                     - Target N...
                                                  (in-memory, parallel)
```

### Multi-Target Architecture
The system supports **N parallel target streams**:
- **Server**: Generates events for multiple targets simultaneously
- **Client**: Can subscribe to one or more targets
- **Display**: Each target gets its own table/dashboard element
- **Linking**: User can link targets to compare same keys across targets

## Technology Stack

### Backend: Go (Golang)
**Justification:**
- **Excellent concurrency**: Goroutines perfect for managing multiple WebSocket connections
- **High performance**: Low latency for real-time message broadcasting
- **Strong standard library**: Built-in `net/http` and WebSocket support via `gorilla/websocket`
- **Simple deployment**: Single binary with no runtime dependencies
- **Efficient memory usage**: Important for tracking many keys simultaneously

**Key Libraries:**
- `gorilla/websocket` - WebSocket implementation
- `google.golang.org/protobuf` - Protocol Buffers serialization
- Standard library for HTTP server

### Frontend: React + TypeScript + Vite
**Justification:**
- **TypeScript**: Type safety for complex data structures (latency metrics)
- **React**: Efficient re-rendering with hooks for managing real-time data
- **Vite**: Fast development experience with HMR
- **TailwindCSS**: Rapid UI development with modern styling
- **ShadCN/UI**: Pre-built accessible components for tables and sorting

**Key Libraries:**
- `react` - UI framework
- `typescript` - Type safety
- `tailwindcss` - Styling
- `lucide-react` - Icons
- `@tanstack/react-table` - Advanced table with sorting capabilities
- `protobufjs` - Protocol Buffers deserialization
- `framer-motion` - Smooth animations for row updates

## System Components

### 1. Backend Components

#### A. Event Generator (Multi-Target)
- Generates random keyed events for **N parallel targets** at variable intervals
- Each target runs independently with its own goroutine
- Configurable parameters per target:
  - Target ID (e.g., "prod-us-east", "prod-eu-west", "staging")
  - Number of unique keys (default: 10-20)
  - Min/max interval between messages (default: 100ms - 5000ms)
  - Key naming pattern (default: "service-A", "service-B", etc.)
  - Payload size range
- Targets can have different keys and different update rates

#### B. WebSocket Manager (Multi-Target)
- Maintains active client connections
- **Target subscription model**: Clients subscribe to specific target(s)
- Broadcasts events only to clients subscribed to that target
- Handles connection lifecycle (connect, disconnect, reconnect)
- Implements heartbeat/ping-pong for connection health
- **Subscription messages**:
  - `SUBSCRIBE target_id` - Start receiving events for target
  - `UNSUBSCRIBE target_id` - Stop receiving events for target
  - Supports multiple concurrent subscriptions per client

#### C. REST API (optional for initial version)
- `GET /health` - Health check endpoint
- `GET /api/stats` - Current aggregated statistics (if needed)

### 2. Frontend Components

#### A. WebSocket Client
- Establishes and maintains connection to backend
- Handles reconnection with exponential backoff
- Processes incoming messages and updates state

#### B. Latency Calculator
- Tracks last timestamp per key
- Calculates latency between consecutive messages
- Maintains running statistics:
  - **Min**: Minimum observed latency
  - **Max**: Maximum observed latency
  - **Average**: Rolling mean of all latencies
  - **P90**: 90th percentile latency
  - **Count**: Total number of intervals measured

#### C. Data Store (React State)
- Stores per-key metrics
- Maintains history for percentile calculations
- Structure:
```typescript
interface KeyMetrics {
  key: string;
  lastTimestamp: number;
  lastServerTimestamp: number;  // For separating network vs processing time
  latencies: number[];  // Keep history for percentile calc (network latency only)
  processingTimes: number[];  // Browser processing time history
  payloadSizes: number[];  // History for throughput calculation
  min: number;
  max: number;
  average: number;
  p90: number;
  avgProcessingTime: number;  // Average browser processing time
  throughput: number;  // Bytes per second
  count: number;
  lastPayloadSize: number;  // For current message
}
```

#### D. Table Component
- Displays metrics in sortable columns
- Columns: Key, Min, Max, Average, P90, Count
- Real-time sorting based on selected column
- Visual indicators for recent updates

## Data Flow

### Message Flow (Multi-Target)
1. **Backend generates event** (per target)
   ```protobuf
   message Event {
     string target_id = 1;         // Which target this event belongs to
     string key = 2;               // Service/component identifier
     int64 server_timestamp = 3;   // When server sent (Unix nanos)
     bytes payload = 4;            // Random data for throughput testing
     uint32 payload_size = 5;      // Size in bytes for easy reference
   }
   ```

2. **Backend broadcasts via WebSocket** to clients subscribed to that target

3. **Frontend receives message**
   - Record receive time (client_receive_timestamp)
   - Deserialize protobuf message
   - Extract target_id to determine which display to update
   - Calculate network latency = client_receive_timestamp - server_timestamp
   - Start processing timer
   - Extract key, payload size, and server timestamp
   - Lookup previous server timestamp for this key **within this target**
   - If previous exists, calculate interval latency = current_server_timestamp - previous_server_timestamp
   - Stop processing timer (processing_time)
   - Update statistics for this key in this target (min, max, average, p90, throughput)
   - Store timestamps and sizes for next calculation

4. **Frontend updates UI** (per target display)
   - Trigger row flash animation (brief highlight) in the specific target's table
   - If **linked mode**: Update all target displays for this key simultaneously
   - If **unlinked mode**: Only re-sort the specific target's table
   - Re-calculate sorted order based on active sort column
   - Animate row position change (bubble up/down to new position)
   - Re-render table with updated metrics
   - Apply color coding based on user-defined thresholds (green → yellow → red)
   - Show only user-selected columns

### WebSocket Message Protocol (Protocol Buffers)

#### Protocol Buffer Schema
```protobuf
syntax = "proto3";

package latency;

message Event {
  string target_id = 1;         // Which target stream (e.g., "prod-us-east")
  string key = 2;               // Service/component key (e.g., "service-A")
  int64 server_timestamp = 3;   // Unix nanoseconds when server sent
  bytes payload = 4;            // Random data (variable size)
  uint32 payload_size = 5;      // Payload size in bytes
}

message InitMessage {
  string message = 1;
  int64 server_time = 2;
  repeated string available_targets = 3;  // List of target IDs available
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

#### Server → Client: Event Message (Binary)
Protobuf-encoded `Event` message sent as binary WebSocket frame.
**Only sent to clients subscribed to that event's target_id.**

#### Server → Client: Initial State (on connect)
Protobuf-encoded `InitMessage` sent on connection establishment.
**Includes list of available targets the client can subscribe to.**

#### Client → Server: Subscribe/Unsubscribe
Protobuf-encoded `SubscriptionMessage` to manage target subscriptions.
- `SUBSCRIBE "prod-us-east"` - Start receiving events for prod-us-east
- `UNSUBSCRIBE "staging"` - Stop receiving events for staging
- Client can subscribe to multiple targets simultaneously

#### Client → Server: Ping (keepalive)
WebSocket PING frame (native protocol).

## Algorithms & Calculations

### Percentile Calculation (P90)
Two approaches:

**Approach 1: Simple (for MVP)**
- Store all latency values in array
- Sort when needed (or maintain sorted)
- Return value at 90th percentile index
- Limit history size (e.g., last 1000 measurements per key)

```typescript
function calculateP90(latencies: number[]): number {
  if (latencies.length === 0) return 0;
  const sorted = [...latencies].sort((a, b) => a - b);
  const index = Math.floor(sorted.length * 0.9);
  return sorted[index];
}
```

**Approach 2: Optimized (future enhancement)**
- Use sliding window with fixed size
- Maintain sorted structure (e.g., binary search tree)
- O(log n) insertion and percentile query

### Rolling Average
```typescript
function updateAverage(
  currentAvg: number,
  newValue: number,
  count: number
): number {
  return (currentAvg * (count - 1) + newValue) / count;
}
```

### Dynamic Sorting
- Use React state for current sort column and direction
- Apply sort function on every state update
- Leverage React's virtual DOM for efficient re-renders

## UI/UX Design

### Layout (Multi-Target)

#### Single Target View
```
┌─────────────────────────────────────────────────────────┐
│  Latency Monitor Dashboard                    [●] Live  │
│  Target: prod-us-east                  [+ Add Target]  │
├─────────────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────────────┐ │
│  │ Key        │ Min (ms) │ Max (ms) │ Avg (ms) │ P90  │ │
│  ├────────────┼──────────┼──────────┼──────────┼──────┤ │
│  │ service-A  │  120     │  4500    │  890     │ 2100 │ │
│  │ service-B  │  95      │  3200    │  650     │ 1800 │ │
│  │ service-C  │  200     │  5000    │  1200    │ 3000 │ │
│  └────────────┴──────────┴──────────┴──────────┴──────┘ │
│  Messages: 1,234                                        │
└─────────────────────────────────────────────────────────┘
```

#### Multi-Target View (Side-by-Side)
```
┌───────────────────────────────────────────────────────────────────────┐
│  Latency Monitor Dashboard                              [●] Live      │
│  [X Link Targets]  Targets: prod-us-east, staging    [+ Add Target]   │
├───────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────┐  ┌─────────────────────────┐             │
│  │ prod-us-east       [×]  │  │ staging            [×]  │             │
│  ├────────┬────┬────┬────┬─┤  ├────────┬────┬────┬────┬─┤             │
│  │ Key    │Min │Max │Avg │P│  │ Key    │Min │Max │Avg │P│             │
│  ├────────┼────┼────┼────┼─┤  ├────────┼────┼────┼────┼─┤             │
│  │ svc-A  │120 │450 │220 │3│  │ svc-A  │ 80 │350 │180 │2│             │
│  │ svc-B  │ 95 │320 │190 │2│  │ svc-B  │110 │400 │210 │3│             │
│  │ svc-C  │200 │500 │310 │4│  │ svc-C  │150 │450 │280 │3│             │
│  └────────┴────┴────┴────┴─┘  └────────┴────┴────┴────┴─┘             │
│  Msgs: 1,234                   Msgs: 987                              │
└───────────────────────────────────────────────────────────────────────┘
```

#### Linked View (Keys Aligned)
```
┌───────────────────────────────────────────────────────────────────────┐
│  Latency Monitor Dashboard                              [●] Live      │
│  [O Link Targets]  Targets: prod-us-east, staging    [+ Add Target]   │
├───────────────────────────────────────────────────────────────────────┤
│  Key     │ prod-us-east              │ staging                        │
│          │ Min  Max  Avg  P90        │ Min  Max  Avg  P90             │
│  ────────┼───────────────────────────┼────────────────────────────    │
│  svc-A   │ 120  450  220  350        │  80  350  180  250             │
│  svc-B   │  95  320  190  280        │ 110  400  210  330             │
│  svc-C   │ 200  500  310  420        │ 150  450  280  390             │
│  svc-D   │ 150  380  240  340        │  -    -    -    -  (no data)   │
│  ────────┴───────────────────────────┴────────────────────────────    │
│  Msgs: 1,234                          Msgs: 987                       │
└───────────────────────────────────────────────────────────────────────┘
```
*In linked mode, keys are aligned in same row for easy comparison*

### Features

#### Core Features
- **Sortable columns**: Click column header to sort (ascending/descending)
- **Configurable columns**: Toggle visibility of columns via settings panel
- **Visual feedback**: Flash animation on row update + bubble sort animation
- **Connection indicator**: Show WebSocket connection status
- **Message counter**: Display total messages received per target
- **Throughput display**: Real-time bytes/second calculation per key
- **Processing time**: Separate network latency from browser processing time
- **Color-coded thresholds**: User-configurable green→yellow→red gradient
- **Responsive design**: Mobile-friendly layout

#### Multi-Target Features 
- **Target selection**: Choose from available targets via dropdown
- **Multiple subscriptions**: Subscribe to multiple targets simultaneously
- **Side-by-side view**: Display multiple targets in separate panels
- **Tiled layout**: Automatically tile targets when many are selected
- **Linked mode**: Align same keys across targets for comparison
  - Keys appear on same row
  - Enables quick cross-target latency comparison
  - Union of all keys across targets (show "-" for missing data)
- **Unlinked mode**: Each target independently sortable
  - Click sort on one target affects only that target
  - Different sort orders per target
  - Independent scrolling
- **Per-target settings**: Each target can have different:
  - Visible columns
  - Sort column/direction
  - Color threshold (when unlinked)
- **Close target**: Remove target from view (unsubscribe)

### Available Columns (user can show/hide)
- Key (always visible)
- Min Latency
- Max Latency
- Avg Latency
- P90 Latency
- Processing Time (browser)
- Throughput (KiB/s, MiB/s)
- Payload Size (current)
- Count (total messages)

### Color Scheme
- **Connected**: Green indicator
- **Disconnected**: Red indicator
- **Recent update**: Flash animation (blue pulse, 500ms)
- **Sorted column**: Bold header with arrow indicator
- **Latency color coding** (based on user threshold):
  - **Green**: Below threshold (good)
  - **Yellow/Orange**: Near threshold (warning)
  - **Red**: Above threshold (critical)
- **Row animations**: Smooth position transitions when sorting (300ms ease-out)

## Implementation Steps

### Phase 1: Backend Foundation (Go)
1. **Initialize Go module**
   ```bash
   go mod init latency-dash
   ```

2. **Create basic HTTP server**
   - Main server setup
   - Health check endpoint
   - Static file serving for frontend

3. **Implement WebSocket handler**
   - Connection management
   - Client registry (map of connections)
   - Broadcast function

4. **Create event generator**
   - Goroutine that generates random events
   - Configurable keys and intervals
   - Timestamp generation

5. **Wire up broadcasting**
   - Generator → Broadcast → All clients

### Phase 2: Frontend Foundation (React + TypeScript)
1. **Initialize Vite project**
   ```bash
   npm create vite@latest frontend -- --template react-ts
   ```

2. **Install dependencies**
   ```bash
   npm install @tanstack/react-table lucide-react
   npm install -D tailwindcss postcss autoprefixer
   npx tailwindcss init -p
   ```

3. **Setup WebSocket client**
   - Custom hook: `useWebSocket`
   - Connection state management
   - Auto-reconnect logic

4. **Create latency calculator**
   - State management for metrics
   - Update functions for min/max/avg/p90
   - Per-key tracking

5. **Build table component**
   - TanStack Table integration
   - Column definitions
   - Sorting logic

### Phase 3: Integration & Polish
1. **Connect frontend to backend**
   - Configure WebSocket URL
   - Handle message parsing
   - Error handling

2. **Add visual enhancements**
   - Recent update highlighting
   - Connection status indicator
   - Loading states

3. **Testing**
   - Test with multiple keys
   - Test sorting functionality
   - Test reconnection behavior
   - Test with high message frequency

4. **Documentation**
   - README with setup instructions
   - API documentation
   - Configuration options

## File Structure

```
latency-dash/
├── backend/
│   ├── main.go                 # Entry point
│   ├── server/
│   │   ├── websocket.go       # WebSocket handler
│   │   ├── client.go          # Client management
│   │   └── hub.go             # Broadcast hub
│   ├── generator/
│   │   └── events.go          # Event generator
│   └── go.mod
│
├── frontend/
│   ├── src/
│   │   ├── App.tsx            # Main app component
│   │   ├── hooks/
│   │   │   └── useWebSocket.ts
│   │   ├── components/
│   │   │   ├── LatencyTable.tsx
│   │   │   └── ConnectionStatus.tsx
│   │   ├── types/
│   │   │   └── index.ts       # TypeScript interfaces
│   │   └── utils/
│   │       └── metrics.ts     # Calculation functions
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── tailwind.config.js
│
├── DESIGN.md                   # This document
└── README.md                   # Setup & usage
```

## Configuration Options

### Backend
```go
type Config struct {
    Port              int           // Server port (default: 8080)
    NumKeys           int           // Number of unique keys (default: 15)
    MinInterval       time.Duration // Min time between events (default: 100ms)
    MaxInterval       time.Duration // Max time between events (default: 5s)
    WebSocketPath     string        // WS endpoint (default: /ws)
}
```

### Frontend
```typescript
interface Config {
  wsUrl: string;              // WebSocket URL
  reconnectInterval: number;  // Reconnect delay (default: 3000ms)
  maxHistory: number;         // Max latencies stored (default: 1000)
  highlightDuration: number;  // Row highlight time (default: 2000ms)
}
```

## Performance Considerations

### Backend
- **Connection limit**: Consider max concurrent WebSocket connections
- **Message rate**: Monitor CPU usage with high-frequency events
- **Memory**: Event history kept in memory (consider TTL/cleanup)

### Frontend
- **State updates**: Batch updates to avoid excessive re-renders
- **History size**: Limit stored latencies per key (circular buffer)
- **Sort performance**: Optimize for large number of keys (>1000)
- **Virtual scrolling**: Consider for >100 keys visible at once

## Future Enhancements

### Phase 4+ (Optional)
1. **Persistence**
   - SQLite or PostgreSQL for historical data
   - Query API for historical analysis

2. **Advanced Features**
   - Time-series graphs (line charts per key)
   - Alerting (threshold-based notifications)
   - Key filtering/search
   - Export to CSV/JSON

3. **Multi-tenancy**
   - Multiple independent monitors
   - Authentication/authorization

4. **Advanced Statistics**
   - P50, P95, P99 percentiles
   - Standard deviation
   - Trend analysis (improving/degrading)

5. **Deployment**
   - Docker containerization
   - Docker Compose for easy setup
   - Cloud deployment guides (AWS, GCP, Azure)

## Testing Strategy

### Backend Tests
- Unit tests for event generator
- WebSocket broadcast logic
- Connection management (connect/disconnect)
- Load testing (many clients, high frequency)

### Frontend Tests
- Unit tests for metric calculations
- WebSocket reconnection logic
- Sorting algorithm correctness
- Component rendering tests (React Testing Library)

### Integration Tests
- End-to-end message flow
- Multi-client scenarios
- Network disruption handling
- Data consistency verification

## Success Metrics
- WebSocket connection stability (>99% uptime)
- UI responsiveness (<100ms for sort operations)
- Accurate latency calculations (verified against known data)
- Supports 10+ simultaneous clients
- Handles 100+ keys efficiently
- Memory usage stable over 24hr operation
