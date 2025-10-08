// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registrator

import (
	"context"
	stdx509 "crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/siderolabs/crypto/x509"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
)

// Registrator implements the SecurityServiceServer interface.
type Registrator struct {
	securityapi.UnimplementedSecurityServiceServer

	CACert      string
	CAKey       string
	AcceptedCAs string
	AuthToken   string
}

// Register implements the gRPC service registration.
func (r *Registrator) Register(s *grpc.Server) {
	securityapi.RegisterSecurityServiceServer(s, r)
}

// Certificate implements the securityapi.SecurityServer interface.
//
// This API is called by Talos worker nodes to request a server certificate for apid running on the node.
// Control plane nodes generate certificates (client and server) directly from machine config PKI.
func (r *Registrator) Certificate(ctx context.Context, in *securityapi.CertificateRequest) (resp *securityapi.CertificateResponse, err error) {
	remotePeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "peer not found")
	}

	// Load CA certificate and key
	caCert, caKey, err := r.loadCACertificate()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load CA certificate: %v", err)
	}

	// Load accepted CAs
	acceptedCAs, err := r.loadAcceptedCAs()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load accepted CAs: %v", err)
	}

	// decode and validate CSR
	csrPemBlock, _ := pem.Decode(in.Csr)
	if csrPemBlock == nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to decode CSR")
	}

	request, err := stdx509.ParseCertificateRequest(csrPemBlock.Bytes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse CSR: %s", err)
	}

	log.Printf("received CSR from %s: subject %s dns %s ips %s", remotePeer.Addr, request.Subject, request.DNSNames, request.IPAddresses)

	// allow only server auth certificates
	x509Opts := []x509.Option{
		x509.KeyUsage(stdx509.KeyUsageDigitalSignature),
		x509.ExtKeyUsage([]stdx509.ExtKeyUsage{stdx509.ExtKeyUsageServerAuth}),
	}

	// don't allow any certificates which can be used for client authentication
	//
	// we don't return an error here, as otherwise workers running old versions of Talos
	// will fail to provision client certificate and will never launch apid
	//
	// instead, the returned certificate will be rejected when being used
	if len(request.Subject.Organization) > 0 {
		log.Printf("removing client auth organization from CSR: %s", request.Subject.Organization)

		x509Opts = append(x509Opts, x509.OverrideSubject(func(subject *pkix.Name) {
			subject.Organization = nil
		}))
	}

	// TODO: Verify that the request is coming from the IP address declared in
	// the CSR.
	signed, err := x509.NewCertificateFromCSRBytes(
		caCert,
		caKey,
		in.Csr,
		x509Opts...,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to sign CSR: %s", err)
	}

	resp = &securityapi.CertificateResponse{
		Ca:  acceptedCAs,
		Crt: signed.X509CertificatePEM,
	}

	// Log successful certificate issuance without dumping full certificate
	log.Printf("issued certificate for %s to %s: notBefore=%s notAfter=%s sanDNS=%v sanIP=%v",
		signed.X509Certificate.Subject, remotePeer.Addr,
		signed.X509Certificate.NotBefore.UTC().Format(time.RFC3339),
		signed.X509Certificate.NotAfter.UTC().Format(time.RFC3339),
		signed.X509Certificate.DNSNames,
		signed.X509Certificate.IPAddresses,
	)

	return resp, nil
}

// loadCACertificate loads the CA certificate and key from files.
func (r *Registrator) loadCACertificate() ([]byte, []byte, error) {
	caCert, err := os.ReadFile(r.CACert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caKey, err := os.ReadFile(r.CAKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA key: %w", err)
	}

	return caCert, caKey, nil
}

// loadAcceptedCAs loads the accepted CA certificates from file.
func (r *Registrator) loadAcceptedCAs() ([]byte, error) {
	acceptedCAs, err := os.ReadFile(r.AcceptedCAs)
	if err != nil {
		return nil, fmt.Errorf("failed to read accepted CAs: %w", err)
	}

	return acceptedCAs, nil
}
