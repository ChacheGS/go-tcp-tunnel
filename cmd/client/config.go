// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package client

import (
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"gopkg.in/yaml.v2"

	tunnel "github.com/jlandowner/go-http-tunnel"
	"github.com/jlandowner/go-http-tunnel/proto"
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
}

// ClientConfig is a tunnel client configuration.
type ClientConfig struct {
	ServerAddr string             `yaml:"server_addr"`
	Backoff    BackoffConfig      `yaml:"backoff"`
	Tunnels    map[string]*Tunnel `yaml:"tunnels"`
}

func loadClientConfigFromFile(file string) (*ClientConfig, error) {
	buf, err := ioutil.ReadFile(file)
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

	if c.ServerAddr == "" {
		return nil, fmt.Errorf("server_addr: missing")
	}
	if c.ServerAddr, err = tunnel.NormalizeAddress(c.ServerAddr); err != nil {
		return nil, fmt.Errorf("server_addr: %s", err)
	}

	for name, t := range c.Tunnels {
		switch t.Protocol {
		case proto.TCP, proto.TCP4, proto.TCP6:
			if err := completeTCP(t); err != nil {
				return nil, fmt.Errorf("%s %s", name, err)
			}
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
