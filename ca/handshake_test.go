// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"
)

// TestIssuedCertsCompleteRealHandshake proves the actual point of this
// package: a server cert and a client cert issued by the same CA complete a
// real mutual-TLS handshake using the same tls.Config shape go-tcp-tunnel's
// server.go/client.go already build from -tls-crt/-tls-key/-ca-crt. This is
// deliberately independent of the tunnel package's own machinery — it only
// needs crypto/tls, proving the certs work on their own merits.
func TestIssuedCertsCompleteRealHandshake(t *testing.T) {
	t.Parallel()

	caCertPEM, caKeyPEM, err := GenerateCA("test CA", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	serverCertPEM, serverKeyPEM, err := IssueCert(caCertPEM, caKeyPEM, "server", []string{"127.0.0.1"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	clientCertPEM, clientKeyPEM, err := IssueCert(caCertPEM, caKeyPEM, "client", nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caCertPEM) {
		t.Fatal("failed to add CA cert to pool")
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		t.Fatal(err)
	}

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    roots,
	}
	clientTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      roots,
		ServerName:   "127.0.0.1",
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		tlsConn, ok := conn.(*tls.Conn)
		if !ok {
			done <- err
			return
		}
		done <- tlsConn.Handshake()
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), clientTLSConfig)
	if err != nil {
		t.Fatalf("client dial/handshake failed: %v", err)
	}
	defer conn.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server-side handshake failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not complete handshake in time")
	}
}
