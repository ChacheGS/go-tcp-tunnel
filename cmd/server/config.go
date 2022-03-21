// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v2"

	tunnel "github.com/jlandowner/go-http-tunnel"
)

// ServerConfig is a tunnel server configuration.
type ServerConfig struct {
	// Addr is TCP address to listen for client connections. If empty ":0"
	// is used.
	Addr      string
	TLSCrt    string   `yaml:"tls_crt"`
	TLSKey    string   `yaml:"tls_key"`
	ClientCA  string   `yaml:"client_ca"`
	ClientIDs []string `yaml:"client_ids"`
}

func LoadServerConfigFromFile(file string) (*ServerConfig, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %s", file, err)
	}

	c := ServerConfig{
		TLSCrt:   filepath.Join(filepath.Dir(file), "tls.crt"),
		TLSKey:   filepath.Join(filepath.Dir(file), "tls.key"),
		ClientCA: filepath.Join(filepath.Dir(file), "ca.crt"),
	}

	if err = yaml.Unmarshal(buf, &c); err != nil {
		return nil, fmt.Errorf("failed to parse file %q: %s", file, err)
	}

	if c.Addr == "" {
		return nil, fmt.Errorf("addr: missing")
	}
	if c.Addr, err = tunnel.NormalizeAddress(c.Addr); err != nil {
		return nil, fmt.Errorf("addr: %s", err)
	}

	return &c, nil
}
