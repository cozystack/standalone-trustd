// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tlsconfig provides TLS configuration for the standalone trustd service.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
)

// TLSConfig provides TLS configuration for the standalone trustd service.
type TLSConfig struct {
	caCert      []byte
	caKey       []byte
	serverCert  []byte
	serverKey   []byte
	acceptedCAs []byte
}

// NewTLSConfig creates a new TLS configuration from file paths.
func NewTLSConfig(caCertPath, caKeyPath, serverCertPath, serverKeyPath, acceptedCAsPath string) (*tls.Config, error) {
	config := &TLSConfig{}

	// Load CA certificate
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}
	config.caCert = caCert

	// Load CA key
	caKey, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA key: %w", err)
	}
	config.caKey = caKey

	// Load server certificate
	serverCert, err := os.ReadFile(serverCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server certificate: %w", err)
	}
	config.serverCert = serverCert

	// Load server key
	serverKey, err := os.ReadFile(serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}
	config.serverKey = serverKey

	// Load accepted CAs
	acceptedCAs, err := os.ReadFile(acceptedCAsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read accepted CAs: %w", err)
	}
	config.acceptedCAs = acceptedCAs

	return config.createTLSConfig()
}

// createTLSConfig creates the actual TLS configuration.
func (c *TLSConfig) createTLSConfig() (*tls.Config, error) {
	// Parse server certificate and key
	cert, err := tls.X509KeyPair(c.serverCert, c.serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server certificate and key: %w", err)
	}

	// Parse accepted CAs
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(c.acceptedCAs) {
		return nil, fmt.Errorf("failed to parse accepted CAs")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	return tlsConfig, nil
}

// GetCA returns the CA certificate.
func (c *TLSConfig) GetCA() []byte {
	return c.caCert
}

// GetCAKey returns the CA private key.
func (c *TLSConfig) GetCAKey() []byte {
	return c.caKey
}

// GetAcceptedCAs returns the accepted CA certificates.
func (c *TLSConfig) GetAcceptedCAs() []byte {
	return c.acceptedCAs
}

// LoadFile is a helper function to load a file.
func LoadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}
