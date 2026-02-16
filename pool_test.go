package tunnel

import (
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
