#!/bin/bash

set -e

# Run backend in background
echo "Starting backend server..."
cd backend
go run main.go &
BACKEND_PID=$!
cd ..

# Wait for server to start
sleep 2

# Start sample probes
echo "Starting sample probes..."
cd probe

echo "Starting US-East probe..."
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="US-East-1" \
  -location="Virginia, USA" \
  -lat=37.5407 \
  -lng=-77.4360 &

echo "Starting EU-West probe..."
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="EU-West-1" \
  -location="London, UK" \
  -lat=51.5074 \
  -lng=-0.1278 &

echo "Starting Asia probe..."
go run main.go -server=ws://localhost:8080/ws/probe \
  -name="Asia-East-1" \
  -location="Tokyo, Japan" \
  -lat=35.6762 \
  -lng=139.6503 &

cd ..

echo ""
echo "===================================="
echo "DN42 Globalping is now running!"
echo "Open http://localhost:8080 in your browser"
echo "Press Ctrl+C to stop all services"
echo "===================================="
echo ""

# Wait for Ctrl+C
trap "kill $BACKEND_PID; killall main; exit" INT
wait
