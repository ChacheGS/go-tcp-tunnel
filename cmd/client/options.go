// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"

	tunnel "github.com/ChacheGS/go-stream-tunnel"
	"github.com/ChacheGS/go-stream-tunnel/id"
	"github.com/ChacheGS/go-stream-tunnel/log"
	"github.com/ChacheGS/go-stream-tunnel/proto"
	backoff "github.com/cenkalti/backoff/v4"
	"gopkg.in/yaml.v2"
)

const usage1 string = `Usage: go-stream-tunnel client [OPTIONS] <command> [command args] [...]
options:
`

const usage2 string = `
Commands:
	go-stream-tunnel client id                      Show client identifier
	go-stream-tunnel client list                    List tunnel names from config file
	go-stream-tunnel client start [tunnel] [...]    Start tunnels by name from config file
	go-stream-tunnel client start-all               Start all tunnels defined in config file

Examples:
	go-stream-tunnel client start www ssh
	go-stream-tunnel client -config config.yaml -log-level 2 start ssh
	go-stream-tunnel client start-all

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
	  myapp:
	    proto: http
	    addr: localhost:8080

`

type options struct {
	config    string
	tlsCrt    string
	tlsKey    string
	rootCA    string
	tlsCrtSet bool
	tlsKeySet bool
	rootCASet bool
	command   string
	args      []string
	logLevel  int
}

var opts options

func Command() *flag.FlagSet {
	cmd := flag.NewFlagSet("client", flag.ExitOnError)
	cmd.Usage = func() {
		fmt.Fprint(os.Stderr, usage1)
		cmd.PrintDefaults()
		fmt.Fprint(os.Stderr, usage2)
	}

	opts.tlsCrtSet = false
	opts.tlsKeySet = false
	opts.rootCASet = false

	cmd.StringVar(&opts.config, "config", "tunnel.yml", "Path to tunnel configuration file")
	cmd.StringVar(&opts.tlsCrt, "tls-crt", "tls.crt", "Path to a TLS certificate file; falls back to tls_crt in the config file if not set")
	cmd.StringVar(&opts.tlsKey, "tls-key", "tls.key", "Path to a TLS key file; falls back to tls_key in the config file if not set")
	cmd.StringVar(&opts.rootCA, "ca-crt", "ca.crt", "Path to the trusted certificate chain used for server certificate authentication; falls back to ca_crt in the config file if not set")
	cmd.IntVar(&opts.logLevel, "log-level", 1, "Level of messages to log, 0-3")

	return cmd
}

// resolvePath returns flagVal if the corresponding flag was passed
// explicitly on the command line (flagSet), otherwise configVal if the
// config file set one, otherwise flagVal -- which still holds the flag's
// own built-in default when neither of the above applies.
func resolvePath(flagVal string, flagSet bool, configVal string) string {
	if flagSet || configVal == "" {
		return flagVal
	}
	return configVal
}

func CompleteArgs(fs *flag.FlagSet) error {
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "tls-crt":
			opts.tlsCrtSet = true
		case "tls-key":
			opts.tlsKeySet = true
		case "ca-crt":
			opts.rootCASet = true
		}
	})

	opts.command = fs.Arg(0)
	switch opts.command {
	case "id", "list":
		opts.args = fs.Args()[1:]
		if len(opts.args) > 0 {
			return fmt.Errorf("list takes no arguments")
		}
	case "start":
		opts.args = fs.Args()[1:]
		if len(opts.args) == 0 {
			return fmt.Errorf("you must specify at least one tunnel to start")
		}
	case "start-all":
		opts.args = fs.Args()[1:]
		if len(opts.args) > 0 {
			return fmt.Errorf("start-all takes no arguments")
		}
	default:
		return fmt.Errorf("unknown command %q", opts.command)
	}
	return nil
}

func Execute(ctx context.Context) error {
	logger := log.NewFilterLogger(log.NewStdLogger(), opts.logLevel)

	// read configuration file
	config, err := loadClientConfigFromFile(opts.config)
	if err != nil {
		return fmt.Errorf("configuration error: %s", err)
	}

	opts.tlsCrt = resolvePath(opts.tlsCrt, opts.tlsCrtSet, config.TLSCrt)
	opts.tlsKey = resolvePath(opts.tlsKey, opts.tlsKeySet, config.TLSKey)
	opts.rootCA = resolvePath(opts.rootCA, opts.rootCASet, config.CACrt)

	switch opts.command {
	case "id":
		if err := tunnel.CheckPrivateKeyPermissions(opts.tlsKey); err != nil {
			return fmt.Errorf("failed to load key pair: %s", err)
		}
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
	}

	if len(config.Tunnels) == 0 {
		return fmt.Errorf("no tunnels")
	}

	tlsconf, err := tlsConfig(config)
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

	return client.Start(ctx)
}

func tlsConfig(config *ClientConfig) (*tls.Config, error) {
	if err := tunnel.CheckPrivateKeyPermissions(opts.tlsKey); err != nil {
		return nil, err
	}

	cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
	if err != nil {
		return nil, err
	}

	if opts.rootCA == "" {
		return nil, fmt.Errorf("no root CA is given")
	}

	roots := x509.NewCertPool()
	rootPEM, err := os.ReadFile(opts.rootCA)
	if err != nil {
		return nil, err
	}
	if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
		return nil, fmt.Errorf("failed to parse CA certificate PEM")
	}

	host, _, err := net.SplitHostPort(config.ServerAddr)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		ServerName:         host,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: len(roots.Subjects()) == 0,
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
		pt := &proto.Tunnel{
			Protocol: t.Protocol,
			Addr:     t.RemoteAddr,
		}
		if t.Protocol == proto.HTTP {
			pt.Host = t.Subdomain
		}
		p[name] = pt
	}

	return p
}

func proxy(m map[string]*Tunnel, logger log.Logger) tunnel.ProxyFunc {
	tcpAddr := make(map[string]string)

	for _, t := range m {
		switch t.Protocol {
		case proto.TCP, proto.TCP4, proto.TCP6:
			tcpAddr[t.RemoteAddr] = t.Addr
		case proto.HTTP:
			tcpAddr[t.Subdomain] = t.Addr
		}
	}

	return tunnel.Proxy(tunnel.ProxyFuncs{
		Stream: tunnel.NewMultiStreamProxy(tcpAddr, log.NewContext(logger).WithPrefix("proxy", "stream")).Proxy,
	})
}
