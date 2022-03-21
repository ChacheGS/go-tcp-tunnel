// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package server

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	tunnel "github.com/jlandowner/go-http-tunnel"
	"github.com/jlandowner/go-http-tunnel/id"
	"github.com/jlandowner/go-http-tunnel/log"
)

const usage1 string = `Usage: tcptunnel server [OPTIONS]
options:
`

const usage2 string = `
Example:
	tcptunnel server
	tcptunnel server -clients YMBKT3V-ESUTZ2Z-7MRILIJ-T35FHGO-D2DHO7D-FXMGSSR-V4LBSZX-BNDONQ4
	tcptunnel server -client-ca client_root.crt -tls-crt server.crt -tls-key server.key

`

func init() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage1)
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, usage2)
	}
}

// options specify arguments read command line arguments.
type options struct {
	tunnelAddr string
	tlsCrt     string
	tlsKey     string
	clientCA   string
	clientIDs  string
}

var opts options

func Command() *flag.FlagSet {
	cmd := flag.NewFlagSet("server", flag.ExitOnError)
	cmd.Usage = func() {
		fmt.Fprint(os.Stderr, usage1)
		cmd.PrintDefaults()
		fmt.Fprint(os.Stderr, usage2)
	}

	cmd.StringVar(&opts.tunnelAddr, "addr", ":5223", "Public address listening for tunnel client")
	cmd.StringVar(&opts.tlsCrt, "tls-crt", "tls.crt", "Path to a TLS certificate file")
	cmd.StringVar(&opts.tlsKey, "tls-key", "tls.key", "Path to a TLS key file")
	cmd.StringVar(&opts.clientCA, "ca-crt", "tls.crt", "Path to the trusted certificate chain used for client certificate authentication")
	cmd.StringVar(&opts.clientIDs, "client-ids", "", "Comma-separated list of tunnel client ids, if empty accept all clients with valid client certificate")

	return cmd
}

func Execute(logLevel int) error {
	logger := log.NewFilterLogger(log.NewStdLogger(), logLevel)

	tlsconf, err := tlsConfig()
	if err != nil {
		return fmt.Errorf("failed to configure tls: %s", err)
	}

	autoSubscribe := opts.clientIDs == ""

	// setup server
	server, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          opts.tunnelAddr,
		AutoSubscribe: autoSubscribe,
		TLSConfig:     tlsconf,
		Logger:        logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %s", err)
	}

	if !autoSubscribe {
		for _, c := range strings.Split(opts.clientIDs, ",") {
			if c == "" {
				return fmt.Errorf("empty client id")
			}
			identifier := id.ID{}
			err := identifier.UnmarshalText([]byte(c))
			if err != nil {
				return fmt.Errorf("invalid identifier %q: %s", c, err)
			}
			server.Subscribe(identifier)
		}
	}

	return server.Start()
}

func tlsConfig() (*tls.Config, error) {
	// load certs
	cert, err := tls.LoadX509KeyPair(opts.tlsCrt, opts.tlsKey)
	if err != nil {
		return nil, err
	}

	// load root CA for client authentication
	if opts.clientCA == "" {
		return nil, fmt.Errorf("no client CA is given")
	}

	roots := x509.NewCertPool()
	rootPEM, err := ioutil.ReadFile(opts.clientCA)
	if err != nil {
		return nil, err
	}
	if ok := roots.AppendCertsFromPEM(rootPEM); !ok {
		return nil, err
	}
	clientAuth := tls.RequireAndVerifyClientCert

	return &tls.Config{
		Certificates:           []tls.Certificate{cert},
		ClientAuth:             clientAuth,
		ClientCAs:              roots,
		SessionTicketsDisabled: true,
		MinVersion:             tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}, nil
}
