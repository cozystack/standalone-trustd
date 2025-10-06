// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main implements a standalone trustd service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/cozystack/standalone-trustd/internal/registrator"
	"github.com/cozystack/standalone-trustd/internal/tlsconfig"
)

var (
	port        = flag.Int("port", 50001, "Port to listen on")
	caCert      = flag.String("ca-cert", "", "Path to CA certificate file")
	caKey       = flag.String("ca-key", "", "Path to CA private key file")
	serverCert  = flag.String("server-cert", "", "Path to server certificate file")
	serverKey   = flag.String("server-key", "", "Path to server private key file")
	acceptedCAs = flag.String("accepted-cas", "", "Path to accepted CA certificates file")
	authToken   = flag.String("auth-token", "", "Authentication token for client connections")
	debugPort   = flag.Int("debug-port", 9983, "Debug server port")
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)

	// Validate required flags
	if *caCert == "" || *caKey == "" {
		return fmt.Errorf("--ca-cert and --ca-key are required")
	}
	if *serverCert == "" || *serverKey == "" {
		return fmt.Errorf("--server-cert and --server-key are required")
	}
	if *acceptedCAs == "" {
		return fmt.Errorf("--accepted-cas is required")
	}
	if *authToken == "" {
		return fmt.Errorf("--auth-token is required")
	}

	// Start debug server
	go runDebugServer(ctx, *debugPort)

	// Load TLS configuration
	tlsConfig, err := tlsconfig.NewTLSConfig(*caCert, *caKey, *serverCert, *serverKey, *acceptedCAs)
	if err != nil {
		return fmt.Errorf("failed to create TLS configuration: %w", err)
	}

	// Create gRPC server
	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
	)

	// Create registrator
	reg := &registrator.Registrator{
		CACert:      *caCert,
		CAKey:       *caKey,
		AcceptedCAs: *acceptedCAs,
		AuthToken:   *authToken,
	}

	// Register services
	reg.Register(server)

	// Start server
	listener, err := createListener(*port)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	log.Printf("Starting standalone trustd on port %d", *port)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("server failed: %w", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	log.Println("Shutting down server...")
	server.GracefulStop()

	// Check if server had any errors
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func runDebugServer(ctx context.Context, port int) {
	// Simple debug server implementation
	// In a real implementation, you might want to use a proper debug server
	log.Printf("Debug server would start on port %d", port)
	<-ctx.Done()
}

func createListener(port int) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	return listener, nil
}
