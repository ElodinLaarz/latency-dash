# Multi-Target Stream Feature

## Overview

The latency dashboard now supports **multiple parallel target streams**, enabling real-time comparison of latency metrics across different environments, regions, or services.

---

## Key Capabilities

### 1. Parallel Target Streams
- **Backend generates events for N targets** simultaneously
- Each target runs in its own goroutine
- Independent event generation with configurable:
  - Keys (service names)
  - Update intervals
  - Payload sizes
  - Target identifiers

### 2. Client Subscription Model
- **Clients choose which targets to monitor**
- Subscribe/unsubscribe via WebSocket messages
- Multiple concurrent subscriptions per client
- Only receive events for subscribed targets

### 3. Multi-Target Display
- **Side-by-side panels**: Each target gets its own table
- **Tiled layout**: Automatically arranged when many targets selected
- **Independent metrics**: Separate min/max/avg/p90 per target
- **Per-target message counts**

### 4. Linked View Mode
- **Align keys across targets** for easy comparison
- Same key appears on same row across all target columns
- Union of all keys (show "-" for missing data)
- Compare latencies side-by-side

### 5. Unlinked View Mode
- **Independent sorting** per target
- Each target can have different sort column/direction
- Separate scrolling
- Per-target settings (columns, thresholds)

---

## Architecture Changes

### Backend (Go)

#### Event Message (Protobuf)
```protobuf
message Event {
    string target_id = 1;         // Target identifier
    string key = 2;               // Service key
    int64 server_timestamp = 3;   // Unix nanoseconds
    bytes payload = 4;            // Random data
    uint32 payload_size = 5;      // Payload size
}
```

#### Subscription Protocol
```protobuf
message SubscriptionMessage {
    enum Action {
        SUBSCRIBE = 0;
        UNSUBSCRIBE = 1;
    }
    Action action = 1;
    string target_id = 2;
}
```

#### Client Subscription Tracking
```go
type Client struct {
    hub           *Hub
    conn          *websocket.Conn
    send          chan []byte
    subscriptions map[string]bool  // targetId -> subscribed
    mu            sync.RWMutex
}

func (c *Client) Subscribe(targetId string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.subscriptions[targetId] = true
}

func (c *Client) IsSubscribed(targetId string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.subscriptions[targetId]
}
```

#### Targeted Broadcast
```go
func (h *Hub) BroadcastToTarget(targetId string, message []byte) {
    for client := range h.clients {
        if client.IsSubscribed(targetId) {
            select {
            case client.send <- message:
            default:
                // Buffer full, disconnect
                close(client.send)
                delete(h.clients, client)
            }
        }
    }
}
```

#### Multi-Target Generator
```go
type TargetConfig struct {
    TargetID       string
    Keys           []string
    MinInterval    time.Duration
    MaxInterval    time.Duration
    MinPayloadSize uint32
    MaxPayloadSize uint32
}

type GeneratorConfig struct {
    Targets []TargetConfig  // N parallel targets
}

func Start(cfg GeneratorConfig, hub *Hub) {
    for _, targetCfg := range cfg.Targets {
        go startTargetGenerator(targetCfg, hub)  // One goroutine per target
    }
}
```

### Frontend (React + TypeScript)

#### Target Metrics Structure
```typescript
interface TargetMetrics {
    targetId: string;
    metrics: Map<string, KeyMetrics>;  // key -> metrics
    messageCount: number;
    sortColumn: ColumnId;
    sortDirection: 'asc' | 'desc';
    settings: UserSettings;  // Per-target settings
}
```

#### Application State
```typescript
interface AppState {
    targets: Map<string, TargetMetrics>;  // targetId -> metrics
    subscribedTargets: Set<string>;       // Currently subscribed
    availableTargets: string[];           // Available from server
    linkedMode: boolean;                  // Link targets for comparison?
    globalSettings: UserSettings;         // Settings when linked
}
```

#### Linked View Row
```typescript
interface LinkedRow {
    key: string;                            // Service key
    targets: Map<string, KeyMetrics | null>; // targetId -> metrics (null if no data)
}
```

#### Event Handling
```typescript
function handleEvent(event: Event) {
    const { targetId, key, serverTimestamp, payloadSize } = event;
    
    // Get or create target metrics
    setAppState(prev => {
        const targets = new Map(prev.targets);
        
        if (!targets.has(targetId)) {
            // Initialize new target
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
        // ...
        
        return { ...prev, targets };
    });
}
```

#### Subscription Management
```typescript
function subscribeToTarget(targetId: string) {
    const subMsg = SubscriptionMessage.create({
        action: SubscriptionMessage.Action.SUBSCRIBE,
        targetId: targetId
    });
    
    const data = SubscriptionMessage.encode(subMsg).finish();
    websocket.send(data);
    
    setAppState(prev => ({
        ...prev,
        subscribedTargets: new Set([...prev.subscribedTargets, targetId])
    }));
}

function unsubscribeFromTarget(targetId: string) {
    const subMsg = SubscriptionMessage.create({
        action: SubscriptionMessage.Action.UNSUBSCRIBE,
        targetId: targetId
    });
    
    const data = SubscriptionMessage.encode(subMsg).finish();
    websocket.send(data);
    
    setAppState(prev => {
        const newSubscribed = new Set(prev.subscribedTargets);
        newSubscribed.delete(targetId);
        return { ...prev, subscribedTargets: newSubscribed };
    });
}
```

---

## Display Modes

### Mode 1: Single Target (Default)
```
┌────────────────────────────────────────────┐
│  Target: prod-us-east     [+ Add Target]   │
├────────────────────────────────────────────┤
│  Key    │ Min │ Max │ Avg │ P90 │          │
│  api    │ 120 │ 450 │ 220 │ 350 │          │
│  auth   │  95 │ 320 │ 190 │ 280 │          │
│  db     │ 200 │ 500 │ 310 │ 420 │          │
└────────────────────────────────────────────┘
```

### Mode 2: Side-by-Side (Unlinked)
```
┌──────────────────────────────────────────────────────┐
│  [☐ Link Targets]  Targets: 2  [+ Add Target]       │
├──────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐           │
│  │ prod-us-east [×]│  │ staging     [×] │           │
│  │ Key   Avg  P90  │  │ Key   Avg  P90  │           │
│  │ api   220  350  │  │ api   180  250  │           │
│  │ auth  190  280  │  │ auth  210  330  │           │
│  │ db    310  420  │  │ db    280  390  │           │
│  └─────────────────┘  └─────────────────┘           │
└──────────────────────────────────────────────────────┘
```
- Each target independently sortable
- Click column header sorts only that target
- Different sort orders possible

### Mode 3: Linked View (Keys Aligned)
```
┌──────────────────────────────────────────────────────┐
│  [☑ Link Targets]  Targets: 2  [+ Add Target]       │
├──────────────────────────────────────────────────────┤
│  Key  │ prod-us-east    │ staging         │         │
│       │ Min Max Avg P90 │ Min Max Avg P90 │         │
│  api  │ 120 450 220 350 │  80 350 180 250 │         │
│  auth │  95 320 190 280 │ 110 400 210 330 │         │
│  db   │ 200 500 310 420 │ 150 450 280 390 │         │
│  cache│ 150 380 240 340 │  -   -   -   -  │ (no data)│
└──────────────────────────────────────────────────────┘
```
- Keys aligned in same row
- Easy cross-target comparison
- Shows "-" for keys not present in a target
- Union of all keys across targets
- Single sort applies to all (by one target's values)

---

## User Workflows

### Workflow 1: Compare Production Regions
1. User selects "prod-us-east" from dropdown
2. Dashboard displays metrics for that target
3. User clicks "+ Add Target"
4. Selects "prod-eu-west"
5. Two panels appear side-by-side
6. User enables "Link Targets"
7. Keys align for easy comparison
8. User sees `api` service has 220ms avg in US, 280ms in EU

### Workflow 2: Production vs Staging
1. Subscribe to "production" and "staging"
2. View side-by-side in unlinked mode
3. Sort production by P90 (descending) to see worst offenders
4. Sort staging by Avg (ascending) to see best performers
5. Independent analysis of each environment

### Workflow 3: Multi-Region Monitoring
1. Subscribe to 4 targets: us-east, us-west, eu-west, ap-south
2. Tiled layout (2×2 grid)
3. Enable linked mode
4. All regions aligned by key
5. Quickly spot regional latency differences
6. Red highlighting shows eu-west `db` exceeds threshold

### Workflow 4: Focus on Single Service
1. Start with all targets in linked mode
2. Notice `auth` service has high latency in one region
3. Keep only that region's target open
4. Remove link mode for detailed analysis
5. Sort by different columns
6. Investigate that specific target in detail

---

## Implementation Checklist

### Backend
- [x] Update protobuf schema with `target_id` field
- [x] Add `SubscriptionMessage` protobuf
- [x] Update `InitMessage` with `available_targets`
- [x] Add `subscriptions` map to `Client` struct
- [x] Implement `Subscribe()` and `Unsubscribe()` methods
- [x] Add `BroadcastToTarget()` method to Hub
- [x] Update event generator to support multiple targets
- [x] Modify `readPump()` to handle subscription messages
- [x] Update main.go to configure multiple targets

### Frontend
- [x] Update TypeScript types for multi-target
- [x] Create `TargetMetrics` interface
- [x] Create `AppState` with targets map
- [x] Create `LinkedRow` interface
- [x] Implement subscription/unsubscription functions
- [ ] Build target selector dropdown UI
- [ ] Build side-by-side target panels
- [ ] Implement linked view table component
- [ ] Add "Link Targets" toggle
- [ ] Implement per-target sorting (unlinked mode)
- [ ] Implement linked sorting algorithm
- [ ] Add target close button (unsubscribe)
- [ ] Handle target addition UI
- [ ] Build tiled layout for 3+ targets
- [ ] Add target-specific settings panel

---

## Performance Considerations

### Backend
**Complexity**:
- Broadcast to target: O(n) clients with subscription check
- Subscribe/Unsubscribe: O(1) with mutex

**Optimization**:
- Use `sync.RWMutex` for subscription map (many reads, few writes)
- Pre-filter clients by target before iteration
- Consider target-specific client maps for O(1) broadcast

**Memory**:
- Per client: O(t) for t subscriptions
- Total: O(n × t) where n = clients, t = avg subscriptions

### Frontend
**Complexity**:
- Update metrics: O(m log m) per key per target
- Generate linked rows: O(t × k) where t = targets, k = keys
- Sort linked view: O(k log k)

**Optimization**:
- Memoize linked rows (only regenerate on data change)
- Use `useMemo` for expensive calculations
- Virtual scrolling for > 100 keys in linked mode

**Memory**:
- O(t × k × m) where:
  - t = subscribed targets (typically 2-5)
  - k = keys per target (typically 10-50)
  - m = history size (1000)
- Example: 3 targets × 20 keys × 1000 = 60,000 measurements

### Recommendations
- **Limit active subscriptions**: Recommend 2-5 targets max
- **Lazy loading**: Don't subscribe until user requests
- **Cleanup**: Unsubscribe when closing target panel
- **Throttle updates**: If > 100 msg/sec, batch state updates

---

## Use Cases

### 1. Regional Latency Comparison
**Scenario**: Multi-region deployment (US, EU, Asia)  
**Solution**: Subscribe to all regions in linked mode  
**Benefit**: Instantly see which region has higher latency for each service

### 2. Environment Comparison
**Scenario**: Compare prod vs staging  
**Solution**: Side-by-side unlinked mode  
**Benefit**: Verify staging matches prod performance before promotion

### 3. A/B Testing
**Scenario**: Two different backend versions  
**Solution**: target_id = "version-a" and "version-b"  
**Benefit**: Real-time performance comparison

### 4. Disaster Recovery
**Scenario**: Primary and failover datacenters  
**Solution**: Monitor both simultaneously  
**Benefit**: Immediate visibility when failover latency degrades

### 5. Load Balancer Analysis
**Scenario**: Multiple backend clusters behind LB  
**Solution**: One target per cluster  
**Benefit**: Identify if one cluster is slower

---

## Configuration Examples

### Example 1: Production Multi-Region
```go
targets := []generator.TargetConfig{
    {
        TargetID:       "prod-us-east-1",
        Keys:           []string{"api", "auth", "db", "cache", "queue"},
        MinInterval:    50 * time.Millisecond,
        MaxInterval:    1 * time.Second,
        MinPayloadSize: 500,
        MaxPayloadSize: 10000,
    },
    {
        TargetID:       "prod-eu-west-1",
        Keys:           []string{"api", "auth", "db", "cache", "queue"},
        MinInterval:    50 * time.Millisecond,
        MaxInterval:    1 * time.Second,
        MinPayloadSize: 500,
        MaxPayloadSize: 10000,
    },
    {
        TargetID:       "prod-ap-south-1",
        Keys:           []string{"api", "auth", "db", "cache", "queue"},
        MinInterval:    50 * time.Millisecond,
        MaxInterval:    1 * time.Second,
        MinPayloadSize: 500,
        MaxPayloadSize: 10000,
    },
}
```

### Example 2: Prod vs Staging
```go
targets := []generator.TargetConfig{
    {
        TargetID:       "production",
        Keys:           []string{"web", "api", "worker", "db"},
        MinInterval:    100 * time.Millisecond,
        MaxInterval:    2 * time.Second,
        MinPayloadSize: 1000,
        MaxPayloadSize: 20000,
    },
    {
        TargetID:       "staging",
        Keys:           []string{"web", "api", "worker", "db"},
        MinInterval:    200 * time.Millisecond,  // Less traffic
        MaxInterval:    5 * time.Second,
        MinPayloadSize: 500,
        MaxPayloadSize: 10000,
    },
}
```

### Example 3: Microservices per Team
```go
targets := []generator.TargetConfig{
    {
        TargetID: "team-platform",
        Keys:     []string{"auth", "user-mgmt", "billing"},
    },
    {
        TargetID: "team-data",
        Keys:     []string{"analytics", "reporting", "etl"},
    },
    {
        TargetID: "team-ml",
        Keys:     []string{"inference", "training", "feature-store"},
    },
}
```

---

## Future Enhancements

### Potential Additions
1. **Target groups**: Organize targets into folders
2. **Saved layouts**: Persist target selection and arrangement
3. **Comparison mode**: Difference/ratio calculations between targets
4. **Historical comparison**: Compare current vs past target data
5. **Target health**: Show if target is active/inactive
6. **Auto-subscribe**: Subscribe to targets matching pattern
7. **Diff highlighting**: Highlight cells where targets differ significantly
8. **Export**: Download comparison data as CSV
9. **Alerts**: Trigger alerts when cross-target delta exceeds threshold
10. **Target metadata**: Show target description, region, version

---

## Migration Notes

### From Single-Target to Multi-Target

**Breaking Changes**:
- Event protobuf message now has `target_id` field
- Field numbers shifted (key is now field 2, not field 1)
- Clients must handle subscription messages

**Backward Compatibility**:
- Not maintained - full rebuild required
- Consider version field in WebSocket handshake for future upgrades

**Migration Path**:
1. Update backend protobuf schema
2. Regenerate protobuf code (Go and TypeScript)
3. Update event generator
4. Update hub to handle subscriptions
5. Update frontend to use new Event structure
6. Test with single target first
7. Add multi-target UI incrementally

---

## Testing Strategy

### Backend Tests
- **Subscription logic**: Subscribe/unsubscribe operations
- **Targeted broadcast**: Only subscribed clients receive events
- **Concurrent access**: Multiple goroutines accessing subscriptions
- **Edge cases**: Unsubscribe from non-subscribed target
- **Memory leaks**: Unsubscribe properly removes subscription

### Frontend Tests
- **Target selection**: Add/remove targets from view
- **Linked mode**: Keys properly aligned across targets
- **Unlinked sorting**: Independent sort per target
- **Missing data**: Handle keys not present in all targets
- **Subscription state**: UI reflects subscription status

### Integration Tests
- **Full flow**: Client subscribes, receives events, unsubscribes
- **Multi-client**: Multiple clients with different subscriptions
- **Stress test**: 10 targets × 100 keys × 100 events/sec
- **Network**: Reconnect preserves subscriptions (future)

---

## Summary

The multi-target stream feature transforms the latency dashboard from a single-environment monitor into a powerful **comparison and analysis tool**. Users can:

✅ **Monitor multiple environments** simultaneously  
✅ **Compare latencies** side-by-side  
✅ **Link targets** for aligned comparison  
✅ **Independently analyze** each target  
✅ **Subscribe/unsubscribe** dynamically  
✅ **Scale** to N parallel targets  

This enables use cases like regional comparison, prod/staging validation, A/B testing, and multi-cluster monitoring—all in a single, real-time dashboard.

**Implementation Status**: Design complete, ready for development 🚀
