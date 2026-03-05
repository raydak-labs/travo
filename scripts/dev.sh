#!/usr/bin/env bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting openwrt-travel-gui development servers...${NC}"

# Trap to kill background processes on exit
cleanup() {
  echo -e "\n${YELLOW}Shutting down dev servers...${NC}"
  kill 0 2>/dev/null
  exit 0
}
trap cleanup SIGINT SIGTERM EXIT

# Start backend (Go with auto-reload if air is installed, otherwise plain go run)
echo -e "${GREEN}Starting backend on :3000...${NC}"
if command -v air &> /dev/null; then
  (cd backend && air) &
else
  (cd backend && go run ./cmd/server) &
fi

# Start frontend (Vite dev server)
echo -e "${GREEN}Starting frontend on :5173...${NC}"
(cd frontend && pnpm dev) &

# Wait for all background processes
wait
