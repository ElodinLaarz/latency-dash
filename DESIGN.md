# Latency Dashboard - Design Document

## Overview
A real-time latency monitoring dashboard that tracks time intervals between keyed updates from a server. The system displays statistical metrics (min, max, average, p90) for each key and provides dynamic sorting capabilities as new data arrives.

## Architecture

### High-Level Design
```
┌─────────────────┐         WebSocket            ┌─────────────────┐
│                 │◄─────────────────────────────┤                 │
│   React SPA     │  MetricsUpdate messages      │   Go Backend    │
│   (Frontend)    │  Subscribe to targets        │   (Server)      │
│                 │─────────────────────────────►│                 │
│  DISPLAYS ONLY  │   SubscriptionMessage        │  COMPUTES ALL   │
└─────────────────┘                              └─────────────────┘
        │                                                 │
        │  Multiple target displays                       │
        │  (side-by-side or tiled)                        │
        │  Toggle: Split/Combine by metadata              │
        ▼                                                 ▼
  Browser State                                   Metrics Calculator
  - Receive computed metrics                      - Process events
  - Display metrics                               - Calculate min/max/avg/p90
  - Split/combine toggle                          - Track per key+metadata
  - Metadata in rows                              - Broadcast MetricsUpdate
  (ephemeral)                                     - Timeout inactive targets
                                                          |
                                                          ▼
                                                  Event Generators
                                                  - Target A + metadata
                                                  - Target B + metadata
                                                  - Target N...
                                                  (parallel, with metadata)
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

#### A. Event Generator (Multi-Target with Metadata)
- Generates random keyed events for **N parallel targets** at variable intervals
- Each target runs independently with its own goroutine
- Configurable parameters per target:
  - Target ID (e.g., "prod-us-east", "prod-eu-west", "staging")
  - Number of unique keys (default: 10-20)
  - Min/max interval between messages (default: 100ms - 5000ms)
  - Key naming pattern (default: "service-A", "service-B", etc.)
  - Payload size range
- Events include metadata (map[string]string)
  - Example: `{"tier": "premium", "region": "us-east"}`
  - Metadata affects latency/payload for demonstration
  - Free tier: 50% slower, smaller payloads
  - Enterprise: 30% faster, 2× larger payloads
  - Regional variance: EU 40% slower than US
- Targets can have different keys and different update rates
- Events are **internal only** (not sent to clients)

#### B. Metrics Calculator (Server-Side)
- Receives events from generators (not sent to clients)
- Calculates metrics server-side for each key (or key+metadata)
- Per-target monitors with configurable timeout
- Split vs Combined mode:
  - Combined: One metric row per key (ignores metadata)
  - Split: Separate metric rows per key+metadata combination
- Tracks per client preference: Different clients can request split vs combined
- Computes:
  - Min/Max/Avg interval latency
  - P90 percentile latency
  - Average backend processing time
  - Throughput (bytes/sec)
  - Message count
- Maintains history: Circular buffer of 1000 measurements per key
- Broadcasts MetricsUpdate messages to subscribed clients
- Timeout behavior: Continues tracking for N seconds after last unsubscribe
  - Allows client to resubscribe without losing data
  - Default timeout: 90 seconds to 5 minutes (configurable)

#### C. WebSocket Manager (Multi-Target)
- Maintains active client connections
- **Target subscription model**: Clients subscribe to specific target(s)
- Broadcasts **MetricsUpdate** messages (not raw events)
- Filters by split/combined preference per client
- Handles connection lifecycle (connect, disconnect, reconnect)
- Implements heartbeat/ping-pong for connection health
- **Subscription messages**:
  - `SUBSCRIBE target_id split_by_metadata` - Start receiving metrics
  - `UNSUBSCRIBE target_id` - Stop receiving metrics
  - Supports multiple concurrent subscriptions per client
  - Client can change split preference

#### D. REST API (optional for initial version)
- `GET /health` - Health check endpoint
- `GET /api/stats` - Current aggregated statistics (if needed)

### 2. Frontend Components

#### A. WebSocket Client
- Establishes and maintains connection to backend
- Handles reconnection with exponential backoff
- **Sends SubscriptionMessage** to subscribe/unsubscribe
- **Receives MetricsUpdate** messages (precomputed metrics)
- Decodes protobuf binary messages
- Updates React state with received metrics

#### B. Data Store (React State) - **SIMPLIFIED**
- **No calculation logic** - just stores received metrics
- Structure:
```typescript
interface KeyMetrics {
  key: string;
  metadata: Record<string, string>;  // Empty {} if combined mode
  
  // All metrics received from backend (already computed)
  min: number;
  max: number;
  average: number;
  p90: number;
  avgProcessingTime: number;  // Backend processing time
  throughput: number;  // Bytes per second
  count: number;
  lastPayloadSize: number;
  lastUpdate: number;  // For UI highlighting
}
```

**Note**: Frontend is now **display-only**. All calculations happen server-side.

#### C. Split/Combined Toggle
- **UI Control**: Toggle per target to split by metadata
- When toggled, sends new SubscriptionMessage with updated preference
- **Split mode**: Shows separate rows for each key+metadata combination
  - E.g., "api {tier:free, region:us-east}" and "api {tier:premium, region:eu-west}"
- **Combined mode**: Shows single row per key (metadata ignored)
  - E.g., "api" with aggregated metrics across all metadata values

#### D. Table Component with Metadata
- Displays metrics in sortable columns
- Columns: Key, Min, Max, Average, P90, Processing, Throughput, Payload, Count, Metadata
- Metadata column (optional, visible in split mode)
- Expandable rows showing full metadata
  - Click arrow icon to expand row
  - Shows metadata key-value pairs in dropdown
- Real-time sorting based on selected column
- Visual indicators for recent updates
- Color coding based on thresholds

## Data Flow

### Message Flow (Multi-Target with Backend Metrics)
1. **Backend generates event** (per target) - **INTERNAL ONLY**
   ```protobuf
   message Event {
     string target_id = 1;         // Which target this event belongs to
     string key = 2;               // Service/component identifier
     int64 server_timestamp = 3;   // When server sent (Unix nanos)
     bytes payload = 4;            // Random data for throughput testing
     uint32 payload_size = 5;      // Size in bytes for easy reference
     map<string, string> metadata = 6;  // Event metadata
   }
   ```
   **Note**: Events stay on server; NOT sent to clients

2. **Backend Metrics Calculator processes event**
   - Looks up TargetMonitor for this target_id
   - Checks which clients are subscribed (split vs combined preference)
   - For split mode:
     - Creates metricsKey = "key|meta1:val1|meta2:val2"
     - Calculates/updates metrics for this specific key+metadata combo
   - For combined mode:
     - Creates metricsKey = "key" (ignores metadata)
     - Calculates/updates aggregated metrics across all metadata
   - Updates circular buffer (max 1000 measurements)
   - Recalculates min/max/avg/p90/throughput
   - Records processing time

3. **Backend broadcasts MetricsUpdate** via WebSocket
   ```protobuf
   message MetricsUpdate {
     string target_id = 1;
     string key = 2;
     map<string, string> metadata = 3;  // Empty if combined mode
     double min_latency = 4;            // All computed server-side
     double max_latency = 5;
     double avg_latency = 6;
     double p90_latency = 7;
     double avg_processing_time = 8;
     double throughput = 9;
     uint64 count = 10;
     uint32 last_payload_size = 11;
     int64 last_update = 12;
   }
   ```
   **Sent only to clients subscribed to target_id with matching split preference**

4. **Frontend receives MetricsUpdate** - **DISPLAY ONLY**
   - Deserialize protobuf message
   - Extract target_id to determine which display to update
   - **No calculations** - metrics already computed
   - Update React state with received metrics
   - Create/update row for this key (and metadata if split mode)

5. **Frontend updates UI** (per target display)
   - Trigger row flash animation (brief highlight) in the specific target's table
   - If **linked mode**: Update all target displays for this key simultaneously
   - If **unlinked mode**: Only re-sort the specific target's table
   - Re-calculate sorted order based on active sort column
   - Animate row position change (bubble up/down to new position)
   - Re-render table with updated metrics
   - Apply color coding based on user-defined thresholds (green → yellow → red)
   - Show only user-selected columns
   - If split mode, show metadata badge or expandable row

### WebSocket Message Protocol (Protocol Buffers)

#### Protocol Buffer Schema
```protobuf
syntax = "proto3";

package latency;

// Internal event (server-only, not sent to clients)
message Event {
  string target_id = 1;
  string key = 2;
  int64 server_timestamp = 3;
  bytes payload = 4;
  uint32 payload_size = 5;
  map<string, string> metadata = 6; 
}

// Computed metrics sent to clients
message MetricsUpdate {
  string target_id = 1;
  string key = 2;
  map<string, string> metadata = 3;  // Empty if combined mode
  double min_latency = 4;
  double max_latency = 5;
  double avg_latency = 6;
  double p90_latency = 7;
  double avg_processing_time = 8;
  double throughput = 9;
  uint64 count = 10;
  uint32 last_payload_size = 11;
  int64 last_update = 12;
}

message InitMessage {
  string message = 1;
  int64 server_time = 2;
  repeated string available_targets = 3;
}

message SubscriptionMessage {
  enum Action {
    SUBSCRIBE = 0;
    UNSUBSCRIBE = 1;
  }
  Action action = 1;
  string target_id = 2;
  bool split_by_metadata = 3;  // Split or combine by metadata
}
```

#### Server → Client: MetricsUpdate Message (Binary)
Protobuf-encoded `MetricsUpdate` message sent as binary WebSocket frame.
**Only sent to clients subscribed to that target_id with matching split preference.**

**Client receives computed metrics** - no calculations needed.

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
│  Target: prod-us-east                  [+ Add Target]   │
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

#### Split by Metadata View
```
┌───────────────────────────────────────────────────────────────────────┐
│  Latency Monitor Dashboard                              [●] Live      │
│  Target: prod-us-east  [Y Split by metadata]        [+ Add Target]    │
├───────────────────────────────────────────────────────────────────────┤
│  Key      │   │ Min │ Max │ Avg │ P90 │ Throughput │ Count │ Meta     │
│  ─────────┼───┼─────┼─────┼─────┼─────┼────────────┼───────┼──────────│
│  api      │ ▼ │ 150 │ 500 │ 220 │ 350 │ 125 KiB/s  │  142  │ tier...  │
│           │   │     │     │     │     │            │       │          │
│           │ Details: tier=free, region=us-east                        │
│  ─────────┼───┼─────┼─────┼─────┼─────┼────────────┼───────┼──────────│
│  api      │ ► │ 100 │ 380 │ 180 │ 280 │ 210 KiB/s  │  201  │ tier...  │
│  api      │ ► │  70 │ 280 │ 140 │ 220 │ 315 KiB/s  │  289  │ tier...  │
│  auth     │ ► │ 120 │ 450 │ 200 │ 340 │  95 KiB/s  │  156  │ tier...  │
│  db       │ ► │ 200 │ 600 │ 310 │ 480 │  45 KiB/s  │   87  │ tier...  │
└───────────────────────────────────────────────────────────────────────┘
```
*Expandable arrow shows full metadata key-value pairs*

#### Combined Mode (Metadata Ignored)
```
┌───────────────────────────────────────────────────────────────────────┐
│  Latency Monitor Dashboard                              [●] Live      │
│  Target: prod-us-east  [X Split by metadata]        [+ Add Target]    │
├───────────────────────────────────────────────────────────────────────┤
│  Key      │ Min │ Max │ Avg │ P90 │ Throughput │ Count │              │
│  ─────────┼─────┼─────┼─────┼─────┼────────────┼────── │              │
│  api      │  70 │ 500 │ 180 │ 310 │ 217 KiB/s  │  632  │  (all meta)  │
│  auth     │ 120 │ 450 │ 200 │ 340 │  95 KiB/s  │  156  │              │
│  db       │ 200 │ 600 │ 310 │ 480 │  45 KiB/s  │   87  │              │
│  cache    │  50 │ 280 │ 110 │ 210 │ 410 KiB/s  │  453  │              │
└───────────────────────────────────────────────────────────────────────┘
```
*Metrics aggregated across all metadata values for each key*

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
  - Split/Combined mode
- **Close target**: Remove target from view (unsubscribe)

#### Metadata Features
- **Split by metadata toggle**: Per-target checkbox to enable/disable metadata splitting
- **Split mode**: Separate rows for each key+metadata combination
  - Shows how different metadata attributes affect latency
  - Example: "api" with tier=free vs tier=enterprise shown separately
  - Demonstrates performance differences (free is slower, enterprise is faster)
- **Combined mode**: Single row per key with aggregated metrics
  - Ignores metadata completely
  - Simpler view when metadata not relevant
- **Expandable metadata rows**: Click arrow to expand and view full metadata
  - Shows all key-value pairs in collapsed section
  - Compact view by default (truncated "tier...")
  - Full details on demand
- **Metadata column**: Optional column showing metadata summary
  - Visible only in split mode
  - Truncated for space efficiency
- **Server-side calculations**: Backend computes metrics for both modes
  - Metrics persist during mode toggle
  - No data loss when switching split/combined

### Available Columns (user can show/hide)
- Key (always visible)
- Min Latency
- Max Latency
- Avg Latency
- P90 Latency
- Processing Time (backend, not browser anymore)
- Throughput (KiB/s, MiB/s)
- Payload Size (current)
- Count (total messages)
- Metadata (only visible in split mode)

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

> **📋 See [TESTING_GUIDE.md](./TESTING_GUIDE.md) for comprehensive testing details**

### Testing Approach

**Incremental Testing**: Test each feature as it's built, layer by layer

1. **Unit Tests** - Individual components in isolation
2. **Integration Tests** - Components working together  
3. **E2E Tests** - Complete user workflows
4. **Manual UI Tests** - Visual behavior and UX

### Backend Tests (Go)

**Protobuf**:
- Event/MetricsUpdate serialization
- Metadata map preservation

**Event Generator**:
- Metadata generation (tier, region)
- Metadata affects latency (free 1.5×, enterprise 0.7×)
- Metadata affects payload (enterprise 2×)

**Metrics Calculator**:
- Metrics key generation (split vs combined)
- Interval latency calculation
- P90/min/max/avg correctness
- Circular buffer (max 1000 entries)
- Throughput calculation
- Split mode: separate rows per metadata
- Combined mode: aggregated rows

**Target Monitor**:
- Subscription tracking
- Timeout cleanup (after 5min inactive)
- Resubscribe within timeout preserves metrics
- Multiple clients with different split preferences

**Concurrency**:
- Race condition testing (`-race` flag)
- Thread-safe metrics updates

### Frontend Tests (TypeScript/React)

**Protobuf**:
- MetricsUpdate decoding
- SubscriptionMessage encoding

**UI Components**:
- Metrics table renders all rows
- Split mode shows metadata column
- Combined mode hides metadata
- Expandable rows show full metadata
- Column sorting (ascending/descending)
- Color coding by threshold (green/yellow/red)
- Recent update flash animation
- Throughput formatting (KiB/s, MiB/s)

**Split/Combined Toggle**:
- Toggle switches state
- Sends subscription message on change
- UI updates correctly

**State Management**:
- Metrics update state correctly
- Split metadata into separate rows
- Combined aggregates rows

### Integration Tests

**End-to-End**:
- Event → Calculator → Client pipeline
- Multiple clients receive consistent metrics
- Split clients get separate rows, combined get aggregated
- Timeout persistence (reconnect within timeout)

**Multi-Client**:
- 10+ simultaneous clients
- Different split preferences work concurrently
- No data inconsistencies

**Performance**:
- 100+ keys handled smoothly
- 10+ events/sec sustained
- UI remains responsive

### Manual UI Testing Checklist

**Visual Behavior**:
- [ ] Rows flash on update (blue pulse, 500ms)
- [ ] Sorted column has arrow indicator
- [ ] Color coding: green → yellow → red
- [ ] Expand arrow (►/▼) shows/hides metadata
- [ ] Animations smooth (300ms ease-out)
- [ ] Numbers format correctly (2 decimals)
- [ ] Connection status indicator updates

**Interactions**:
- [ ] Toggle switches between split/combined
- [ ] Click header sorts table
- [ ] Click expand shows metadata details
- [ ] Multiple expansions work simultaneously

**Performance**:
- [ ] No lag with 100+ rows
- [ ] Smooth with 10+ events/sec
- [ ] Memory stable over 10+ minutes

## Success Metrics

### Correctness
- ✅ Metrics match backend calculations (verified against test data)
- ✅ P90 calculation accurate (tested with known datasets)
- ✅ Split/combined modes produce expected results
- ✅ Metadata sorting deterministic (alphabetical)

### Performance
- ✅ WebSocket connection stability (>99% uptime)
- ✅ UI responsiveness (<100ms for sort operations)
- ✅ Supports 10+ simultaneous clients
- ✅ Handles 100+ keys efficiently
- ✅ Memory usage stable over 24hr operation

### Code Quality
- ✅ Backend test coverage >80%
- ✅ Frontend test coverage >70%
- ✅ Critical paths 100% covered (metrics calc, protobuf)
- ✅ No race conditions (verified with `-race` flag)
- ✅ All unit tests pass
- ✅ All integration tests pass
