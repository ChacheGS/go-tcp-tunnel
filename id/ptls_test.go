package id

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"strings"
	"testing"
)

func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
		Leaf:        cert,
	}

	return tlsCert, cert
}

func TestPeerID_Success(t *testing.T) {
	t.Parallel()

	clientCert, rawCert := generateSelfSignedCert(t)
	serverCert, _ := generateSelfSignedCert(t)

	// Create a cert pool that trusts the client cert
	clientCertPool := x509.NewCertPool()
	clientCertPool.AddCert(rawCert)

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAnyClientCert,
		ClientCAs:    clientCertPool,
	}

	// Create a cert pool that trusts the server cert
	serverCertPool := x509.NewCertPool()
	serverCertPool.AddCert(serverCert.Leaf)

	clientTLSConfig := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: true,
	}

	// Set up TLS listener
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	errCh := make(chan error, 1)
	idCh := make(chan ID, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		peerID, err := PeerID(tlsConn)
		if err != nil {
			errCh <- err
			return
		}
		idCh <- peerID
	}()

	// Client connects
	conn, err := tls.Dial("tcp", ln.Addr().String(), clientTLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	select {
	case err := <-errCh:
		t.Fatal("server error:", err)
	case peerID := <-idCh:
		expected := New(rawCert.Raw)
		if peerID != expected {
			t.Fatalf("expected ID %v, got %v", expected, peerID)
		}
	}
}

func TestPeerID_NoCert(t *testing.T) {
	t.Parallel()

	serverCert, _ := generateSelfSignedCert(t)

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}

	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	errCh := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		_, err = PeerID(tlsConn)
		errCh <- err
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	tlsConn := tls.Client(conn, clientTLSConfig)
	defer tlsConn.Close()

	if err := tlsConn.Handshake(); err != nil {
		t.Fatal(err)
	}

	err = <-errCh
	if err == nil {
		t.Fatal("expected error for no client cert")
	}

	improperErr, ok := err.(ImproperCertsNumberError)
	if !ok {
		t.Fatalf("expected ImproperCertsNumberError, got %T: %v", err, err)
	}
	if improperErr.n != 0 {
		t.Fatalf("expected 0 certs, got %d", improperErr.n)
	}
}

func TestImproperCertsNumberError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		n        int
		expected string
	}{
		{0, "ptls: expecting 1 peer certificate, got 0"},
		{3, "ptls: expecting 1 peer certificate, got 3"},
	}

	for _, tt := range tests {
		err := ImproperCertsNumberError{tt.n}
		if !strings.Contains(err.Error(), tt.expected) {
			t.Errorf("expected %q, got %q", tt.expected, err.Error())
		}
	}
}
