#!/bin/bash

# Exit on error
set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to kill processes on exit
cleanup() {
    echo -e "\n${YELLOW}Shutting down servers...${NC}"
    # Kill the process group to ensure all child processes are terminated
    kill -9 $BACKEND_PID $FRONTEND_PID 2>/dev/null || true
        # Kill any remaining processes on our ports
    kill_port 3000
    kill_port 8080
    echo -e "${GREEN}Servers stopped.${NC}"
    exit 0
}

# Function to check if a port is in use
port_in_use() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -i :"$port" >/dev/null 2>&1
    elif command -v fuser >/dev/null 2>&1; then
        fuser "$port/tcp" >/dev/null 2>&1
    else
        # Fallback to checking if netstat is available
        if command -v netstat >/dev/null 2>&1; then
            netstat -tuln | grep -q ":$port "
        else
            echo "Neither lsof, fuser, nor netstat is available. Cannot check port $port."
            return 1
        fi
    fi
}

# Function to kill process on a port
kill_port() {
    local port=$1
    if command -v lsof >/dev/null 2>&1; then
        lsof -ti :"$port" | xargs kill -9 2>/dev/null || true
    elif command -v fuser >/dev/null 2>&1; then
        fuser -k "$port/tcp" 2>/dev/null || true
    elif command -v netstat >/dev/null 2>&1; then
        # This is a more complex fallback that might require sudo
        netstat -tuln | grep ":$port " | awk '{print $7}' | cut -d'/' -f1 | xargs kill -9 2>/dev/null || true
    else
        echo "Warning: Could not kill process on port $port - no suitable tool found"
    fi
}

# Trap SIGINT (Ctrl+C) and call cleanup
trap cleanup INT TERM

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

# Check if ports are available
if port_in_use 3000; then
    echo -e "${RED}Error: Port 3000 is already in use (frontend).${NC}"
    exit 1
fi

if port_in_use 8080; then
    echo -e "${YELLOW}Port 8080 is in use. Attempting to free it...${NC}"
    kill_port 8080
    sleep 1
    
    if port_in_use 8080; then
        echo -e "${RED}Failed to free port 8080. Please close the application using this port and try again.${NC}"
        echo -e "${YELLOW}You can try running:${NC}"
        echo "  sudo lsof -i :8080"
        echo "  # Then manually kill the process with: kill -9 <PID>"
        exit 1
    fi
fi

# Start the backend server
echo -e "${GREEN}Starting backend server...${NC}"
cd "$ROOT_DIR/backend"
# Build the backend first to catch any compilation errors
echo -e "${YELLOW}Building backend...${NC}"
if ! go build -o /tmp/backend ./cmd/server; then
    echo -e "${RED}Failed to build backend. Please check for compilation errors.${NC}"
    exit 1
fi

# Start the backend in the background
/tmp/backend &
BACKEND_PID=$!

# Give the backend a moment to start up
echo -e "${YELLOW}Waiting for backend to initialize...${NC}"
for i in {1..10}; do
    if curl -s http://localhost:8080 >/dev/null; then
        echo -e "${GREEN}Backend is up and running!${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}Backend failed to start. Check for errors above.${NC}"
        cleanup
        exit 1
    fi
    echo -n "."
    sleep 1
done

# Start the frontend server
echo -e "\n${GREEN}Starting frontend server...${NC}"
cd "$ROOT_DIR/frontend"
# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo -e "${YELLOW}Installing frontend dependencies...${NC}"
    npm install --legacy-peer-deps
fi

# Start the frontend in the background
npm start &
FRONTEND_PID=$!

# Wait for frontend to be ready
echo -e "${YELLOW}Waiting for frontend to start...${NC}"
for i in {1..10}; do
    if curl -s http://localhost:3000 >/dev/null; then
        echo -e "${GREEN}Frontend is up and running!${NC}"
        break
    fi
    if [ $i -eq 10 ]; then
        echo -e "${RED}Frontend failed to start. Check for errors above.${NC}"
        cleanup
        exit 1
    fi
    echo -n "."
    sleep 1
done

# Open the frontend in the default browser
echo -e "\n${GREEN}Opening dashboard in default browser...${NC}"
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    open "http://localhost:3000"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux
    xdg-open "http://localhost:3000" 2>/dev/null || 
        echo -e "${YELLOW}Could not open browser automatically. Please visit http://localhost:3000${NC}"
else
    echo -e "${YELLOW}Please open your browser and navigate to http://localhost:3000${NC}"
fi

echo -e "\n${GREEN}==================================================${NC}"
echo -e "${GREEN}  Servers are running!${NC}"
echo -e "${GREEN}  Frontend: http://localhost:3000${NC}"
echo -e "${GREEN}  Backend:  http://localhost:8080${NC}"
echo -e "${GREEN}==================================================${NC}"
echo -e "\n${YELLOW}Press Ctrl+C to stop both servers${NC}"

# Keep the script running and wait for Ctrl+C
wait $BACKEND_PID $FRONTEND_PID
cleanup
