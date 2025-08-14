#!/bin/bash

set -e

echo "Building g_multithreaded_microservice..."

cd g_multithreaded_microservice

if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

echo "Downloading dependencies..."
go mod download

echo "Building service..."
go build -o mt-service cmd/service/main.go

echo "Checking for Redis..."
if ! nc -z localhost 6379 2>/dev/null; then
    echo "Redis not running. Starting Redis in Docker..."
    docker run -d --name mt-redis -p 6379:6379 redis:7-alpine
    sleep 2
fi

echo "Creating output directory..."
mkdir -p ../data/g_output

echo "Starting multithreaded microservice..."
./mt-service \
    -mode=all \
    -redis=localhost:6379 \
    -input=../data/input \
    -output=../data/g_output \
    -workers=10 \
    -kernel=15

echo "Processing complete! Check ../data/g_output for results."