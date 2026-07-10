package tunnel

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/jlandowner/go-tcp-tunnel/id"
	"golang.org/x/net/http2"
)

func TestConnPool_URL(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	url := p.URL(identifier)
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("expected URL to start with https://, got %s", url)
	}
	if !strings.Contains(url, identifier.String()) {
		t.Fatalf("expected URL to contain identifier string, got %s", url)
	}
}

func TestConnPool_Addr(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	addr := p.addr(identifier)
	if !strings.HasSuffix(addr, ":443") {
		t.Fatalf("expected addr to end with :443, got %s", addr)
	}
	if !strings.Contains(addr, identifier.String()) {
		t.Fatalf("expected addr to contain identifier, got %s", addr)
	}
}

func TestConnPool_Identifier(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	original := id.New([]byte("test"))

	addr := p.addr(original)
	recovered := p.identifier(addr)

	if original != recovered {
		t.Fatalf("expected identifier round-trip: %v != %v", original, recovered)
	}
}

func TestConnPool_GetClientConn_Connected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	h2Server := &http2.Server{}
	go func() {
		h2Server.ServeConn(serverConn, &http2.ServeConnOpts{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		})
	}()

	err := p.AddConn(clientConn, identifier)
	if err != nil {
		t.Fatalf("AddConn failed: %v", err)
	}

	addr := p.addr(identifier)
	cc, err := p.GetClientConn(nil, addr)
	if err != nil {
		t.Fatalf("GetClientConn failed: %v", err)
	}
	if cc == nil {
		t.Fatal("expected non-nil ClientConn")
	}

	p.DeleteConn(identifier)
}

func TestConnPool_GetClientConn_NotConnected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)

	_, err := p.GetClientConn(nil, "nonexistent:443")
	if err != errClientNotConnected {
		t.Fatalf("expected errClientNotConnected, got %v", err)
	}
}

func TestConnPool_DeleteConn_NotConnected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	// Should not panic
	p.DeleteConn(identifier)
}

func TestConnPool_MarkDead_NotConnected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)

	// Should not panic when no matching connection
	p.MarkDead(nil)
}

func TestConnPool_Ping_NotConnected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	_, err := p.Ping(identifier)
	if err != errClientNotConnected {
		t.Fatalf("expected errClientNotConnected, got %v", err)
	}
}

func TestConnPool_FreeCallback(t *testing.T) {
	t.Parallel()

	var freedID id.ID
	freed := false

	p := newConnPool(&http2.Transport{}, func(identifier id.ID) {
		freedID = identifier
		freed = true
	})

	// Verify the free callback is stored
	if p.free == nil {
		t.Fatal("expected free callback to be set")
	}

	// We can't easily add a real HTTP/2 connection, but we can verify
	// the pool structure is correct
	if len(p.conns) != 0 {
		t.Fatal("expected empty conns map")
	}

	// Verify free callback works when called directly (as close() would call it)
	identifier := id.New([]byte("test"))
	p.free(identifier)

	if !freed {
		t.Fatal("expected free callback to be called")
	}
	if freedID != identifier {
		t.Fatalf("expected freed ID %v, got %v", identifier, freedID)
	}
}

// h2Listener creates a TCP listener with an HTTP/2 server running on it.
// Returns the listener. Caller must close it.
func h2Listener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h2s := &http2.Server{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go h2s.ServeConn(conn, &http2.ServeConnOpts{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
			})
		}
	}()
	return ln
}

func TestConnPool_AddConn_Success(t *testing.T) {
	t.Parallel()

	ln := h2Listener(t)
	defer ln.Close()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = p.AddConn(conn, identifier)
	if err != nil {
		t.Fatalf("AddConn failed: %v", err)
	}

	// Verify connection is in the pool
	addr := p.addr(identifier)
	p.mu.RLock()
	_, exists := p.conns[addr]
	p.mu.RUnlock()
	if !exists {
		t.Fatal("expected connection in pool")
	}

	// Cleanup
	p.DeleteConn(identifier)
}

func TestConnPool_AddConn_AlreadyConnected(t *testing.T) {
	t.Parallel()

	ln := h2Listener(t)
	defer ln.Close()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	conn1, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = p.AddConn(conn1, identifier)
	if err != nil {
		t.Fatalf("first AddConn failed: %v", err)
	}

	// Second AddConn with same identifier should return errClientAlreadyConnected
	conn2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = p.AddConn(conn2, identifier)
	if err != errClientAlreadyConnected {
		t.Fatalf("expected errClientAlreadyConnected, got %v", err)
	}

	// Cleanup
	p.DeleteConn(identifier)
}

func TestConnPool_DeleteConn_Connected(t *testing.T) {
	t.Parallel()

	ln := h2Listener(t)
	defer ln.Close()

	freed := false
	p := newConnPool(&http2.Transport{}, func(identifier id.ID) {
		freed = true
	})
	identifier := id.New([]byte("test"))

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = p.AddConn(conn, identifier)
	if err != nil {
		t.Fatalf("AddConn failed: %v", err)
	}

	p.DeleteConn(identifier)

	// Verify removed
	addr := p.addr(identifier)
	p.mu.RLock()
	_, exists := p.conns[addr]
	p.mu.RUnlock()
	if exists {
		t.Fatal("expected connection to be removed from pool")
	}

	if !freed {
		t.Fatal("expected free callback to be called")
	}
}

func TestConnPool_Ping_Connected(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	h2Server := &http2.Server{}
	go func() {
		h2Server.ServeConn(serverConn, &http2.ServeConnOpts{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		})
	}()

	err := p.AddConn(clientConn, identifier)
	if err != nil {
		t.Fatalf("AddConn failed: %v", err)
	}

	dur, err := p.Ping(identifier)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if dur <= 0 {
		t.Fatalf("expected positive duration, got %v", dur)
	}

	p.DeleteConn(identifier)
}

func TestConnPool_AddConn_TransportError(t *testing.T) {
	t.Parallel()

	// Use a pipe where the server side is immediately closed,
	// causing NewClientConn to fail
	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	clientConn, serverConn := net.Pipe()
	serverConn.Close() // close immediately so HTTP/2 handshake fails

	err := p.AddConn(clientConn, identifier)
	if err == nil {
		t.Fatal("expected error from AddConn with closed server side")
	}
}

func TestConnPool_AddConn_ReplaceDead(t *testing.T) {
	t.Parallel()

	ln := h2Listener(t)
	defer ln.Close()

	p := newConnPool(&http2.Transport{}, nil)
	identifier := id.New([]byte("test"))

	// Add first connection
	conn1, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	err = p.AddConn(conn1, identifier)
	if err != nil {
		t.Fatalf("first AddConn failed: %v", err)
	}

	// Close the connection to make it dead
	conn1.Close()

	// Second AddConn should detect dead connection via failed ping and replace it
	conn2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	err = p.AddConn(conn2, identifier)
	if err != nil {
		t.Fatalf("second AddConn should succeed after dead conn: %v", err)
	}

	// Cleanup
	p.DeleteConn(identifier)
}

func TestConnPool_Identifier_InvalidAddr(t *testing.T) {
	t.Parallel()

	p := newConnPool(&http2.Transport{}, nil)

	// Invalid addr (no port) should return zero ID
	result := p.identifier("no-port-here")
	var zero id.ID
	if result != zero {
		t.Fatalf("expected zero ID for invalid addr, got %v", result)
	}

	// Invalid base32 in host should return zero ID
	result = p.identifier("!!!invalid!!!:443")
	if result != zero {
		t.Fatalf("expected zero ID for invalid host, got %v", result)
	}
}

func TestConnPool_MarkDead_Connected(t *testing.T) {
	t.Parallel()

	ln := h2Listener(t)
	defer ln.Close()

	freed := false
	p := newConnPool(&http2.Transport{}, func(identifier id.ID) {
		freed = true
	})
	identifier := id.New([]byte("test"))

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	err = p.AddConn(conn, identifier)
	if err != nil {
		t.Fatalf("AddConn failed: %v", err)
	}

	// Get the clientConn from the pool to pass to MarkDead
	addr := p.addr(identifier)
	p.mu.RLock()
	cp := p.conns[addr]
	p.mu.RUnlock()

	p.MarkDead(cp.clientConn)

	// Verify removed
	p.mu.RLock()
	_, exists := p.conns[addr]
	p.mu.RUnlock()
	if exists {
		t.Fatal("expected connection to be removed after MarkDead")
	}

	if !freed {
		t.Fatal("expected free callback to be called after MarkDead")
	}
}
