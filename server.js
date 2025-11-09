const express = require('express');
const WebSocket = require('ws');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;

// Serve static files from the public directory
app.use(express.static(path.join(__dirname, 'public')));

// Start HTTP server
const server = app.listen(PORT, () => {
  console.log(`Server running on http://localhost:${PORT}`);
});

// Create WebSocket server
const wss = new WebSocket.Server({ server });

// List of keys to randomly choose from
const KEYS = ['api-endpoint-1', 'api-endpoint-2', 'api-endpoint-3', 'database-query', 'cache-lookup'];

// Function to get random delay between 100ms and 5000ms
function getRandomDelay() {
  return Math.floor(Math.random() * 4900) + 100;
}

// Function to get random key
function getRandomKey() {
  return KEYS[Math.floor(Math.random() * KEYS.length)];
}

// Handle WebSocket connections
wss.on('connection', (ws) => {
  console.log('Client connected');
  
  // Send initial message
  ws.send(JSON.stringify({ type: 'connected', message: 'Connected to latency dashboard server' }));
  
  // Function to send updates
  function sendUpdate() {
    if (ws.readyState === WebSocket.OPEN) {
      const key = getRandomKey();
      const timestamp = Date.now();
      
      ws.send(JSON.stringify({
        type: 'update',
        key: key,
        timestamp: timestamp
      }));
      
      // Schedule next update with random delay
      const delay = getRandomDelay();
      setTimeout(sendUpdate, delay);
    }
  }
  
  // Start sending updates
  sendUpdate();
  
  ws.on('close', () => {
    console.log('Client disconnected');
  });
  
  ws.on('error', (error) => {
    console.error('WebSocket error:', error);
  });
});

console.log('WebSocket server ready');
