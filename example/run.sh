#!/bin/bash

# Example script to run the standalone trustd service
# This script generates the required certificates and runs the service

set -e

# Create a temporary directory for certificates
CERT_DIR=$(mktemp -d)
echo "Using certificate directory: $CERT_DIR"

# Generate CA certificate and key
echo "Generating CA certificate..."
openssl genrsa -out "$CERT_DIR/ca.key" 4096
openssl req -new -x509 -days 365 -key "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
    -subj "/CN=standalone-trustd-ca"

# Generate server certificate
echo "Generating server certificate..."
openssl genrsa -out "$CERT_DIR/server.key" 4096
openssl req -new -key "$CERT_DIR/server.key" -out "$CERT_DIR/server.csr" \
    -subj "/CN=standalone-trustd-server"
openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial -out "$CERT_DIR/server.crt" -days 365

# Copy CA cert as accepted CAs
cp "$CERT_DIR/ca.crt" "$CERT_DIR/accepted-cas.crt"

# Clean up CSR
rm "$CERT_DIR/server.csr"

echo "Certificates generated successfully!"
echo "CA Certificate: $CERT_DIR/ca.crt"
echo "CA Key: $CERT_DIR/ca.key"
echo "Server Certificate: $CERT_DIR/server.crt"
echo "Server Key: $CERT_DIR/server.key"
echo "Accepted CAs: $CERT_DIR/accepted-cas.crt"

# Build the standalone trustd service
echo "Building standalone trustd..."
go build -o standalone-trustd .

# Run the service
echo "Starting standalone trustd service..."
./standalone-trustd \
    --ca-cert="$CERT_DIR/ca.crt" \
    --ca-key="$CERT_DIR/ca.key" \
    --server-cert="$CERT_DIR/server.crt" \
    --server-key="$CERT_DIR/server.key" \
    --accepted-cas="$CERT_DIR/accepted-cas.crt" \
    --auth-token="example-auth-token" \
    --port=50001

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    rm -rf "$CERT_DIR"
    rm -f standalone-trustd
}

# Set up cleanup on exit
trap cleanup EXIT
