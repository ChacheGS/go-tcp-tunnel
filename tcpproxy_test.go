package tunnel

import (
	"testing"

	"github.com/jlandowner/go-tcp-tunnel/proto"
)

func TestNewTCPProxy(t *testing.T) {
	t.Parallel()

	p := NewTCPProxy("localhost:8080", nil)
	if p == nil {
		t.Fatal("expected non-nil proxy")
	}
	if p.localAddr != "localhost:8080" {
		t.Fatalf("expected localAddr localhost:8080, got %s", p.localAddr)
	}
	if p.localAddrMap != nil {
		t.Fatal("expected nil localAddrMap for simple proxy")
	}
}

func TestNewMultiTCPProxy(t *testing.T) {
	t.Parallel()

	addrMap := map[string]string{
		"example.com:80": "localhost:8080",
		"other.com:443":  "localhost:8443",
	}

	p := NewMultiTCPProxy(addrMap, nil)
	if p == nil {
		t.Fatal("expected non-nil proxy")
	}
	if p.localAddrMap == nil {
		t.Fatal("expected non-nil localAddrMap")
	}
	if len(p.localAddrMap) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(p.localAddrMap))
	}
}

func TestTCPProxy_localAddrFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		localAddr    string
		localAddrMap map[string]string
		hostPort     string
		expected     string
	}{
		{
			name:      "fallback to localAddr when no map",
			localAddr: "localhost:9090",
			hostPort:  "anything:80",
			expected:  "localhost:9090",
		},
		{
			name:      "fallback to localAddr with empty map",
			localAddr: "localhost:9090",
			localAddrMap: map[string]string{},
			hostPort:  "anything:80",
			expected:  "localhost:9090",
		},
		{
			name: "exact host:port match",
			localAddrMap: map[string]string{
				"example.com:80": "localhost:8080",
			},
			hostPort: "example.com:80",
			expected: "localhost:8080",
		},
		{
			name: "port-only match",
			localAddrMap: map[string]string{
				"80": "localhost:8080",
			},
			hostPort: "example.com:80",
			expected: "localhost:8080",
		},
		{
			name: "host-only match",
			localAddrMap: map[string]string{
				"example.com": "localhost:8080",
			},
			hostPort: "example.com:80",
			expected: "localhost:8080",
		},
		{
			name: "0.0.0.0:port match",
			localAddrMap: map[string]string{
				"0.0.0.0:80": "localhost:8080",
			},
			hostPort: "example.com:80",
			expected: "localhost:8080",
		},
		{
			name: "precedence: host:port over port",
			localAddrMap: map[string]string{
				"example.com:80": "localhost:1111",
				"80":             "localhost:2222",
			},
			hostPort: "example.com:80",
			expected: "localhost:1111",
		},
		{
			name: "precedence: port over host",
			localAddrMap: map[string]string{
				"80":          "localhost:2222",
				"example.com": "localhost:3333",
			},
			hostPort: "example.com:80",
			expected: "localhost:2222",
		},
		{
			name: "no match falls back to localAddr",
			localAddr: "localhost:9999",
			localAddrMap: map[string]string{
				"other.com:80": "localhost:8080",
			},
			hostPort: "example.com:80",
			expected: "localhost:9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TCPProxy{
				localAddr:    tt.localAddr,
				localAddrMap: tt.localAddrMap,
			}
			result := p.localAddrFor(tt.hostPort)
			if result != tt.expected {
				t.Errorf("localAddrFor(%q) = %q, want %q", tt.hostPort, result, tt.expected)
			}
		})
	}
}

func TestTCPProxy_Proxy_UnsupportedProtocol(t *testing.T) {
	t.Parallel()

	p := NewTCPProxy("localhost:8080", nil)

	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedHost:  "localhost:80",
		ForwardedProto: "udp",
	}

	// Should return without panic (unsupported protocol logged)
	p.Proxy(nil, nil, msg)
}
