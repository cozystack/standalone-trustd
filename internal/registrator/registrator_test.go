// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registrator_test

import (
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/peer"

	"github.com/cozystack/standalone-trustd/internal/registrator"
	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
)

func TestCertificate(t *testing.T) {
	// Create temporary directory for test certificates
	tempDir := t.TempDir()

	// Generate CA certificate and key
	ca, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("test-ca"),
		x509.NotAfter(time.Now().Add(time.Hour)),
	)
	require.NoError(t, err)

	// Write CA certificate and key to files
	caCertPath := filepath.Join(tempDir, "ca.crt")
	caKeyPath := filepath.Join(tempDir, "ca.key")
	acceptedCAsPath := filepath.Join(tempDir, "accepted-cas.crt")

	require.NoError(t, os.WriteFile(caCertPath, ca.CrtPEM, 0644))
	require.NoError(t, os.WriteFile(caKeyPath, ca.KeyPEM, 0644))
	require.NoError(t, os.WriteFile(acceptedCAsPath, ca.CrtPEM, 0644))

	// Create registrator
	reg := &registrator.Registrator{
		CACert:      caCertPath,
		CAKey:       caKeyPath,
		AcceptedCAs: acceptedCAsPath,
		AuthToken:   "test-token",
	}

	// Create test context with peer
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   netip.MustParseAddr("127.0.0.1").AsSlice(),
			Port: 30000,
		},
	})

	// Generate CSR
	serverCSR, _, err := x509.NewEd25519CSRAndIdentity(
		x509.IPAddresses([]net.IP{netip.MustParseAddr("10.5.0.4").AsSlice()}),
		x509.DNSNames([]string{"test-server"}),
		x509.CommonName("test-server"),
	)
	require.NoError(t, err)

	// Test certificate signing
	resp, err := reg.Certificate(ctx, &securityapi.CertificateRequest{
		Csr: serverCSR.X509CertificateRequestPEM,
	})
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, ca.CrtPEM, resp.Ca)
	assert.NotEmpty(t, resp.Crt)

	// Parse and verify the signed certificate
	block, _ := pem.Decode(resp.Crt)
	require.NotNil(t, block)

	cert, err := stdx509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, stdx509.KeyUsageDigitalSignature, cert.KeyUsage)
	assert.Equal(t, []stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}, cert.ExtKeyUsage)
	assert.Equal(t, "test-server", cert.Subject.CommonName)
}

func TestCertificateWithClientAuthOrganization(t *testing.T) {
	// Create temporary directory for test certificates
	tempDir := t.TempDir()

	// Generate CA certificate and key
	ca, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("test-ca"),
		x509.NotAfter(time.Now().Add(time.Hour)),
	)
	require.NoError(t, err)

	// Write CA certificate and key to files
	caCertPath := filepath.Join(tempDir, "ca.crt")
	caKeyPath := filepath.Join(tempDir, "ca.key")
	acceptedCAsPath := filepath.Join(tempDir, "accepted-cas.crt")

	require.NoError(t, os.WriteFile(caCertPath, ca.CrtPEM, 0644))
	require.NoError(t, os.WriteFile(caKeyPath, ca.KeyPEM, 0644))
	require.NoError(t, os.WriteFile(acceptedCAsPath, ca.CrtPEM, 0644))

	// Create registrator
	reg := &registrator.Registrator{
		CACert:      caCertPath,
		CAKey:       caKeyPath,
		AcceptedCAs: acceptedCAsPath,
		AuthToken:   "test-token",
	}

	// Create test context with peer
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{
			IP:   netip.MustParseAddr("127.0.0.1").AsSlice(),
			Port: 30000,
		},
	})

	// Generate CSR with client auth organization (should be stripped)
	serverCSR, _, err := x509.NewEd25519CSRAndIdentity(
		x509.IPAddresses([]net.IP{netip.MustParseAddr("10.5.0.4").AsSlice()}),
		x509.DNSNames([]string{"test-server"}),
		x509.CommonName("test-server"),
		x509.Organization("client-auth"), // This should be stripped
	)
	require.NoError(t, err)

	// Test certificate signing
	resp, err := reg.Certificate(ctx, &securityapi.CertificateRequest{
		Csr: serverCSR.X509CertificateRequestPEM,
	})
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, ca.CrtPEM, resp.Ca)
	assert.NotEmpty(t, resp.Crt)

	// Parse and verify the signed certificate
	block, _ := pem.Decode(resp.Crt)
	require.NotNil(t, block)

	cert, err := stdx509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify that organization was stripped
	assert.Equal(t, []string(nil), cert.Subject.Organization)
	assert.Equal(t, "test-server", cert.Subject.CommonName)
}
