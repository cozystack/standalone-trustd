# Standalone Trustd Service

This is a standalone implementation of the Talos trustd service that provides certificate signing functionality without Talos dependencies. It accepts all necessary configuration through command-line options.

## Overview

The standalone trustd service provides:
- gRPC-based certificate signing service
- TLS with mutual authentication
- CSR (Certificate Signing Request) validation and signing
- Support for server certificates only (client certificates are rejected)

## Usage

### Command Line Options

```bash
./standalone-trustd \
  --ca-cert=/path/to/ca.crt \
  --ca-key=/path/to/ca.key \
  --server-cert=/path/to/server.crt \
  --server-key=/path/to/server.key \
  --accepted-cas=/path/to/accepted-cas.crt \
  --auth-token=your-auth-token \
  --port=50001
```

### Required Options

- `--ca-cert`: Path to CA certificate file (used for signing)
- `--ca-key`: Path to CA private key file (used for signing)
- `--server-cert`: Path to server certificate file (for TLS)
- `--server-key`: Path to server private key file (for TLS)
- `--accepted-cas`: Path to accepted CA certificates file (returned to clients)
- `--auth-token`: Authentication token for client connections

### Optional Options

- `--port`: Port to listen on (default: 50001)
- `--debug-port`: Debug server port (default: 9983)

## Certificate Files

### CA Certificate and Key
The CA certificate and key are used to sign client certificates. These should be the same CA that issued the server certificate.

### Server Certificate and Key
The server certificate and key are used for TLS connections. These should be issued by a trusted CA.

### Accepted CAs
The accepted CAs file contains the CA certificates that will be returned to clients in the certificate response. This is typically the same as the CA certificate used for signing.

## API

The service implements the `SecurityService` gRPC interface with the following method:

### Certificate
Signs a Certificate Signing Request (CSR) and returns a signed certificate along with the CA certificate.

**Request:**
```protobuf
message CertificateRequest {
  bytes csr = 1;  // Certificate Signing Request in PEM format
}
```

**Response:**
```protobuf
message CertificateResponse {
  bytes ca = 1;   // CA certificate in PEM format
  bytes crt = 2;  // Signed certificate in PEM format
}
```

## Security Considerations

1. **Server Certificates Only**: The service only signs server certificates and strips any client authentication capabilities from CSRs.

2. **TLS Mutual Authentication**: All connections require valid client certificates.

3. **CSR Validation**: CSRs are validated before signing to ensure they meet security requirements.

4. **Organization Stripping**: Any organization fields in CSRs are removed to prevent client authentication.

## Building

```bash
go build -o standalone-trustd ./cmd/standalone-trustd
```

## Testing

```bash
go test ./cmd/standalone-trustd/...
```

## Example Certificate Generation

To generate the required certificates for testing:

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt -subj "/CN=test-ca"

# Generate server certificate
openssl genrsa -out server.key 4096
openssl req -new -key server.key -out server.csr -subj "/CN=trustd-server"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 365

# Copy CA cert as accepted CAs
cp ca.crt accepted-cas.crt
```

## Differences from Original Trustd

1. **No Talos Dependencies**: Removed dependency on Talos resource system and state management
2. **Command Line Configuration**: All configuration through command-line options instead of Talos configuration
3. **Simplified Architecture**: No resource watching or dynamic configuration updates
4. **Standalone Operation**: Can run independently without Talos runtime
