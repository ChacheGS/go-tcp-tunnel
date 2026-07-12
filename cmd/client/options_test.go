package client

import (
	"bytes"
	"context"
	"flag"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ChacheGS/go-stream-tunnel/log"
	"github.com/ChacheGS/go-stream-tunnel/proto"
)

func TestMain(m *testing.M) {
	// testdata/selfsigned.key is a git-tracked fixture; git doesn't preserve
	// permission bits beyond the executable flag, so a fresh checkout
	// leaves it group/other-readable regardless of local history. Tighten
	// it before any test that loads it through CheckPrivateKeyPermissions
	// runs, since that's a real restriction the production code now
	// enforces, not just a local artifact of this machine.
	os.Chmod("../../testdata/selfsigned.key", 0600)
	os.Exit(m.Run())
}

func TestCompleteArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "id command",
			args: []string{"id"},
		},
		{
			name: "list command",
			args: []string{"list"},
		},
		{
			name:    "list with extra args",
			args:    []string{"list", "extra"},
			wantErr: true,
		},
		{
			name: "start with tunnel name",
			args: []string{"start", "web"},
		},
		{
			name: "start with multiple tunnels",
			args: []string{"start", "web", "ssh"},
		},
		{
			name:    "start with no tunnels",
			args:    []string{"start"},
			wantErr: true,
		},
		{
			name: "start-all command",
			args: []string{"start-all"},
		},
		{
			name:    "start-all with extra args",
			args:    []string{"start-all", "extra"},
			wantErr: true,
		},
		{
			name:    "unknown command",
			args:    []string{"foobar"},
			wantErr: true,
		},
		{
			name:    "empty command",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.Parse(tt.args)

			err := CompleteArgs(fs)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExpBackoff(t *testing.T) {
	t.Parallel()

	c := BackoffConfig{
		Interval:    500 * time.Millisecond,
		Multiplier:  2.0,
		MaxInterval: 30 * time.Second,
		MaxTime:     10 * time.Minute,
	}

	b := expBackoff(c)
	if b.InitialInterval != c.Interval {
		t.Fatalf("expected InitialInterval %v, got %v", c.Interval, b.InitialInterval)
	}
	if b.Multiplier != c.Multiplier {
		t.Fatalf("expected Multiplier %v, got %v", c.Multiplier, b.Multiplier)
	}
	if b.MaxInterval != c.MaxInterval {
		t.Fatalf("expected MaxInterval %v, got %v", c.MaxInterval, b.MaxInterval)
	}
	if b.MaxElapsedTime != c.MaxTime {
		t.Fatalf("expected MaxElapsedTime %v, got %v", c.MaxTime, b.MaxElapsedTime)
	}
}

func TestTunnels(t *testing.T) {
	t.Parallel()

	m := map[string]*Tunnel{
		"web": {Protocol: "tcp", Addr: "localhost:8080", RemoteAddr: "0.0.0.0:80"},
		"ssh": {Protocol: "tcp4", Addr: "localhost:22", RemoteAddr: "0.0.0.0:22"},
	}

	result := tunnels(m)
	if len(result) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(result))
	}

	web := result["web"]
	if web == nil {
		t.Fatal("expected 'web' tunnel")
	}
	if web.Protocol != "tcp" {
		t.Fatalf("expected protocol tcp, got %s", web.Protocol)
	}
	if web.Addr != "0.0.0.0:80" {
		t.Fatalf("expected addr 0.0.0.0:80, got %s", web.Addr)
	}

	ssh := result["ssh"]
	if ssh == nil {
		t.Fatal("expected 'ssh' tunnel")
	}
	if ssh.Protocol != "tcp4" {
		t.Fatalf("expected protocol tcp4, got %s", ssh.Protocol)
	}
	if ssh.Addr != "0.0.0.0:22" {
		t.Fatalf("expected addr 0.0.0.0:22, got %s", ssh.Addr)
	}
}

func TestProxy(t *testing.T) {
	t.Parallel()

	m := map[string]*Tunnel{
		"web": {Protocol: proto.TCP, Addr: "localhost:8080", RemoteAddr: "0.0.0.0:80"},
	}

	pf := proxy(m, log.NewNopLogger())
	if pf == nil {
		t.Fatal("expected non-nil ProxyFunc")
	}
}

func TestTunnels_HTTP(t *testing.T) {
	t.Parallel()

	m := map[string]*Tunnel{
		"myapp": {Protocol: proto.HTTP, Addr: "localhost:8080", Subdomain: "myapp"},
	}

	p := tunnels(m)

	got := p["myapp"]
	if got.Protocol != proto.HTTP {
		t.Fatalf("expected protocol http, got %s", got.Protocol)
	}
	if got.Host != "myapp" {
		t.Fatalf("expected host myapp, got %s", got.Host)
	}
}

func TestProxy_HTTP_BuildsTargetMap(t *testing.T) {
	t.Parallel()

	// local echo server the proxy should dial when it sees ForwardedHost
	// "myapp" — this is the actual behavior TestProxy_HTTP_BuildsTargetMap
	// is named after, not just "does proxy() return non-nil".
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		if n > 0 {
			conn.Write(buf[:n])
		}
	}()

	m := map[string]*Tunnel{
		"myapp": {Protocol: proto.HTTP, Addr: ln.Addr().String(), Subdomain: "myapp"},
	}

	pf := proxy(m, log.NewNopLogger())
	if pf == nil {
		t.Fatal("expected non-nil ProxyFunc")
	}

	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte("ping"))
		pw.Close()
	}()

	var out bytes.Buffer
	pf(&out, pr, &proto.ControlMessage{
		ForwardedHost:  "myapp",
		ForwardedProto: proto.HTTP,
	})

	if out.String() != "ping" {
		t.Fatalf("expected proxy to dial the tunnel's target addr and echo %q, got %q", "ping", out.String())
	}
}

func TestCommand_ClientDefaults(t *testing.T) {
	cmd := Command()
	if err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}

	if opts.config != "tunnel.yml" {
		t.Fatalf("expected default config tunnel.yml, got %s", opts.config)
	}
	if opts.tlsCrt != "tls.crt" {
		t.Fatalf("expected default tls-crt tls.crt, got %s", opts.tlsCrt)
	}
	if opts.tlsKey != "tls.key" {
		t.Fatalf("expected default tls-key tls.key, got %s", opts.tlsKey)
	}
	if opts.rootCA != "tls.crt" {
		t.Fatalf("expected default ca-crt tls.crt, got %s", opts.rootCA)
	}
	if opts.logLevel != 1 {
		t.Fatalf("expected default log-level 1, got %d", opts.logLevel)
	}
}

func TestCommand_ClientCustomFlags(t *testing.T) {
	cmd := Command()
	args := []string{
		"-config", "custom.yml",
		"-tls-crt", "custom.crt",
		"-tls-key", "custom.key",
		"-ca-crt", "custom-ca.crt",
		"-log-level", "2",
	}
	if err := cmd.Parse(args); err != nil {
		t.Fatal(err)
	}

	if opts.config != "custom.yml" {
		t.Fatalf("expected config custom.yml, got %s", opts.config)
	}
	if opts.tlsCrt != "custom.crt" {
		t.Fatalf("expected tls-crt custom.crt, got %s", opts.tlsCrt)
	}
	if opts.tlsKey != "custom.key" {
		t.Fatalf("expected tls-key custom.key, got %s", opts.tlsKey)
	}
	if opts.rootCA != "custom-ca.crt" {
		t.Fatalf("expected ca-crt custom-ca.crt, got %s", opts.rootCA)
	}
	if opts.logLevel != 2 {
		t.Fatalf("expected log-level 2, got %d", opts.logLevel)
	}
}

func TestTLSConfig_Client_MissingCertFile(t *testing.T) {
	Command()
	opts.tlsCrt = "/nonexistent/tls.crt"
	opts.tlsKey = "/nonexistent/tls.key"

	_, err := tlsConfig(&ClientConfig{ServerAddr: "example.com:5223"})
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
}

func TestTLSConfig_Client_KeyFileTooOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningfully enforced on windows")
	}

	keyBytes, err := os.ReadFile("../../testdata/selfsigned.key")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	insecureKey := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(insecureKey, keyBytes, 0644); err != nil {
		t.Fatal(err)
	}

	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = insecureKey
	opts.rootCA = "../../testdata/selfsigned.crt"

	_, err = tlsConfig(&ClientConfig{ServerAddr: "example.com:5223"})
	if err == nil {
		t.Fatal("expected error for a world-readable key file")
	}
	if !strings.Contains(err.Error(), "too open") {
		t.Fatalf("expected 'too open' permissions error, got: %v", err)
	}
}

func TestExecute_IDCommand_KeyFileTooOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningfully enforced on windows")
	}

	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	keyBytes, err := os.ReadFile("../../testdata/selfsigned.key")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	insecureKey := filepath.Join(dir, "tls.key")
	if err := os.WriteFile(insecureKey, keyBytes, 0644); err != nil {
		t.Fatal(err)
	}

	Command()
	opts.config = f
	opts.command = "id"
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = insecureKey

	err = Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for a world-readable key file")
	}
	if !strings.Contains(err.Error(), "too open") {
		t.Fatalf("expected 'too open' permissions error, got: %v", err)
	}
}

func TestTLSConfig_Client_MissingRootCA(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.rootCA = ""

	_, err := tlsConfig(&ClientConfig{ServerAddr: "example.com:5223"})
	if err == nil {
		t.Fatal("expected error for missing root CA")
	}
	if !strings.Contains(err.Error(), "no root CA is given") {
		t.Fatalf("expected 'no root CA is given' error, got: %v", err)
	}
}

func TestTLSConfig_Client_CACertFileNotFound(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.rootCA = "/nonexistent/ca.crt"

	_, err := tlsConfig(&ClientConfig{ServerAddr: "example.com:5223"})
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestTLSConfig_Client_InvalidCAPEM(t *testing.T) {
	dir := t.TempDir()
	badCA := filepath.Join(dir, "bad-ca.crt")
	if err := os.WriteFile(badCA, []byte("not a valid PEM certificate"), 0644); err != nil {
		t.Fatal(err)
	}

	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.rootCA = badCA

	_, err := tlsConfig(&ClientConfig{ServerAddr: "example.com:5223"})
	if err == nil {
		t.Fatal("expected error for invalid CA PEM")
	}
	if !strings.Contains(err.Error(), "failed to parse CA certificate PEM") {
		t.Fatalf("expected PEM parse error, got: %v", err)
	}
}

func TestTLSConfig_Client_InvalidServerAddr(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.rootCA = "../../testdata/selfsigned.crt"

	_, err := tlsConfig(&ClientConfig{ServerAddr: "no-port-here"})
	if err == nil {
		t.Fatal("expected error for server_addr with no port")
	}
}

func TestTLSConfig_Client_Success(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.rootCA = "../../testdata/selfsigned.crt"

	conf, err := tlsConfig(&ClientConfig{ServerAddr: "localhost:5223"})
	if err != nil {
		t.Fatal(err)
	}
	if conf.ServerName != "localhost" {
		t.Fatalf("expected ServerName localhost, got %s", conf.ServerName)
	}
	if len(conf.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(conf.Certificates))
	}
	if conf.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify false when a root CA is configured")
	}
}

func TestExecute_ConfigFileNotFound(t *testing.T) {
	Command()
	opts.config = "/nonexistent/tunnel.yml"

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	if !strings.Contains(err.Error(), "configuration error") {
		t.Fatalf("expected configuration error, got: %v", err)
	}
}

func TestExecute_StartUnknownTunnel(t *testing.T) {
	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	Command()
	opts.config = f
	opts.command = "start"
	opts.args = []string{"nonexistent-tunnel"}

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for unknown tunnel name")
	}
	if !strings.Contains(err.Error(), `no such tunnel "nonexistent-tunnel"`) {
		t.Fatalf("expected 'no such tunnel' error, got: %v", err)
	}
}

func TestExecute_IDCommand_KeyPairError(t *testing.T) {
	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	Command()
	opts.config = f
	opts.command = "id"
	opts.tlsCrt = "/nonexistent/tls.crt"
	opts.tlsKey = "/nonexistent/tls.key"

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for missing key pair")
	}
	if !strings.Contains(err.Error(), "failed to load key pair") {
		t.Fatalf("expected key pair load error, got: %v", err)
	}
}

func TestExecute_TLSConfigError(t *testing.T) {
	content := `
server_addr: 192.168.1.1:5223
tunnels:
  web:
    proto: tcp
    addr: localhost:8080
`
	f := writeTempFile(t, content)

	Command()
	opts.config = f
	opts.command = "start-all"
	opts.tlsCrt = "/nonexistent/tls.crt"
	opts.tlsKey = "/nonexistent/tls.key"

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error when tls config fails")
	}
	if !strings.Contains(err.Error(), "failed to configure tls") {
		t.Fatalf("expected tls configuration error, got: %v", err)
	}
}
