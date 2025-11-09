# Comprehensive Testing Guide

## Testing Philosophy

**Incremental Testing**: Test each feature as it's implemented, building confidence layer by layer

- **Unit tests first**: Verify individual components in isolation
- **Integration tests**: Verify components work together
- **E2E tests**: Verify complete user workflows  
- **Manual UI testing**: Verify visual behavior and UX

---

## Phase 1: Backend Foundation Tests

### 1.1 Protobuf Message Tests (Go)

**File**: `backend/proto_test.go`

**Purpose**: Verify protobuf serialization/deserialization

```go
func TestEventMarshaling(t *testing.T) {
    event := &pb.Event{
        TargetId:        "test-target",
        Key:             "api",
        ServerTimestamp: time.Now().UnixNano(),
        Payload:         []byte("test data"),
        PayloadSize:     9,
        Metadata: map[string]string{
            "tier":   "premium",
            "region": "us-east",
        },
    }
    
    data, err := proto.Marshal(event)
    assert.NoError(t, err)
    
    decoded := &pb.Event{}
    err = proto.Unmarshal(data, decoded)
    assert.NoError(t, err)
    assert.Equal(t, event.Key, decoded.Key)
    assert.Equal(t, event.Metadata["tier"], decoded.Metadata["tier"])
}
```

**Expected Results**:
- ✅ Event message marshals to binary
- ✅ Event message unmarshals correctly
- ✅ Metadata map preserved
- ✅ All fields match after round-trip

---

### 1.2 Event Generator Tests (Go)

**File**: `backend/generator/generator_test.go`

#### Test 1: Metadata Generation

```go
func TestMetadataGeneration(t *testing.T) {
    events := make([]*pb.Event, 10)
    for i := 0; i < 10; i++ {
        event := generateEventWithMetadata(testConfig)
        assert.NotNil(t, event.Metadata)
        assert.Contains(t, []string{"free", "premium", "enterprise"}, 
                       event.Metadata["tier"])
        assert.Contains(t, []string{"us-east", "us-west", "eu-west"}, 
                       event.Metadata["region"])
    }
}
```

**Expected**: Every event has valid tier and region metadata

#### Test 2: Metadata Affects Latency

```go
func TestMetadataAffectsLatency(t *testing.T) {
    // Generate 30 events, measure intervals by tier
    eventsByTier := make(map[string][]time.Duration)
    
    // ... generate and track
    
    avgFree := average(eventsByTier["free"])
    avgPremium := average(eventsByTier["premium"])
    avgEnterprise := average(eventsByTier["enterprise"])
    
    assert.Greater(t, avgFree, avgPremium, "Free tier should be slower")
    assert.Less(t, avgEnterprise, avgPremium, "Enterprise should be faster")
}
```

**Expected**: 
- ✅ Free tier ~1.5× slower than premium
- ✅ Enterprise ~0.7× faster than premium
- ✅ Statistical variance demonstrates multipliers

#### Test 3: Metadata Affects Payload Size

```go
func TestMetadataAffectsPayloadSize(t *testing.T) {
    // Separate events by tier
    enterpriseSizes := []uint32{}
    otherSizes := []uint32{}
    
    // ... collect sizes
    
    avgEnterprise := avgUint32(enterpriseSizes)
    avgOthers := avgUint32(otherSizes)
    assert.Greater(t, avgEnterprise, avgOthers * 1.5, 
                   "Enterprise should have ~2× larger payloads")
}
```

**Expected**: Enterprise tier has approximately 2× larger payloads

---

### 1.3 Metrics Calculator Tests (Go)

**File**: `backend/calculator/metrics_test.go`

#### Test 1: Metrics Key Generation

```go
func TestMetricsKeyGeneration(t *testing.T) {
    metadata := map[string]string{"tier": "premium", "region": "us-east"}
    
    // Combined mode: just key
    combined := createMetricsKey("api", metadata, false)
    assert.Equal(t, "api", combined)
    
    // Split mode: key|sorted_metadata
    split := createMetricsKey("api", metadata, true)
    assert.Equal(t, "api|region:us-east|tier:premium", split)
    
    // Empty metadata
    splitEmpty := createMetricsKey("api", map[string]string{}, true)
    assert.Equal(t, "api", splitEmpty)
}
```

**Expected**:
- ✅ Combined mode returns just key
- ✅ Split mode returns key with sorted metadata
- ✅ Metadata sorting is deterministic (alphabetical)
- ✅ Empty metadata treated as combined

#### Test 2: Interval Latency Calculation

```go
func TestIntervalLatencyCalculation(t *testing.T) {
    calc := NewMetricsCalculator(testHub, 5*time.Minute)
    
    // First event: no interval yet
    event1 := &pb.Event{
        TargetId:        "test",
        Key:             "api",
        ServerTimestamp: 1000000000, // 1s in nanos
        Metadata:        map[string]string{"tier": "premium"},
    }
    calc.ProcessEvent(event1)
    
    monitor := calc.Targets["test"]
    metrics := monitor.Metrics["api"]
    assert.Equal(t, uint64(0), metrics.Count) // No interval yet
    
    // Second event: 500ms later
    event2 := &pb.Event{
        TargetId:        "test",
        Key:             "api",
        ServerTimestamp: 1500000000, // 1.5s in nanos
        Metadata:        map[string]string{"tier": "premium"},
    }
    calc.ProcessEvent(event2)
    
    metrics = monitor.Metrics["api"]
    assert.Equal(t, uint64(1), metrics.Count)
    assert.Equal(t, 500.0, metrics.AvgLatency) // 500ms
}
```

**Expected**:
- ✅ First event doesn't create interval (no previous timestamp)
- ✅ Second event creates 500ms interval
- ✅ Interval calculated from ServerTimestamp difference
- ✅ Latency stored in milliseconds

#### Test 3: Split vs Combined Metrics

```go
func TestSplitVsCombinedMetrics(t *testing.T) {
    calc := NewMetricsCalculator(testHub, 5*time.Minute)
    
    // Two clients with different preferences
    calc.UpdateSubscription("test", "client1", true, true)   // split
    calc.UpdateSubscription("test", "client2", true, false)  // combined
    
    // Send events with different metadata
    events := []*pb.Event{
        {TargetId: "test", Key: "api", ServerTimestamp: 1000, 
         Metadata: map[string]string{"tier": "free"}},
        {TargetId: "test", Key: "api", ServerTimestamp: 2000, 
         Metadata: map[string]string{"tier": "free"}},
        {TargetId: "test", Key: "api", ServerTimestamp: 1500, 
         Metadata: map[string]string{"tier": "premium"}},
    }
    
    for _, event := range events {
        calc.ProcessEvent(event)
    }
    
    monitor := calc.Targets["test"]
    
    // Should have BOTH split and combined metrics
    assert.Contains(t, monitor.Metrics, "api|tier:free")
    assert.Contains(t, monitor.Metrics, "api|tier:premium")
    assert.Contains(t, monitor.Metrics, "api")
}
```

**Expected**:
- ✅ Split mode creates separate metrics per metadata combo
- ✅ Combined mode creates single aggregated metric
- ✅ Calculator computes BOTH when clients want different modes
- ✅ Each client receives their preferred format

#### Test 4: P90 Calculation

```go
func TestP90Calculation(t *testing.T) {
    metrics := &KeyMetrics{
        IntervalLatencies: []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
    }
    
    metrics.recalculate()
    
    assert.Equal(t, 900.0, metrics.P90Latency)
    assert.Equal(t, 100.0, metrics.MinLatency)
    assert.Equal(t, 1000.0, metrics.MaxLatency)
    assert.InDelta(t, 550.0, metrics.AvgLatency, 1.0)
}
```

**Expected**:
- ✅ P90 of [100..1000] is 900
- ✅ Min/max correct
- ✅ Average calculated correctly

#### Test 5: Circular Buffer Limit

```go
func TestCircularBufferLimit(t *testing.T) {
    metrics := &KeyMetrics{}
    
    // Add 1200 latencies (over limit of 1000)
    for i := 0; i < 1200; i++ {
        metrics.IntervalLatencies = append(metrics.IntervalLatencies, float64(i))
        if len(metrics.IntervalLatencies) > 1000 {
            metrics.IntervalLatencies = metrics.IntervalLatencies[1:]
        }
    }
    
    assert.Equal(t, 1000, len(metrics.IntervalLatencies))
    assert.Equal(t, 200.0, metrics.IntervalLatencies[0]) // Oldest 200 removed
    assert.Equal(t, 1199.0, metrics.IntervalLatencies[999]) // Latest
}
```

**Expected**:
- ✅ Buffer caps at exactly 1000 entries
- ✅ Oldest entries removed (FIFO)
- ✅ Latest entries preserved

#### Test 6: Throughput Calculation

```go
func TestThroughputCalculation(t *testing.T) {
    metrics := &KeyMetrics{
        PayloadSizes:      []uint32{1000, 2000, 3000},
        PayloadTimestamps: []int64{
            1000000000,         // T=0
            1000000000 + 1e9,   // T=1s
            1000000000 + 2e9,   // T=2s
        },
    }
    
    metrics.recalculate()
    
    // 6000 bytes over 2 seconds = 3000 bytes/sec
    assert.InDelta(t, 3000.0, metrics.Throughput, 10.0)
}
```

**Expected**: Throughput correctly calculated as bytes/second

---

### 1.4 Target Monitor Timeout Tests (Go)

#### Test 1: Subscription Tracking

```go
func TestSubscriptionTracking(t *testing.T) {
    calc := NewMetricsCalculator(testHub, 5*time.Second)
    
    calc.UpdateSubscription("target1", "client1", true, false)
    
    monitor := calc.Targets["target1"]
    assert.True(t, monitor.HasSubscribers)
    assert.Contains(t, monitor.SplitByMetadata, "client1")
    
    calc.UpdateSubscription("target1", "client1", false, false)
    assert.False(t, monitor.HasSubscribers)
}
```

**Expected**: Subscriptions tracked and updated correctly

#### Test 2: Timeout Cleanup

```go
func TestTimeoutCleanup(t *testing.T) {
    calc := NewMetricsCalculator(testHub, 2*time.Second)
    
    calc.UpdateSubscription("target1", "client1", true, false)
    calc.UpdateSubscription("target1", "client1", false, false)
    
    // Wait less than timeout
    time.Sleep(1 * time.Second)
    calc.cleanupInactiveTargets()
    assert.Contains(t, calc.Targets, "target1", "Should still exist")
    
    // Wait past timeout
    time.Sleep(2 * time.Second)
    calc.cleanupInactiveTargets()
    assert.NotContains(t, calc.Targets, "target1", "Should be removed")
}
```

**Expected**:
- ✅ Monitor persists during timeout window
- ✅ Monitor removed after timeout expires

#### Test 3: Resubscribe Within Timeout

```go
func TestResubscribeWithinTimeout(t *testing.T) {
    calc := NewMetricsCalculator(testHub, 5*time.Second)
    
    // Subscribe, generate events, unsubscribe
    calc.UpdateSubscription("target1", "client1", true, false)
    event := &pb.Event{TargetId: "target1", Key: "api", ServerTimestamp: 1000}
    calc.ProcessEvent(event)
    
    metricsCount := len(calc.Targets["target1"].Metrics)
    
    calc.UpdateSubscription("target1", "client1", false, false)
    time.Sleep(1 * time.Second)
    
    // Resubscribe
    calc.UpdateSubscription("target1", "client1", true, false)
    
    // Metrics should be preserved
    assert.Equal(t, metricsCount, len(calc.Targets["target1"].Metrics))
}
```

**Expected**: Metrics preserved when resubscribing within timeout

---

## Phase 2: Frontend Tests

### 2.1 Protobuf Decoding Tests (TypeScript)

**File**: `frontend/src/__tests__/protobuf.test.ts`

```typescript
import { MetricsUpdate, SubscriptionMessage } from '../proto/latency';

describe('Protobuf Decoding', () => {
  test('decode MetricsUpdate message', () => {
    const mockUpdate = MetricsUpdate.create({
      targetId: 'test-target',
      key: 'api',
      metadata: { tier: 'premium', region: 'us-east' },
      minLatency: 100.5,
      maxLatency: 500.3,
      avgLatency: 220.1,
      p90Latency: 350.7,
      avgProcessingTime: 0.5,
      throughput: 125000,
      count: 142,
      lastPayloadSize: 1024,
      lastUpdate: Date.now() * 1_000_000,
    });
    
    const bytes = MetricsUpdate.encode(mockUpdate).finish();
    const decoded = MetricsUpdate.decode(bytes);
    
    expect(decoded.key).toBe('api');
    expect(decoded.metadata.tier).toBe('premium');
    expect(decoded.avgLatency).toBeCloseTo(220.1);
  });
});
```

**Expected**: Messages encode/decode correctly

---

### 2.2 UI Component Tests

**File**: `frontend/src/__tests__/components/MetricsTable.test.tsx`

#### Test 1: Render Metrics Rows

```typescript
test('renders all metrics rows', () => {
  const mockMetrics = [
    { key: 'api', metadata: { tier: 'free' }, average: 330, /* ... */ },
    { key: 'api', metadata: { tier: 'premium' }, average: 220, /* ... */ },
  ];
  
  render(<MetricsTable metrics={mockMetrics} targetId="target1" />);
  
  expect(screen.getAllByText('api').length).toBe(2);
  expect(screen.getByText('330')).toBeInTheDocument();
  expect(screen.getByText('220')).toBeInTheDocument();
});
```

**Expected**: All rows render with correct data

#### Test 2: Split Mode Shows Metadata

```typescript
test('shows metadata in split mode', () => {
  render(<MetricsTable metrics={mockMetrics} splitMode={true} />);
  
  expect(screen.getByText(/tier/)).toBeInTheDocument();
  expect(screen.getByText(/free/)).toBeInTheDocument();
  expect(screen.getByText(/premium/)).toBeInTheDocument();
});
```

**Expected**: Metadata column visible and populated

#### Test 3: Combined Mode Hides Metadata

```typescript
test('hides metadata in combined mode', () => {
  const combinedMetrics = [{ 
    key: 'api', 
    metadata: {}, 
    average: 220 
  }];
  
  render(<MetricsTable metrics={combinedMetrics} splitMode={false} />);
  
  expect(screen.queryByText(/tier:/)).not.toBeInTheDocument();
});
```

**Expected**: Metadata column hidden or empty

#### Test 4: Expandable Rows

```typescript
test('expands row to show full metadata', () => {
  render(<MetricsTable metrics={mockMetrics} splitMode={true} />);
  
  const expandButtons = screen.getAllByRole('button', { name: /expand/i });
  fireEvent.click(expandButtons[0]);
  
  expect(screen.getByText('tier: free')).toBeInTheDocument();
  expect(screen.getByText('region: us-east')).toBeInTheDocument();
});
```

**Expected**:
- ✅ Expand button present on each row
- ✅ Click expands to show full metadata
- ✅ Format: "key: value" on separate lines

#### Test 5: Sorting

```typescript
test('sorts by column', () => {
  render(<MetricsTable metrics={mockMetrics} />);
  
  const avgHeader = screen.getByText('Avg');
  fireEvent.click(avgHeader);
  
  const rows = screen.getAllByRole('row');
  // First data row should have lower average
  expect(rows[1]).toHaveTextContent('220');
});
```

**Expected**: Clicking column header sorts ascending/descending

#### Test 6: Color Coding

```typescript
test('applies color coding based on thresholds', () => {
  render(<MetricsTable metrics={mockMetrics} threshold={250} />);
  
  const cell330 = screen.getByText('330');
  const cell220 = screen.getByText('220');
  
  expect(cell330).toHaveClass(/text-red/);     // Above threshold
  expect(cell220).toHaveClass(/text-green/);   // Below threshold
});
```

**Expected**:
- ✅ Values > threshold are red
- ✅ Values 80-100% of threshold are yellow
- ✅ Values < 80% of threshold are green

#### Test 7: Highlight Recent Updates

```typescript
test('highlights recently updated rows', () => {
  const recentMetrics = [{
    ...mockMetrics[0],
    lastUpdate: Date.now(),
  }];
  
  render(<MetricsTable metrics={recentMetrics} />);
  
  const row = screen.getAllByRole('row')[1];
  expect(row).toHaveClass(/animate-flash|highlight/);
});
```

**Expected**: Rows updated in last 500ms show flash animation

#### Test 8: Throughput Formatting

```typescript
test('formats throughput correctly', () => {
  const metrics = [{ 
    key: 'api', 
    throughput: 125000,  // bytes/sec
    /* ... */ 
  }];
  
  render(<MetricsTable metrics={metrics} />);
  
  // 125000 bytes/sec = 122.07 KiB/s
  expect(screen.getByText(/122.*KiB\/s/)).toBeInTheDocument();
});
```

**Expected**: Throughput displays as KiB/s or MiB/s with 2 decimals

---

### 2.3 Split/Combined Toggle Tests

```typescript
describe('Split/Combined Toggle', () => {
  test('toggle switches state', () => {
    const onToggle = jest.fn();
    render(<SplitToggle checked={false} onChange={onToggle} />);
    
    const toggle = screen.getByRole('checkbox');
    fireEvent.click(toggle);
    
    expect(onToggle).toHaveBeenCalledWith(true);
  });
  
  test('sends subscription message on toggle', async () => {
    const { result } = renderHook(() => useWebSocket());
    
    act(() => {
      result.current.toggleSplitMode('target1', true);
    });
    
    // Should send UNSUBSCRIBE then SUBSCRIBE with new preference
    expect(result.current.sentMessages).toHaveLength(2);
  });
});
```

**Expected**:
- ✅ Toggle UI element works
- ✅ Changing toggle sends new SubscriptionMessage
- ✅ Backend receives updated split preference

---

## Phase 3: Integration Tests

### 3.1 End-to-End Message Flow

**File**: `e2e/message_flow.test.ts`

```typescript
describe('Complete Message Flow', () => {
  test('event → metrics → client display', async () => {
    // 1. Start backend
    const backend = await startTestBackend();
    
    // 2. Connect frontend client
    const client = new WebSocketClient('ws://localhost:8080/ws');
    await client.connect();
    
    // 3. Subscribe to target
    client.subscribe('test-target', false);
    
    // 4. Backend generates events
    // (events happen automatically)
    
    // 5. Wait for MetricsUpdate
    const updates = await waitForMetricsUpdates(client, 1);
    
    expect(updates.length).toBeGreaterThan(0);
    expect(updates[0].key).toBeDefined();
    expect(updates[0].avgLatency).toBeGreaterThan(0);
  });
});
```

**Expected**: Full pipeline works end-to-end

---

### 3.2 Multi-Client Test

```typescript
test('multiple clients receive same metrics', async () => {
  const client1 = new WebSocketClient('ws://localhost:8080/ws');
  const client2 = new WebSocketClient('ws://localhost:8080/ws');
  
  await client1.connect();
  await client2.connect();
  
  client1.subscribe('target1', false);
  client2.subscribe('target1', false);
  
  // Wait for updates
  const updates1 = await waitForMetricsUpdates(client1, 5);
  const updates2 = await waitForMetricsUpdates(client2, 5);
  
  // Both should receive identical metrics
  expect(updates1[0].avgLatency).toBe(updates2[0].avgLatency);
});
```

**Expected**: All clients see consistent metrics

---

### 3.3 Split vs Combined Mode Test

```typescript
test('split and combined clients receive different data', async () => {
  const splitClient = new WebSocketClient('ws://localhost:8080/ws');
  const combinedClient = new WebSocketClient('ws://localhost:8080/ws');
  
  await splitClient.connect();
  await combinedClient.connect();
  
  splitClient.subscribe('target1', true);      // Split
  combinedClient.subscribe('target1', false);  // Combined
  
  const splitUpdates = await waitForMetricsUpdates(splitClient, 5);
  const combinedUpdates = await waitForMetricsUpdates(combinedClient, 5);
  
  // Split client should receive multiple "api" rows with different metadata
  const apiRowsSplit = splitUpdates.filter(u => u.key === 'api');
  expect(apiRowsSplit.length).toBeGreaterThan(1);
  expect(apiRowsSplit[0].metadata).not.toEqual({});
  
  // Combined client should receive single "api" row with empty metadata
  const apiRowsCombined = combinedUpdates.filter(u => u.key === 'api');
  expect(apiRowsCombined).toHaveLength(1);
  expect(apiRowsCombined[0].metadata).toEqual({});
});
```

**Expected**:
- ✅ Split client gets separate rows per metadata
- ✅ Combined client gets single aggregated row
- ✅ Both modes work simultaneously

---

### 3.4 Timeout Persistence Test

```typescript
test('metrics persist after disconnect within timeout', async () => {
  const client = new WebSocketClient('ws://localhost:8080/ws');
  await client.connect();
  client.subscribe('target1', false);
  
  // Wait for some metrics
  const initialUpdates = await waitForMetricsUpdates(client, 5);
  const initialCount = initialUpdates[0].count;
  
  // Disconnect
  client.disconnect();
  
  // Wait 2 seconds (less than 5min timeout)
  await new Promise(resolve => setTimeout(resolve, 2000));
  
  // Reconnect
  await client.connect();
  client.subscribe('target1', false);
  
  // Get metrics again
  const laterUpdates = await waitForMetricsUpdates(client, 1);
  
  // Count should be higher (metrics continued during disconnect)
  expect(laterUpdates[0].count).toBeGreaterThan(initialCount);
});
```

**Expected**: Metrics preserved and continue accumulating

---

## Phase 4: Manual UI Testing Checklist

### Visual Behavior Expectations

#### Metrics Table Display

- [ ] **Table renders** with all columns visible
- [ ] **Rows appear** for each key (or key+metadata in split mode)
- [ ] **Numbers format** correctly (2 decimals for ms, throughput as KiB/s)
- [ ] **Rows flash** briefly (blue pulse, 500ms) when updated
- [ ] **Sorted column** has bold header with arrow indicator
- [ ] **Color coding** applied: green < yellow < red based on threshold

#### Split/Combined Toggle

- [ ] **Toggle switch** visible and labeled clearly
- [ ] **Clicking toggle** sends subscription message (check network tab)
- [ ] **Split mode**: Multiple rows per key, metadata column visible
- [ ] **Combined mode**: Single row per key, metadata column hidden
- [ ] **Switching modes** smooth, no data loss

#### Expandable Metadata Rows

- [ ] **Expand arrow** (►) visible on each row in split mode
- [ ] **Clicking arrow** expands row to show full metadata
- [ ] **Expanded view** shows key-value pairs formatted nicely
- [ ] **Clicking again** collapses row
- [ ] **Multiple rows** can be expanded simultaneously

#### Sorting

- [ ] **Clicking column header** sorts ascending
- [ ] **Clicking again** toggles to descending
- [ ] **Arrow indicator** shows sort direction (↑ or ↓)
- [ ] **Rows animate** smoothly to new positions (300ms ease-out)
- [ ] **Flash animation** still works during sort

#### Multi-Target Display

- [ ] **Multiple targets** display side-by-side
- [ ] **Each target** has own split toggle
- [ ] **Each target** can sort independently
- [ ] **Target panels** resizable or fixed width
- [ ] **Linked mode** aligns keys across targets

#### Connection Status

- [ ] **Green dot** when connected
- [ ] **Red dot** when disconnected
- [ ] **Reconnecting indicator** during reconnect attempts
- [ ] **Auto-reconnect** works after disconnect

#### Performance

- [ ] **UI responsive** with 100+ rows
- [ ] **No lag** during rapid updates (10+ events/sec)
- [ ] **Smooth animations** even under load
- [ ] **Memory stable** over 10+ minutes

---

## Success Criteria Summary

### Backend

✅ **Protobuf**: All messages serialize/deserialize correctly  
✅ **Generator**: Metadata affects latency and payload as expected  
✅ **Calculator**: Metrics computed correctly for split and combined  
✅ **P90**: Percentile calculation matches expected values  
✅ **Timeout**: Monitors persist and clean up correctly  
✅ **Concurrency**: No race conditions under load  

### Frontend

✅ **Decoding**: Protobuf messages decode correctly  
✅ **Display**: All metrics render in table  
✅ **Split toggle**: Switches between modes correctly  
✅ **Expandable rows**: Metadata shown on expand  
✅ **Sorting**: Columns sort ascending/descending  
✅ **Colors**: Threshold-based coloring works  
✅ **Animations**: Flash and sort animations smooth  

### Integration

✅ **End-to-end**: Event → Calculator → Client works  
✅ **Multi-client**: Clients receive consistent metrics  
✅ **Split/Combined**: Different modes work simultaneously  
✅ **Persistence**: Metrics preserved during timeout  
✅ **Performance**: Handles 10+ clients, 100+ keys  
✅ **Stability**: No memory leaks over 24hr  

---

## Running Tests

### Backend Tests

```bash
cd backend
go test ./... -v -race -cover
```

### Frontend Tests

```bash
cd frontend
npm test
npm run test:coverage
```

### Integration Tests

```bash
# Start backend
cd backend && go run main.go &

# Run E2E tests
cd e2e
npm run test:e2e
```

### Manual Testing

```bash
# Terminal 1: Backend
cd backend && go run main.go

# Terminal 2: Frontend
cd frontend && npm run dev

# Open browser to http://localhost:5173
# Follow manual testing checklist above
```

---

## Test Coverage Goals

- **Backend**: >80% code coverage
- **Frontend**: >70% code coverage  
- **Critical paths**: 100% coverage (metrics calculation, protobuf handling)
- **UI components**: All major components tested
- **Integration**: All user workflows tested

**Testing Status**: Comprehensive test plan defined, ready for implementation! 🎯
