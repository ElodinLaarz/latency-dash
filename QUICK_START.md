# Quick Start Guide - Latency Dashboard

## Project Overview
Real-time latency monitoring dashboard that tracks time intervals between keyed updates. Features sortable metrics (min, max, avg, p90) and WebSocket-based live updates.

## Tech Stack
- **Backend**: Go 1.21+ with gorilla/websocket
- **Frontend**: React + TypeScript + Vite + TailwindCSS + TanStack Table
- **Protocol**: WebSocket for real-time communication

## Quick Commands

### Development Mode

**Terminal 1 - Backend**:
```bash
cd backend
go mod download
go run main.go
# Server runs on http://localhost:8080
```

**Terminal 2 - Frontend**:
```bash
cd frontend
npm install
npm run dev
# Dev server runs on http://localhost:5173
```

### Production Build

```bash
# Build frontend
cd frontend
npm run build
cd ..

# Build backend (serves frontend from dist/)
cd backend
go build -o ../latency-dash
cd ..

# Run
./latency-dash
# Open http://localhost:8080
```

## Project Structure

```
latency-dash/
├── DESIGN.md                    # Comprehensive architecture doc
├── IMPLEMENTATION_GUIDE.md      # Step-by-step build instructions
├── QUICK_START.md              # This file
│
├── backend/                     # Go backend
│   ├── main.go                 # Entry point
│   ├── server/
│   │   ├── hub.go              # WebSocket hub
│   │   ├── client.go           # Client handler
│   │   └── websocket.go        # WS upgrade handler
│   └── generator/
│       └── events.go           # Event generator
│
└── frontend/                    # React frontend
    ├── src/
    │   ├── App.tsx             # Main component
    │   ├── hooks/
    │   │   └── useWebSocket.ts # WebSocket hook
    │   ├── components/
    │   │   ├── LatencyTable.tsx
    │   │   └── ConnectionStatus.tsx
    │   ├── types/
    │   │   └── index.ts        # TypeScript types
    │   └── utils/
    │       └── metrics.ts      # Calculation functions
    └── package.json
```

## Key Features

✅ Real-time event streaming via WebSocket  
✅ Per-key latency tracking (min, max, avg, p90)  
✅ Sortable table (click any column header)  
✅ Visual highlighting for recent updates  
✅ Auto-reconnection on disconnect  
✅ Configurable event generation  

## Configuration

### Backend Environment Variables
```bash
PORT=8080              # Server port
NUM_KEYS=15           # Number of unique keys
MIN_INTERVAL=100ms    # Min time between events
MAX_INTERVAL=5s       # Max time between events
```

### Frontend Environment Variables
Create `frontend/.env`:
```env
VITE_WS_URL=ws://localhost:8080/ws
```

## Message Protocol

### Server → Client (Event)
```json
{
  "type": "event",
  "data": {
    "key": "service-A",
    "timestamp": 1699534860123
  }
}
```

### Server → Client (Init)
```json
{
  "type": "init",
  "data": {
    "message": "Connected to latency monitor",
    "serverTime": 1699534860123
  }
}
```

## Testing WebSocket Manually

### Using Browser Console
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => console.log('Connected');
ws.onmessage = (e) => console.log('Received:', JSON.parse(e.data));
ws.onerror = (e) => console.error('Error:', e);
```

### Using curl/websocat
```bash
# Install websocat: cargo install websocat
websocat ws://localhost:8080/ws
```

## Implementation Order

1. ✅ **Planning Phase** (You are here)
   - Review DESIGN.md
   - Review IMPLEMENTATION_GUIDE.md

2. **Backend Foundation**
   - Set up Go project structure
   - Implement WebSocket hub
   - Implement client handler
   - Create event generator
   - Test with manual WebSocket client

3. **Frontend Foundation**
   - Set up React + TypeScript project
   - Create WebSocket hook
   - Implement metrics calculation
   - Build table component
   - Add connection status indicator

4. **Integration**
   - Connect frontend to backend
   - Test end-to-end flow
   - Add error handling
   - Polish UI/UX

5. **Enhancement** (Optional)
   - Add persistence (SQLite)
   - Add charts/graphs
   - Add filtering/search
   - Add alerting
   - Dockerize

## Troubleshooting

**WebSocket won't connect**
- Check backend is running on port 8080
- Verify CORS settings in upgrader
- Check browser console for errors

**Metrics not updating**
- Verify event messages have correct format
- Check browser console for JSON parse errors
- Ensure `handleMessage` callback is wired up

**Sorting not working**
- Verify TanStack Table sorting state
- Check column definitions have correct `accessorKey`

**Build fails**
- Backend: Run `go mod tidy`
- Frontend: Delete `node_modules` and `npm install`
- Check Go version (1.21+) and Node version (18+)

## Next Steps

1. Read **DESIGN.md** for full architectural understanding
2. Follow **IMPLEMENTATION_GUIDE.md** for detailed build steps
3. Start with backend (Phase 1)
4. Build frontend (Phase 2)
5. Integrate and polish (Phase 3)

## Resources

- **Design Doc**: `DESIGN.md` - Complete architecture and data flow
- **Implementation**: `IMPLEMENTATION_GUIDE.md` - Step-by-step instructions
- **Go Docs**: https://golang.org/doc/
- **React Docs**: https://react.dev/
- **TanStack Table**: https://tanstack.com/table/
- **Gorilla WebSocket**: https://github.com/gorilla/websocket

---

**Ready to build?** Start with Phase 1 in IMPLEMENTATION_GUIDE.md!
