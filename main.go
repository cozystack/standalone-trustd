// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main implements a standalone trustd service.
package main

import (
	"context"
	"crypto/subtle"
	"flag"
	"fmt"
	"log"
	"net"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	securityapi "github.com/siderolabs/talos/pkg/machinery/api/security"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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
	verbosity   = flag.Int("v", 2, "verbosity level (0=min, 1=conn, 2=rpc, 3=payload)")
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

	// Create gRPC server with logging and Basic Auth interceptors
	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.ChainUnaryInterceptor(
			unaryLoggingInterceptor(),
			basicAuthInterceptor(*authToken),
		),
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

	logv(0, "Starting standalone trustd on port %d", *port)

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
	logv(0, "Shutting down server...")
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
	logv(1, "Debug server would start on port %d", port)
	<-ctx.Done()
}

func createListener(port int) (net.Listener, error) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	return &loggingListener{Listener: listener}, nil
}

// loggingListener wraps a net.Listener to log accepted and closed connections.
type loggingListener struct {
	net.Listener
}

func (l *loggingListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	logv(1, "accepted connection from %v", c.RemoteAddr())
	return &loggingConn{Conn: c}, nil
}

type loggingConn struct {
	net.Conn
	closeOnce sync.Once
}

func (c *loggingConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.Conn.Close()
		logv(1, "closed connection from %v", c.RemoteAddr())
	})
	return err
}

// basicAuthInterceptor enforces Basic auth on incoming RPC calls.
// Username is ignored; password must equal expectedToken.
func basicAuthInterceptor(expectedToken string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		p, _ := peer.FromContext(ctx)

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Printf("auth failed for %s from %v: missing metadata", info.FullMethod, peerAddr(p))
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Require raw token header (Talos sends `token: <value>`)
		tokenHeaders := md.Get("token")
		if len(tokenHeaders) == 0 {
			logv(2, "auth failed for %s from %v: missing token header", info.FullMethod, peerAddr(p))
			return nil, status.Error(codes.Unauthenticated, "missing token header")
		}
		providedToken := tokenHeaders[0]
		if subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) != 1 {
			logv(2, "auth failed for %s from %v: invalid token", info.FullMethod, peerAddr(p))
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		return handler(ctx, req)
	}
}

// unaryLoggingInterceptor logs incoming requests, their outcome and latency.
func unaryLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		p, _ := peer.FromContext(ctx)

		// Log incoming metadata (with redaction)
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if *verbosity >= 2 {
				log.Printf("rpc %s from %v headers: %v", info.FullMethod, peerAddr(p), md)
			}
		}

		// Log request payload
		if pm, ok := req.(proto.Message); ok {
			if *verbosity >= 3 {
				b, _ := (protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}).Marshal(pm)
				log.Printf("rpc %s request json:\n%s", info.FullMethod, string(b))
			}
		}
		if r, ok := req.(*securityapi.CertificateRequest); ok {
			if *verbosity >= 3 {
				log.Printf("rpc %s request.csr (len=%d):\n%s", info.FullMethod, len(r.Csr), string(r.Csr))
			}
		}

		resp, err := handler(ctx, req)
		duration := time.Since(start)
		code := status.Code(err)

		// Log response payload
		if resp != nil {
			if pm, ok := resp.(proto.Message); ok {
				if *verbosity >= 3 {
					b, _ := (protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}).Marshal(pm)
					log.Printf("rpc %s response json:\n%s", info.FullMethod, string(b))
				}
			}
			if r, ok := resp.(*securityapi.CertificateResponse); ok {
				if *verbosity >= 3 {
					log.Printf("rpc %s response.ca (len=%d):\n%s", info.FullMethod, len(r.Ca), string(r.Ca))
					log.Printf("rpc %s response.crt (len=%d):\n%s", info.FullMethod, len(r.Crt), string(r.Crt))
				}
			}
		}

		if err != nil {
			logv(2, "rpc %s from %v -> %s (%s): %v", info.FullMethod, peerAddr(p), code, duration, err)
		} else {
			logv(2, "rpc %s from %v -> %s (%s)", info.FullMethod, peerAddr(p), code, duration)
		}

		return resp, err
	}
}

// peerAddr formats peer address safely for logging.
func peerAddr(p *peer.Peer) interface{} {
	if p == nil || p.Addr == nil {
		return "unknown"
	}
	return p.Addr
}

// logv prints a log line if the current verbosity is >= level.
func logv(level int, format string, args ...interface{}) {
	if verbosity != nil && *verbosity >= level {
		log.Printf(format, args...)
	}
}
