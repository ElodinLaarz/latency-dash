# Latency Dashboard

A real-time latency monitoring dashboard that tracks time intervals between keyed events with WebSocket-based live updates.

![Status](https://img.shields.io/badge/status-design%20phase-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![React](https://img.shields.io/badge/React-18+-61DAFB?logo=react)
![TypeScript](https://img.shields.io/badge/TypeScript-5+-3178C6?logo=typescript)

## Overview

This system monitors and visualizes latency metrics for keyed events in real-time. It displays min, max, average, and P90 latency measurements, with dynamic sorting and visual feedback as new data arrives.

**Key Features:**
- 📊 Real-time latency tracking per key
- 📈 Statistical metrics (min, max, avg, p90)
- 🔄 Dynamic sortable table
- 🎨 Visual indicators for recent updates
- 🔌 WebSocket-based live updates
- 🔁 Auto-reconnection on disconnect

## Quick Start

### Development

**Backend** (Terminal 1):
```bash
cd backend
go mod download
go run main.go
```

**Frontend** (Terminal 2):
```bash
cd frontend
npm install
npm run dev
```

Open http://localhost:5173 in your browser.

### Production

```bash
./build.sh
./latency-dash
```

Open http://localhost:8080 in your browser.

## Architecture

```
┌─────────────────┐         WebSocket          ┌─────────────────┐
│   React SPA     │◄───────────────────────────►│   Go Server     │
│   + TypeScript  │                             │   + WebSocket   │
│   + TanStack    │                             │   + Generator   │
└─────────────────┘                             └─────────────────┘
        │                                                │
        ▼                                                ▼
  Browser State                                  Event Generator
  - Per-key metrics                              - Random intervals
  - Latency history                              - Multiple keys
  - Sorting state                                - Timestamp events
```

**Data Flow:**
1. Backend generates random keyed events with timestamps
2. Events broadcast via WebSocket to all connected clients
3. Frontend calculates latency between consecutive events per key
4. UI updates metrics and re-sorts table in real-time

## Documentation

- **[DESIGN.md](DESIGN.md)** - Comprehensive architecture, data structures, and design decisions
- **[IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md)** - Step-by-step implementation instructions
- **[QUICK_START.md](QUICK_START.md)** - Fast reference guide and common commands

## Technology Stack

### Backend (Go)
- **Go 1.21+** - Core language
- **gorilla/websocket** - WebSocket implementation
- **Standard library** - HTTP server

**Why Go?**
- Excellent concurrency with goroutines
- High performance for real-time broadcasting
- Simple deployment (single binary)
- Strong standard library

### Frontend (React + TypeScript)
- **React 18+** - UI framework
- **TypeScript 5+** - Type safety
- **Vite** - Build tool with fast HMR
- **TailwindCSS** - Styling
- **TanStack Table** - Advanced table with sorting
- **Lucide React** - Icons

**Why React + TypeScript?**
- Type-safe data structures for complex metrics
- Efficient re-rendering for real-time updates
- Rich ecosystem and tooling
- Modern development experience

## Project Structure

```
latency-dash/
├── backend/                     # Go backend server
│   ├── main.go                 # Entry point
│   ├── server/
│   │   ├── hub.go              # WebSocket connection hub
│   │   ├── client.go           # Individual client handler
│   │   └── websocket.go        # WebSocket upgrade handler
│   └── generator/
│       └── events.go           # Event generator
│
├── frontend/                    # React TypeScript frontend
│   ├── src/
│   │   ├── App.tsx             # Main application component
│   │   ├── hooks/
│   │   │   └── useWebSocket.ts # WebSocket connection hook
│   │   ├── components/
│   │   │   ├── LatencyTable.tsx      # Main metrics table
│   │   │   └── ConnectionStatus.tsx  # Connection indicator
│   │   ├── types/
│   │   │   └── index.ts        # TypeScript interfaces
│   │   └── utils/
│   │       └── metrics.ts      # Metric calculation functions
│   └── package.json
│
├── DESIGN.md                    # Architecture documentation
├── IMPLEMENTATION_GUIDE.md      # Build instructions
├── QUICK_START.md              # Quick reference
└── README.md                    # This file
```

## Configuration

### Backend Environment Variables

```bash
PORT=8080              # HTTP server port (default: 8080)
NUM_KEYS=15           # Number of unique keys (default: 15)
MIN_INTERVAL=100ms    # Min time between events (default: 100ms)
MAX_INTERVAL=5s       # Max time between events (default: 5s)
```

### Frontend Environment Variables

Create `frontend/.env`:
```env
VITE_WS_URL=ws://localhost:8080/ws
```

## WebSocket Protocol

### Event Message (Server → Client)
```json
{
  "type": "event",
  "data": {
    "key": "service-A",
    "timestamp": 1699534860123
  }
}
```

### Init Message (Server → Client)
```json
{
  "type": "init",
  "data": {
    "message": "Connected to latency monitor",
    "serverTime": 1699534860123
  }
}
```

## Features

### Current Features
- ✅ Real-time event streaming via WebSocket
- ✅ Per-key latency tracking (min, max, avg, p90)
- ✅ Sortable table by any column
- ✅ Visual highlighting for recent updates (2s fade)
- ✅ Connection status indicator
- ✅ Auto-reconnection with exponential backoff
- ✅ Configurable event generation

### Planned Enhancements (Phase 4+)
- 📊 Time-series graphs per key
- 💾 Persistent storage (SQLite/PostgreSQL)
- 🔍 Key filtering and search
- 🚨 Threshold-based alerting
- 📤 Export to CSV/JSON
- 🐳 Docker containerization
- 📈 Additional percentiles (P50, P95, P99)

## Development

### Prerequisites
- Go 1.21 or later
- Node.js 18 or later
- npm or pnpm

### Setup

1. **Clone repository**
   ```bash
   git clone <repository-url>
   cd latency-dash
   ```

2. **Backend setup**
   ```bash
   cd backend
   go mod download
   ```

3. **Frontend setup**
   ```bash
   cd frontend
   npm install
   ```

### Running Tests

**Backend:**
```bash
cd backend
go test ./...
```

**Frontend:**
```bash
cd frontend
npm run test
```

### Building

**Development build:**
```bash
# Backend
cd backend && go build

# Frontend
cd frontend && npm run build
```

**Production build:**
```bash
./build.sh
```

This creates a single `latency-dash` binary that serves both backend and frontend.

## Usage

### Starting the Server
```bash
./latency-dash
# or
cd backend && go run main.go
```

Server starts on http://localhost:8080

### Connecting to WebSocket
The frontend automatically connects to `/ws` endpoint. You can also test manually:

```javascript
// Browser console
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

### UI Features

**Sorting:**
- Click any column header to sort
- Click again to reverse sort order
- Arrow indicator shows current sort

**Visual Feedback:**
- Recently updated rows highlight in blue
- Highlight fades after 2 seconds
- Connection indicator shows green (connected) or red (disconnected)

**Metrics:**
- **Min**: Lowest latency observed for this key
- **Max**: Highest latency observed for this key
- **Avg**: Mean of all latency measurements
- **P90**: 90th percentile latency (useful for SLO monitoring)
- **Count**: Number of latency measurements recorded

## Performance

### Benchmarks (Expected)
- Support 10+ concurrent clients
- Handle 100+ unique keys efficiently
- Sub-100ms latency for message delivery
- Stable memory usage over 24hr operation
- Smooth UI at 10+ updates/second

### Optimization Tips
- Frontend history limited to 1000 measurements per key
- Backend uses buffered channels for broadcasting
- React table uses memoization for rows
- Consider virtual scrolling for 100+ keys

## Troubleshooting

### WebSocket Connection Issues
**Symptom**: Red connection indicator, no updates
**Solutions**:
- Verify backend is running on port 8080
- Check browser console for errors
- Verify WebSocket URL in frontend config
- Check firewall settings

### Metrics Not Updating
**Symptom**: Table shows but doesn't update
**Solutions**:
- Check browser console for message parsing errors
- Verify WebSocket messages have correct format
- Ensure `handleMessage` callback is wired up in App.tsx

### Sort Not Working
**Symptom**: Clicking headers doesn't sort
**Solutions**:
- Verify TanStack Table sorting state is configured
- Check column definitions have `accessorKey`
- Review browser console for React errors

### High Memory Usage
**Symptom**: Browser/server consuming excessive memory
**Solutions**:
- Reduce `MAX_HISTORY` in metrics calculation (frontend)
- Limit number of active keys (backend config)
- Check for memory leaks with browser profiler

## Contributing

See [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md) for detailed development instructions.

### Development Workflow
1. Create feature branch
2. Follow existing code style
3. Add tests for new features
4. Update documentation
5. Submit pull request

## License

MIT License - see LICENSE file for details

## Support

For questions or issues:
- Review documentation in `DESIGN.md` and `IMPLEMENTATION_GUIDE.md`
- Check [Troubleshooting](#troubleshooting) section
- Open an issue on GitHub

## Acknowledgments

- Built with Go and React
- WebSocket implementation using gorilla/websocket
- Table sorting powered by TanStack Table
- UI styled with TailwindCSS

---

**Ready to get started?** See [QUICK_START.md](QUICK_START.md) for fast setup or [IMPLEMENTATION_GUIDE.md](IMPLEMENTATION_GUIDE.md) for detailed build instructions.
