// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"

	tunnel "github.com/ChacheGS/go-stream-tunnel"
	"github.com/ChacheGS/go-stream-tunnel/proto"
)

// Default backoff configuration.
const (
	DefaultBackoffInterval    = 500 * time.Millisecond
	DefaultBackoffMultiplier  = 1.5
	DefaultBackoffMaxInterval = 60 * time.Second
	DefaultBackoffMaxTime     = 15 * time.Minute
)

// BackoffConfig defines behavior of staggering reconnection retries.
type BackoffConfig struct {
	Interval    time.Duration `yaml:"interval"`
	Multiplier  float64       `yaml:"multiplier"`
	MaxInterval time.Duration `yaml:"max_interval"`
	MaxTime     time.Duration `yaml:"max_time"`
}

// Tunnel defines a tunnel.
type Tunnel struct {
	Protocol   string `yaml:"proto,omitempty"`
	Addr       string `yaml:"addr,omitempty"`
	RemoteAddr string `yaml:"remote_addr,omitempty"`
	// Subdomain is used by proto "http" tunnels: the tunnel becomes
	// reachable at <Subdomain>.<server's base domain>. Defaults to the
	// tunnel's own key in the tunnels map if omitted.
	Subdomain string `yaml:"subdomain,omitempty"`
}

// ClientConfig is a tunnel client configuration.
type ClientConfig struct {
	ServerAddr string `yaml:"server_addr"`
	// TLSCrt, TLSKey, and CACrt are fallbacks used only when the
	// corresponding -tls-crt/-tls-key/-ca-crt flag isn't passed explicitly
	// on the command line. A relative path here resolves against this
	// config file's own directory (not the process's working directory),
	// so a config file and the certs it references can be moved together.
	TLSCrt  string             `yaml:"tls_crt,omitempty"`
	TLSKey  string             `yaml:"tls_key,omitempty"`
	CACrt   string             `yaml:"ca_crt,omitempty"`
	Backoff BackoffConfig      `yaml:"backoff"`
	Tunnels map[string]*Tunnel `yaml:"tunnels"`
}

// resolveConfigPath returns p unchanged if it's empty or already absolute;
// otherwise it's resolved relative to the directory containing configFile.
func resolveConfigPath(configFile, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(configFile), p)
}

func loadClientConfigFromFile(file string) (*ClientConfig, error) {
	buf, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %s", file, err)
	}

	c := ClientConfig{
		Backoff: BackoffConfig{
			Interval:    DefaultBackoffInterval,
			Multiplier:  DefaultBackoffMultiplier,
			MaxInterval: DefaultBackoffMaxInterval,
			MaxTime:     DefaultBackoffMaxTime,
		},
	}

	if err = yaml.Unmarshal(buf, &c); err != nil {
		return nil, fmt.Errorf("failed to parse file %q: %s", file, err)
	}

	c.TLSCrt = resolveConfigPath(file, c.TLSCrt)
	c.TLSKey = resolveConfigPath(file, c.TLSKey)
	c.CACrt = resolveConfigPath(file, c.CACrt)

	if c.ServerAddr == "" {
		return nil, fmt.Errorf("server_addr: missing")
	}
	if c.ServerAddr, err = tunnel.NormalizeAddress(c.ServerAddr); err != nil {
		return nil, fmt.Errorf("server_addr: %s", err)
	}

	subdomains := make(map[string]string)
	for name, t := range c.Tunnels {
		switch t.Protocol {
		case proto.TCP, proto.TCP4, proto.TCP6:
			if err := completeTCP(t); err != nil {
				return nil, fmt.Errorf("%s %s", name, err)
			}
		case proto.HTTP:
			if err := completeHTTP(t, name); err != nil {
				return nil, fmt.Errorf("%s %s", name, err)
			}
			// Two tunnels sharing a subdomain would silently overwrite
			// each other's local target in the proxy's dial-target map,
			// with no error surfaced at connection time.
			if other, ok := subdomains[t.Subdomain]; ok {
				return nil, fmt.Errorf("%s and %s: subdomain %q used by more than one tunnel", other, name, t.Subdomain)
			}
			subdomains[t.Subdomain] = name
		default:
			return nil, fmt.Errorf("%s invalid protocol %q", name, t.Protocol)
		}
	}

	return &c, nil
}

func completeTCP(t *Tunnel) error {
	var err error
	if t.Addr == "" {
		return fmt.Errorf("addr: missing")
	}
	if t.Addr, err = tunnel.NormalizeAddress(t.Addr); err != nil {
		return fmt.Errorf("addr: %s", err)
	}

	if t.RemoteAddr == "" {
		_, port, err := net.SplitHostPort(t.Addr)
		if err != nil {
			return fmt.Errorf("addr: %s", err)
		}
		t.RemoteAddr = fmt.Sprintf("0.0.0.0:%s", port)
	}
	if t.RemoteAddr, err = tunnel.NormalizeAddress(t.RemoteAddr); err != nil {
		return fmt.Errorf("remote_addr: %s", err)
	}

	return nil
}

// completeHTTP validates and fills in defaults for an http tunnel. name is
// the tunnel's own key in the tunnels map, used as the default subdomain
// when one isn't given explicitly, so a config doesn't have to repeat the
// same name twice (tunnels: myapp: proto: http ... rather than also writing
// subdomain: myapp).
func completeHTTP(t *Tunnel, name string) error {
	var err error
	if t.Addr == "" {
		return fmt.Errorf("addr: missing")
	}
	if t.Addr, err = tunnel.NormalizeAddress(t.Addr); err != nil {
		return fmt.Errorf("addr: %s", err)
	}

	defaulted := false
	if t.Subdomain == "" {
		t.Subdomain = name
		defaulted = true
	}
	if !proto.ValidSubdomainLabel(t.Subdomain) {
		if defaulted {
			return fmt.Errorf("subdomain: tunnel name %q is not a valid DNS label (lowercase letters, digits, hyphens only, no leading/trailing hyphen); add an explicit subdomain field to override it", t.Subdomain)
		}
		return fmt.Errorf("subdomain: %q is not a valid DNS label (lowercase letters, digits, hyphens only, no leading/trailing hyphen)", t.Subdomain)
	}
	if t.RemoteAddr != "" {
		return fmt.Errorf("remote_addr: not supported for proto http, use subdomain instead")
	}

	return nil
}
