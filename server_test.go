package tunnel

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ChacheGS/go-tcp-tunnel/id"
	"github.com/ChacheGS/go-tcp-tunnel/proto"
)

// testTLSConfig creates a CA + server + client cert setup for mTLS testing.
func testTLSConfig(t *testing.T) (serverTLS *tls.Config, clientTLS *tls.Config) {
	t.Helper()

	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatal(err)
	}
	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	caKeyDER, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		t.Fatal(err)
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: caKeyDER})

	// Generate server cert
	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverCertDER, _ := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	serverKeyDER, _ := x509.MarshalECPrivateKey(serverKey)
	serverCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertDER})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyDER})
	serverTLSCert, _ := tls.X509KeyPair(serverCertPEM, serverKeyPEM)

	// Generate client cert
	clientKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertDER, _ := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	clientKeyDER, _ := x509.MarshalECPrivateKey(clientKey)
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: clientKeyDER})
	clientTLSCert, _ := tls.X509KeyPair(clientCertPEM, clientKeyPEM)

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCertPEM)

	_ = caKeyPEM // used for CA cert generation

	serverTLS = &tls.Config{
		Certificates: []tls.Certificate{serverTLSCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}

	clientTLS = &tls.Config{
		Certificates: []tls.Certificate{clientTLSCert},
		RootCAs:      caPool,
		ServerName:   "localhost",
	}

	return serverTLS, clientTLS
}

func TestNewServer_WithListener(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener: ln,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServer_MissingAddr(t *testing.T) {
	t.Parallel()

	_, err := NewServer(&ServerConfig{})
	if err == nil {
		t.Fatal("expected error for missing addr")
	}
}

func TestNewServer_MissingTLSConfig(t *testing.T) {
	t.Parallel()

	_, err := NewServer(&ServerConfig{
		Addr: "127.0.0.1:0",
	})
	if err == nil {
		t.Fatal("expected error for missing TLS config")
	}
}

func TestNewServer_NilLogger(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener: ln,
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.logger == nil {
		t.Fatal("expected non-nil default logger")
	}
}

func TestServer_Addr(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	addr := s.Addr()
	if addr == "" {
		t.Fatal("expected non-empty addr")
	}
	if addr != ln.Addr().String() {
		t.Fatalf("expected %s, got %s", ln.Addr().String(), addr)
	}
}

func TestServer_Addr_NilListener(t *testing.T) {
	t.Parallel()

	s := &Server{}
	if got := s.Addr(); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestServer_HTTPAddr_NilListener(t *testing.T) {
	t.Parallel()

	s := &Server{}
	if got := s.HTTPAddr(); got != "" {
		t.Fatalf("expected empty string when http listener is not running, got %q", got)
	}
}

func TestServer_Stop(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic and should close listener
	s.Stop()

	// Verify listener is closed
	_, err = ln.Accept()
	if err == nil {
		t.Fatal("expected error after stopping server")
	}
}

func TestServer_disconnected_NoItem(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic when identifier is not in registry
	identifier := id.New([]byte("unknown"))
	s.disconnected(identifier)
}

func TestServer_disconnected_WithListeners(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	// Create a listener to be tracked
	tunnelLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	item := &RegistryItem{
		Hosts:     []string{},
		Listeners: []net.Listener{tunnelLn},
	}
	if err := s.set(item, identifier); err != nil {
		t.Fatal(err)
	}

	// disconnected should close the tunnel listener
	s.disconnected(identifier)

	// Verify listener was closed
	_, err = tunnelLn.Accept()
	if err == nil {
		t.Fatal("expected error from closed listener")
	}
}

// waitConnected polls c.Connected() until it's true or timeout elapses,
// failing the test in the latter case.
func waitConnected(t *testing.T, c *Client, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	for !c.Connected() {
		select {
		case <-deadline:
			t.Fatal("client did not connect within timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestServer_addTunnels_TCP(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"web": {Protocol: proto.TCP, Addr: "127.0.0.1:0"},
	}

	err = s.addTunnels(tunnels, identifier)
	if err != nil {
		t.Fatalf("addTunnels failed: %v", err)
	}

	// Cleanup: clear the registry to close created listeners
	s.disconnected(identifier)
}

func TestServer_addTunnels_UnsupportedProto(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"web": {Protocol: "udp", Addr: "127.0.0.1:0"},
	}

	err = s.addTunnels(tunnels, identifier)
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func TestServer_addTunnels_ListenError(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"web": {Protocol: proto.TCP, Addr: "invalid-addr-no-port"},
	}

	err = s.addTunnels(tunnels, identifier)
	if err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}

func TestServer_notifyError_NonNilError(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))

	// notifyError with non-nil error but no connection in pool.
	// Should not panic; the HTTP request will fail but error is only logged.
	s.notifyError(fmt.Errorf("test error"), identifier)
}

func TestServer_notifyError_NilError(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))

	// Should short-circuit, no panic
	s.notifyError(nil, identifier)
}

func TestServer_Unsubscribe(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	if !s.IsSubscribed(identifier) {
		t.Fatal("expected client to be subscribed")
	}

	item := s.Unsubscribe(identifier)
	if item == nil {
		t.Fatal("expected non-nil registry item")
	}

	if s.IsSubscribed(identifier) {
		t.Fatal("expected client to be unsubscribed")
	}
}

func TestServer_Ping_NotConnected(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))
	_, err = s.Ping(identifier)
	if err != errClientNotConnected {
		t.Fatalf("expected errClientNotConnected, got %v", err)
	}
}

func TestServer_connectRequest(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))
	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedHost:  "localhost:80",
		ForwardedProto: proto.TCP,
	}

	req, err := s.connectRequest(identifier, msg, nil)
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != "PUT" {
		t.Fatalf("expected PUT method, got %s", req.Method)
	}

	if req.URL.String() != s.connPool.URL(identifier) {
		t.Fatalf("expected URL %s, got %s", s.connPool.URL(identifier), req.URL.String())
	}

	if req.Header.Get(proto.HeaderAction) != proto.ActionProxy {
		t.Fatalf("expected action header %s, got %s", proto.ActionProxy, req.Header.Get(proto.HeaderAction))
	}

	if req.Header.Get(proto.HeaderForwardedHost) != "localhost:80" {
		t.Fatalf("expected forwarded host localhost:80, got %s", req.Header.Get(proto.HeaderForwardedHost))
	}

	if req.Header.Get(proto.HeaderForwardedProto) != proto.TCP {
		t.Fatalf("expected forwarded proto tcp, got %s", req.Header.Get(proto.HeaderForwardedProto))
	}
}

func TestServer_Start_ContextCancel(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()

	// Give Start time to enter its loop
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		// net.ErrClosed is expected since Stop closes the listener
		if err == nil {
			t.Fatal("expected error from Start")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after context cancel")
	}
}

func TestServer_Start_HTTPListenerBindFailureClosesControlListener(t *testing.T) {
	t.Parallel()

	// Occupy an address so the internal http listener fails to bind.
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
		HTTPAddr:   occupied.Addr().String(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail when the http listener can't bind")
	}

	// The control listener (already open before Start was called) must be
	// closed on this early-return path, or its file descriptor leaks since
	// nothing else will ever close it.
	if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("expected control listener to be closed after Start failure, got err: %v", err)
	}
}

func TestServer_listen_ClosedListener(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	tunnelLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))

	// Close the listener before calling listen
	tunnelLn.Close()

	done := make(chan struct{})
	go func() {
		s.listen(tunnelLn, identifier)
		close(done)
	}()

	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(3 * time.Second):
		t.Fatal("listen did not return for closed listener")
	}
}

func TestServer_handleClient_AutoSubscribe_E2E(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := testTLSConfig(t)

	// Create a raw TCP listener (not TLS) — server wraps with tls.Server internally
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:      ln,
		TLSConfig:     serverTLS,
		AutoSubscribe: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.Start(ctx)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Connect as a client with TLS
	c, err := NewClient(&ClientConfig{
		ServerAddr:      ln.Addr().String(),
		TLSClientConfig: clientTLS,
		Tunnels: map[string]*proto.Tunnel{
			"web": {Protocol: proto.TCP, Addr: "127.0.0.1:0"},
		},
		Proxy: Proxy(ProxyFuncs{}),
	})
	if err != nil {
		t.Fatal(err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())

	clientDone := make(chan error, 1)
	go func() {
		clientDone <- c.Start(clientCtx)
	}()

	// Wait for connection to be established
	deadline := time.After(5 * time.Second)
	for {
		if c.Connected() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("client did not connect within timeout")
		case err := <-clientDone:
			t.Fatalf("client Start returned early: %v", err)
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Cleanup: stop client first, then server
	c.Stop()
	clientCancel()
	select {
	case <-clientDone:
	case <-time.After(3 * time.Second):
		// Client may not return promptly; that's OK for this test
	}
	s.Stop()
}

func TestServer_handleClient_NotSubscribed(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:      ln,
		AutoSubscribe: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a pipe to simulate a connection (not a *tls.Conn, so it will be rejected)
	server, client := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		s.handleClient(server)
		close(done)
	}()

	select {
	case <-done:
		// handleClient returned (rejected due to non-TLS conn)
	case <-time.After(3 * time.Second):
		t.Fatal("handleClient did not return")
	}
}

func TestServer_HTTPAddr_StoredOnConfig(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
		HTTPAddr:   "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.config.BaseDomain != "tunnel.example.com" {
		t.Fatalf("expected base domain to be stored, got %q", s.config.BaseDomain)
	}
	if s.config.HTTPAddr != "127.0.0.1:0" {
		t.Fatalf("expected http addr to be stored, got %q", s.config.HTTPAddr)
	}
}

func TestServer_addTunnels_HTTP(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: "myapp"},
	}

	if err := s.addTunnels(tunnels, identifier); err != nil {
		t.Fatalf("addTunnels failed: %v", err)
	}

	got, ok := s.registry.Subscriber("myapp.tunnel.example.com")
	if !ok {
		t.Fatal("expected host to be registered")
	}
	if got != identifier {
		t.Fatalf("expected identifier %v, got %v", identifier, got)
	}

	s.disconnected(identifier)
}

func TestServer_addTunnels_HTTP_MissingBaseDomain(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: "myapp"},
	}

	err = s.addTunnels(tunnels, identifier)
	if err == nil {
		t.Fatal("expected error when server has no base domain configured")
	}
}

func TestServer_addTunnels_HTTP_MissingHost(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	tunnels := map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: ""},
	}

	err = s.addTunnels(tunnels, identifier)
	if err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestServer_addTunnels_HTTP_InvalidHost(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test-client"))
	s.Subscribe(identifier)

	// A malicious or misconfigured client could send a Host containing a
	// dot, attempting to register a nested/unintended subdomain, or other
	// characters invalid in a DNS label; the server must reject these
	// rather than trust wire data from any connected client.
	tunnels := map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: "evil.attacker"},
	}

	err = s.addTunnels(tunnels, identifier)
	if err == nil {
		t.Fatal("expected error for invalid host")
	}
}

func TestServer_addTunnels_HTTP_HostCollision(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	first := id.New([]byte("first-client"))
	s.Subscribe(first)
	if err := s.addTunnels(map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: "myapp"},
	}, first); err != nil {
		t.Fatalf("first addTunnels failed: %v", err)
	}
	defer s.disconnected(first)

	second := id.New([]byte("second-client"))
	s.Subscribe(second)
	err = s.addTunnels(map[string]*proto.Tunnel{
		"myapp": {Protocol: proto.HTTP, Host: "myapp"},
	}, second)
	if err == nil {
		t.Fatal("expected error for colliding subdomain")
	}
}

func TestServer_handleHTTPConn_StalledConnectionTimesOut(t *testing.T) {
	// Not t.Parallel(): this test temporarily overrides the package-level
	// DefaultTimeout var, which must not race with other tests reading it.

	original := DefaultTimeout
	DefaultTimeout = 100 * time.Millisecond
	defer func() { DefaultTimeout = original }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	server, client := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		s.handleHTTPConn(server)
		close(done)
	}()

	// Client deliberately never sends anything.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleHTTPConn did not return for a stalled connection; read deadline was not enforced")
	}
}

func TestServer_handleHTTPConn_UnknownHost(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{
		Listener:   ln,
		BaseDomain: "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	server, client := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		s.handleHTTPConn(server)
		close(done)
	}()

	client.SetReadDeadline(time.Now().Add(3 * time.Second))
	req, err := http.NewRequest(http.MethodGet, "http://nosuchapp.tunnel.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Write(client); err != nil {
		t.Fatal(err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(client), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown host, got %d", resp.StatusCode)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleHTTPConn did not return")
	}
}

func TestServer_handleHTTPConn_KnownHost_RoutesToClient(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := testTLSConfig(t)

	controlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer controlLn.Close()

	s, err := NewServer(&ServerConfig{
		Listener:      controlLn,
		TLSConfig:     serverTLS,
		AutoSubscribe: true,
		BaseDomain:    "tunnel.example.com",
		HTTPAddr:      "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	// local echo server the tunnel client will forward to
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		conn, err := echoLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
		_ = n
	}()

	tcpProxy := NewMultiStreamProxy(map[string]string{
		"myapp": echoLn.Addr().String(),
	}, nil)

	c, err := NewClient(&ClientConfig{
		ServerAddr:      controlLn.Addr().String(),
		TLSClientConfig: clientTLS,
		Tunnels: map[string]*proto.Tunnel{
			"myapp": {Protocol: proto.HTTP, Host: "myapp"},
		},
		Proxy: Proxy(ProxyFuncs{Stream: tcpProxy.Proxy}),
	})
	if err != nil {
		t.Fatal(err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()
	go c.Start(clientCtx)

	waitConnected(t, c, 5*time.Second)

	// Dial the server's internal HTTP router directly (simulating the
	// reverse proxy) and request the registered subdomain.
	conn, err := net.Dial("tcp", s.httpListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	req, err := http.NewRequest(http.MethodGet, "http://myapp.tunnel.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from routed tunnel, got %d", resp.StatusCode)
	}
}

func TestServer_handleHTTPConn_HostHeaderIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := testTLSConfig(t)

	controlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer controlLn.Close()

	s, err := NewServer(&ServerConfig{
		Listener:      controlLn,
		TLSConfig:     serverTLS,
		AutoSubscribe: true,
		BaseDomain:    "tunnel.example.com",
		HTTPAddr:      "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		conn, err := echoLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
		_ = n
	}()

	tcpProxy := NewMultiStreamProxy(map[string]string{
		"myapp": echoLn.Addr().String(),
	}, nil)

	c, err := NewClient(&ClientConfig{
		ServerAddr:      controlLn.Addr().String(),
		TLSClientConfig: clientTLS,
		Tunnels: map[string]*proto.Tunnel{
			"myapp": {Protocol: proto.HTTP, Host: "myapp"},
		},
		Proxy: Proxy(ProxyFuncs{Stream: tcpProxy.Proxy}),
	})
	if err != nil {
		t.Fatal(err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()
	go c.Start(clientCtx)

	waitConnected(t, c, 5*time.Second)

	conn, err := net.Dial("tcp", s.httpListener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Mixed-case Host header, legal per RFC 9110 host-matching rules, must
	// still route to the tunnel registered under its lowercase form.
	req, err := http.NewRequest(http.MethodGet, "http://MyApp.Tunnel.Example.Com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for case-varied host of a registered tunnel, got %d", resp.StatusCode)
	}
}

func TestServer_notifyTunnelInfo_NoOpWhenEmpty(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	s, err := NewServer(&ServerConfig{Listener: ln})
	if err != nil {
		t.Fatal(err)
	}

	identifier := id.New([]byte("test"))

	// Should short-circuit without making any request; no panic expected.
	s.notifyTunnelInfo(map[string]string{}, identifier)
}

func TestServer_notifyTunnelInfo_SendsPushMessage(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := testTLSConfig(t)

	controlLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer controlLn.Close()

	s, err := NewServer(&ServerConfig{
		Listener:      controlLn,
		TLSConfig:     serverTLS,
		AutoSubscribe: true,
		BaseDomain:    "tunnel.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	received := make(chan map[string]string, 1)

	c, err := NewClient(&ClientConfig{
		ServerAddr:      controlLn.Addr().String(),
		TLSClientConfig: clientTLS,
		Tunnels: map[string]*proto.Tunnel{
			"myapp": {Protocol: proto.HTTP, Host: "myapp"},
		},
		Proxy: Proxy(ProxyFuncs{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	c.onTunnelInfo = func(hosts map[string]string) {
		received <- hosts
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	defer clientCancel()
	go c.Start(clientCtx)

	select {
	case hosts := <-received:
		if hosts["myapp"] != "myapp.tunnel.example.com" {
			t.Fatalf("expected myapp -> myapp.tunnel.example.com, got %v", hosts)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not receive tunnel info push within timeout")
	}
}
