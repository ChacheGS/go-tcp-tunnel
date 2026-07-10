package client

import (
	"bytes"
	"flag"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jlandowner/go-tcp-tunnel/log"
	"github.com/jlandowner/go-tcp-tunnel/proto"
)

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
