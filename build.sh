#!/bin/bash

set -e

echo "Building DN42 Globalping..."

# Build backend
echo "Building backend..."
cd backend
go mod download
go build -o server main.go
cd ..

# Build probe
echo "Building probe..."
cd probe
go mod download
go build -o probe main.go
cd ..

# Prepare frontend
echo "Preparing frontend..."
mkdir -p frontend/dist
cp frontend/index.html frontend/dist/

echo "Build complete!"
echo ""
echo "To run the server:"
echo "  cd backend && ./server"
echo ""
echo "To run a probe:"
echo "  cd probe && ./probe -server=ws://localhost:8080/ws/probe -name=MyProbe -location='My Location' -lat=0 -lng=0"
