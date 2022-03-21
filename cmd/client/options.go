// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package client

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	backoff "github.com/cenkalti/backoff/v4"
	tunnel "github.com/jlandowner/go-http-tunnel"
	"github.com/jlandowner/go-http-tunnel/id"
	"github.com/jlandowner/go-http-tunnel/log"
	"github.com/jlandowner/go-http-tunnel/proto"
	"gopkg.in/yaml.v2"
)

const usage1 string = `Usage: tcptunnel client [OPTIONS] <command> [command args] [...]
options:
`

const usage2 string = `
Commands:
	tcptunnel client id                      Show client identifier
	tcptunnel client list                    List tunnel names from config file
	tcptunnel client start [tunnel] [...]    Start tunnels by name from config file
	tcptunnel client start-all               Start all tunnels defined in config file

Examples:
	tcptunnel client start www ssh
	tcptunnel client -config config.yaml -log-level 2 start ssh
	tcptunnel client start-all

config.yaml:
	server_addr: SERVER_IP:5223
	tunnels:
	  ssh:
	    proto: tcp
	    addr: 192.168.0.5:22
	    remote_addr: 0.0.0.0:22
	  www:
	    proto: tcp
	    addr: 192.168.0.5:80
	    remote_addr: 0.0.0.0:80

`

type options struct {
	config  string
	tlsCrt  string
	tlsKey  string
	rootCA  string
	command string
	args    []string
}

var opts options

func Command() *flag.FlagSet {
	cmd := flag.NewFlagSet("client", flag.ExitOnError)
	cmd.Usage = func() {
		fmt.Fprint(os.Stderr, usage1)
		cmd.PrintDefaults()
		fmt.Fprint(os.Stderr, usage2)
	}

	cmd.StringVar(&opts.config, "config", "tunnel.yml", "Path to tunnel configuration file")
	cmd.StringVar(&opts.tlsCrt, "tls-crt", "tls.crt", "Path to a TLS certificate file")
	cmd.StringVar(&opts.tlsKey, "tls-key", "tls.key", "Path to a TLS key file")
	cmd.StringVar(&opts.rootCA, "ca-crt", "tls.crt", "Path to the trusted certificate chain used for server certificate authentication")

	return cmd
}

func Execute(logLevel int) error {
	logger := log.NewFilterLogger(log.NewStdLogger(), logLevel)

	// read configuration file
	config, err := loadClientConfigFromFile(opts.config)
	if err != nil {
		return fmt.Errorf("configuration error: %s", err)
	}

	switch opts.command {
	case "id":
		cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
		if err != nil {
			return fmt.Errorf("failed to load key pair: %s", err)
		}
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %s", err)
		}
		fmt.Println(id.New(x509Cert.Raw))

		return nil
	case "list":
		var names []string
		for n := range config.Tunnels {
			names = append(names, n)
		}

		sort.Strings(names)

		for _, n := range names {
			fmt.Println(n)
		}

		return nil
	case "start":
		tunnels := make(map[string]*Tunnel)
		for _, arg := range opts.args {
			t, ok := config.Tunnels[arg]
			if !ok {
				return fmt.Errorf("no such tunnel %q", arg)
			}
			tunnels[arg] = t
		}
		config.Tunnels = tunnels
	default:
		panic("unexpected command")
	}

	if len(config.Tunnels) == 0 {
		return fmt.Errorf("no tunnels")
	}

	tlsconf, err := tlsConfig()
	if err != nil {
		return fmt.Errorf("failed to configure tls: %s", err)
	}

	b, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to dump config: %s", err)
	}
	logger.Log("config", string(b))

	client, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      config.ServerAddr,
		TLSClientConfig: tlsconf,
		Backoff:         expBackoff(config.Backoff),
		Tunnels:         tunnels(config.Tunnels),
		Proxy:           proxy(config.Tunnels, logger),
		Logger:          logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %s", err)
	}

	return client.Start()
}

func tlsConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
	if err != nil {
		return nil, err
	}

	if opts.rootCA == "" {
		return nil, fmt.Errorf("no root CA is given")
	}

	roots := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(opts.rootCA)
	if err != nil {
		return nil, err
	}
	if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
		return nil, err
	}

	// host, _, err := net.SplitHostPort(config.ServerAddr)
	// if err != nil {
	// 	return nil, err
	// }

	return &tls.Config{
		// ServerName:         host,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: roots == nil,
		RootCAs:            roots,
	}, nil
}

func expBackoff(c BackoffConfig) *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = c.Interval
	b.Multiplier = c.Multiplier
	b.MaxInterval = c.MaxInterval
	b.MaxElapsedTime = c.MaxTime

	return b
}

func tunnels(m map[string]*Tunnel) map[string]*proto.Tunnel {
	p := make(map[string]*proto.Tunnel)

	for name, t := range m {
		p[name] = &proto.Tunnel{
			Protocol: t.Protocol,
			Addr:     t.RemoteAddr,
		}
	}

	return p
}

func proxy(m map[string]*Tunnel, logger log.Logger) tunnel.ProxyFunc {
	tcpAddr := make(map[string]string)

	for _, t := range m {
		switch t.Protocol {
		case proto.TCP, proto.TCP4, proto.TCP6:
			tcpAddr[t.RemoteAddr] = t.Addr
		}
	}

	return tunnel.Proxy(tunnel.ProxyFuncs{
		TCP: tunnel.NewMultiTCPProxy(tcpAddr, log.NewContext(logger).WithPrefix("proxy", "TCP")).Proxy,
	})
}
