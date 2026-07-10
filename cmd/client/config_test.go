package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadClientConfigFromFile_Valid(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
backoff:
  interval: 1s
  multiplier: 2.0
  max_interval: 30s
  max_time: 10m
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
    remote_addr: 0.0.0.0:80
  api:
    proto: tcp4
    addr: :9090
    remote_addr: 0.0.0.0:9090
`
	f := writeTempFile(t, content)

	c, err := loadClientConfigFromFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if c.ServerAddr != "192.168.1.1:5223" {
		t.Fatalf("expected server_addr 192.168.1.1:5223, got %s", c.ServerAddr)
	}
	if c.Backoff.Interval != 1*time.Second {
		t.Fatalf("expected interval 1s, got %v", c.Backoff.Interval)
	}
	if c.Backoff.Multiplier != 2.0 {
		t.Fatalf("expected multiplier 2.0, got %v", c.Backoff.Multiplier)
	}
	if c.Backoff.MaxInterval != 30*time.Second {
		t.Fatalf("expected max_interval 30s, got %v", c.Backoff.MaxInterval)
	}
	if c.Backoff.MaxTime != 10*time.Minute {
		t.Fatalf("expected max_time 10m, got %v", c.Backoff.MaxTime)
	}

	if len(c.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(c.Tunnels))
	}

	web := c.Tunnels["web"]
	if web.Addr != "localhost:8080" {
		t.Fatalf("expected addr localhost:8080, got %s", web.Addr)
	}
	if web.RemoteAddr != "0.0.0.0:80" {
		t.Fatalf("expected remote_addr 0.0.0.0:80, got %s", web.RemoteAddr)
	}

	api := c.Tunnels["api"]
	if api.Addr != "127.0.0.1:9090" {
		t.Fatalf("expected addr 127.0.0.1:9090, got %s", api.Addr)
	}
}

func TestLoadClientConfigFromFile_Defaults(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	c, err := loadClientConfigFromFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if c.Backoff.Interval != DefaultBackoffInterval {
		t.Fatalf("expected default interval %v, got %v", DefaultBackoffInterval, c.Backoff.Interval)
	}
	if c.Backoff.Multiplier != DefaultBackoffMultiplier {
		t.Fatalf("expected default multiplier %v, got %v", DefaultBackoffMultiplier, c.Backoff.Multiplier)
	}
	if c.Backoff.MaxInterval != DefaultBackoffMaxInterval {
		t.Fatalf("expected default max_interval %v, got %v", DefaultBackoffMaxInterval, c.Backoff.MaxInterval)
	}
	if c.Backoff.MaxTime != DefaultBackoffMaxTime {
		t.Fatalf("expected default max_time %v, got %v", DefaultBackoffMaxTime, c.Backoff.MaxTime)
	}
}

func TestLoadClientConfigFromFile_MissingServerAddr(t *testing.T) {
	t.Parallel()

	content := `
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	_, err := loadClientConfigFromFile(f)
	if err == nil {
		t.Fatal("expected error for missing server_addr")
	}
	if !strings.Contains(err.Error(), "server_addr") {
		t.Fatalf("expected server_addr error, got: %v", err)
	}
}

func TestLoadClientConfigFromFile_InvalidProtocol(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: udp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	_, err := loadClientConfigFromFile(f)
	if err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if !strings.Contains(err.Error(), "invalid protocol") {
		t.Fatalf("expected invalid protocol error, got: %v", err)
	}
}

func TestLoadClientConfigFromFile_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := loadClientConfigFromFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for file not found")
	}
	if !strings.Contains(err.Error(), "failed to read file") {
		t.Fatalf("expected read file error, got: %v", err)
	}
}

func TestLoadClientConfigFromFile_InvalidYAML(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  - this is invalid
  - yaml structure
`
	f := writeTempFile(t, content)

	_, err := loadClientConfigFromFile(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse file") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestCompleteTCP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tunnel      Tunnel
		wantAddr    string
		wantRemote  string
		wantErr     bool
		errContains string
	}{
		{
			name:    "full config",
			tunnel:  Tunnel{Protocol: "tcp", Addr: "localhost:8080", RemoteAddr: "0.0.0.0:80"},
			wantAddr:   "localhost:8080",
			wantRemote: "0.0.0.0:80",
		},
		{
			name:    "auto remote_addr from addr port",
			tunnel:  Tunnel{Protocol: "tcp", Addr: "localhost:8080"},
			wantAddr:   "localhost:8080",
			wantRemote: "0.0.0.0:8080",
		},
		{
			name:    "port-only addr normalization",
			tunnel:  Tunnel{Protocol: "tcp", Addr: ":9090", RemoteAddr: "0.0.0.0:9090"},
			wantAddr:   "127.0.0.1:9090",
			wantRemote: "0.0.0.0:9090",
		},
		{
			name:        "missing addr",
			tunnel:      Tunnel{Protocol: "tcp"},
			wantErr:     true,
			errContains: "addr: missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tun := tt.tunnel
			err := completeTCP(&tun)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatal(err)
			}
			if tun.Addr != tt.wantAddr {
				t.Fatalf("expected addr %q, got %q", tt.wantAddr, tun.Addr)
			}
			if tun.RemoteAddr != tt.wantRemote {
				t.Fatalf("expected remote_addr %q, got %q", tt.wantRemote, tun.RemoteAddr)
			}
		})
	}
}

func TestLoadClientConfigFromFile_HTTPTunnel(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  myapp:
    proto: http
    addr: localhost:8080
    subdomain: myapp
`
	f := writeTempFile(t, content)

	c, err := loadClientConfigFromFile(f)
	if err != nil {
		t.Fatal(err)
	}

	myapp := c.Tunnels["myapp"]
	if myapp.Addr != "localhost:8080" {
		t.Fatalf("expected addr localhost:8080, got %s", myapp.Addr)
	}
	if myapp.Subdomain != "myapp" {
		t.Fatalf("expected subdomain myapp, got %s", myapp.Subdomain)
	}
}

func TestCompleteHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tunnel      Tunnel
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid",
			tunnel: Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "myapp"},
		},
		{
			name:        "missing addr",
			tunnel:      Tunnel{Protocol: "http", Subdomain: "myapp"},
			wantErr:     true,
			errContains: "addr: missing",
		},
		{
			name:        "missing subdomain",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080"},
			wantErr:     true,
			errContains: "subdomain: missing",
		},
		{
			name:        "remote_addr not allowed",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "myapp", RemoteAddr: "0.0.0.0:80"},
			wantErr:     true,
			errContains: "remote_addr: not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tun := tt.tunnel
			err := completeHTTP(&tun)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tt.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}
