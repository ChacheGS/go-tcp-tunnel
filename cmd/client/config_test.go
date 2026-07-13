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

func TestLoadClientConfigFromFile_TLSFieldsRelativeToConfigDir(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tls_crt: certs/laptop.crt
tls_key: certs/laptop.key
ca_crt: certs/ca.crt
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)
	dir := filepath.Dir(f)

	c, err := loadClientConfigFromFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(dir, "certs/laptop.crt"); c.TLSCrt != want {
		t.Fatalf("expected tls_crt %s, got %s", want, c.TLSCrt)
	}
	if want := filepath.Join(dir, "certs/laptop.key"); c.TLSKey != want {
		t.Fatalf("expected tls_key %s, got %s", want, c.TLSKey)
	}
	if want := filepath.Join(dir, "certs/ca.crt"); c.CACrt != want {
		t.Fatalf("expected ca_crt %s, got %s", want, c.CACrt)
	}
}

func TestLoadClientConfigFromFile_TLSFieldsAbsoluteUnchanged(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tls_crt: /etc/goku/tls.crt
tls_key: /etc/goku/tls.key
ca_crt: /etc/goku/ca.crt
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

	if c.TLSCrt != "/etc/goku/tls.crt" {
		t.Fatalf("expected absolute tls_crt unchanged, got %s", c.TLSCrt)
	}
	if c.TLSKey != "/etc/goku/tls.key" {
		t.Fatalf("expected absolute tls_key unchanged, got %s", c.TLSKey)
	}
	if c.CACrt != "/etc/goku/ca.crt" {
		t.Fatalf("expected absolute ca_crt unchanged, got %s", c.CACrt)
	}
}

func TestLoadClientConfigFromFile_TLSFieldsOmitted(t *testing.T) {
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

	if c.TLSCrt != "" || c.TLSKey != "" || c.CACrt != "" {
		t.Fatalf("expected empty TLS fields when omitted, got crt=%q key=%q ca=%q", c.TLSCrt, c.TLSKey, c.CACrt)
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
			name:       "full config",
			tunnel:     Tunnel{Protocol: "tcp", Addr: "localhost:8080", RemoteAddr: "0.0.0.0:80"},
			wantAddr:   "localhost:8080",
			wantRemote: "0.0.0.0:80",
		},
		{
			name:       "auto remote_addr from addr port",
			tunnel:     Tunnel{Protocol: "tcp", Addr: "localhost:8080"},
			wantAddr:   "localhost:8080",
			wantRemote: "0.0.0.0:8080",
		},
		{
			name:       "port-only addr normalization",
			tunnel:     Tunnel{Protocol: "tcp", Addr: ":9090", RemoteAddr: "0.0.0.0:9090"},
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

func TestLoadClientConfigFromFile_HTTPTunnel_SubdomainDefaultsToName(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  myapp:
    proto: http
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	c, err := loadClientConfigFromFile(f)
	if err != nil {
		t.Fatal(err)
	}

	myapp := c.Tunnels["myapp"]
	if myapp.Subdomain != "myapp" {
		t.Fatalf("expected subdomain to default to tunnel name myapp, got %s", myapp.Subdomain)
	}
}

func TestLoadClientConfigFromFile_HTTPTunnel_InvalidNameNoSubdomainOverride(t *testing.T) {
	t.Parallel()

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  My_App:
    proto: http
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	_, err := loadClientConfigFromFile(f)
	if err == nil {
		t.Fatal("expected error: tunnel name My_App is not a valid DNS label and no subdomain override was given")
	}
	if !strings.Contains(err.Error(), "My_App") || !strings.Contains(err.Error(), "subdomain") {
		t.Fatalf("expected error mentioning the invalid tunnel name and subdomain, got: %v", err)
	}
}

func TestLoadClientConfigFromFile_DuplicateSubdomainRejected(t *testing.T) {
	t.Parallel()

	// Two differently-named tunnels sharing a subdomain would silently
	// overwrite each other's target in the proxy's dial-target map with no
	// error surfaced at connection time, so this must be rejected at load.
	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: http
    addr: localhost:3000
    subdomain: shared
  api:
    proto: http
    addr: localhost:4000
    subdomain: shared
`
	f := writeTempFile(t, content)

	_, err := loadClientConfigFromFile(f)
	if err == nil {
		t.Fatal("expected error for duplicate subdomain across tunnels")
	}
	if !strings.Contains(err.Error(), "shared") {
		t.Fatalf("expected error mentioning the colliding subdomain, got: %v", err)
	}
}

func TestCompleteHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		tunnelName  string
		tunnel      Tunnel
		wantErr     bool
		errContains string
		wantSubdom  string
	}{
		{
			name:       "valid",
			tunnelName: "irrelevant",
			tunnel:     Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "myapp"},
			wantSubdom: "myapp",
		},
		{
			name:        "missing addr",
			tunnelName:  "myapp",
			tunnel:      Tunnel{Protocol: "http", Subdomain: "myapp"},
			wantErr:     true,
			errContains: "addr: missing",
		},
		{
			name:       "subdomain defaults to tunnel name when omitted",
			tunnelName: "myapp",
			tunnel:     Tunnel{Protocol: "http", Addr: "localhost:8080"},
			wantSubdom: "myapp",
		},
		{
			name:        "invalid tunnel name with no subdomain override",
			tunnelName:  "My_App",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080"},
			wantErr:     true,
			errContains: "not a valid DNS label",
		},
		{
			name:        "remote_addr not allowed",
			tunnelName:  "myapp",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "myapp", RemoteAddr: "0.0.0.0:80"},
			wantErr:     true,
			errContains: "remote_addr: not supported",
		},
		{
			name:        "subdomain with dot rejected",
			tunnelName:  "myapp",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "my.app"},
			wantErr:     true,
			errContains: "not a valid DNS label",
		},
		{
			name:        "uppercase subdomain rejected",
			tunnelName:  "myapp",
			tunnel:      Tunnel{Protocol: "http", Addr: "localhost:8080", Subdomain: "MyApp"},
			wantErr:     true,
			errContains: "not a valid DNS label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tun := tt.tunnel
			err := completeHTTP(&tun, tt.tunnelName)

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
			if tun.Subdomain != tt.wantSubdom {
				t.Fatalf("expected subdomain %q, got %q", tt.wantSubdom, tun.Subdomain)
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
