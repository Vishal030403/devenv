#!/bin/bash

set -e

echo "====================================="
echo " Building DevEnv CLI"
echo "====================================="

mkdir -p dist

echo "[INFO] Building current platform binary..."
go build -o dist/devenv

echo "[INFO] Building macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -o dist/devenv-mac-arm64

echo "[INFO] Building macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build -o dist/devenv-mac-amd64

echo "[INFO] Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -o dist/devenv-linux-amd64

echo "[INFO] Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -o dist/devenv-linux-arm64

echo "[INFO] Building Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -o dist/devenv-windows-amd64.exe

echo "[SUCCESS] All binaries built successfully."
