package client

import (
	"flag"
	"testing"
	"time"

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

	pf := proxy(m, nil)
	if pf == nil {
		t.Fatal("expected non-nil ProxyFunc")
	}
}
