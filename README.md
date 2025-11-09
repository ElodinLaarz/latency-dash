# latency-dash
A real-time latency dashboard that displays latency metrics for keyed message updates.

## Features

- Real-time WebSocket updates from server
- Displays latency statistics per key:
  - Min latency
  - Max latency
  - Average latency
  - P90 (90th percentile) latency
  - Message count
- Sortable columns (click any column header to sort)
- Automatic re-sorting as new messages arrive
- Responsive design

## Installation

```bash
npm install
```

## Usage

Start the server:

```bash
npm start
```

Then open your browser to http://localhost:3000

The server will automatically send updates with random keys at random intervals, and the dashboard will display the latency metrics in real-time.

## How it works

1. The server sends messages with the same key at random intervals
2. The client calculates the time between messages with the same key (latency)
3. Statistics are computed and displayed for each key
4. The table can be sorted by any column and maintains sort order as new data arrives
