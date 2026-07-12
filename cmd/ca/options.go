// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tunnel "github.com/ChacheGS/go-stream-tunnel"
	capki "github.com/ChacheGS/go-stream-tunnel/ca"
	"github.com/ChacheGS/go-stream-tunnel/id"
)

const usage1 string = `Usage: go-stream-tunnel ca <command> [OPTIONS]
options:
`

const usage2 string = `
Commands:
	go-stream-tunnel ca [-ca-dir ./ca] init
	go-stream-tunnel ca -name <label> [-addr <host>] [-ca-dir ./ca] [-out-dir ...] issue

Note: flags must come before the command (init/issue), not after.

Examples:
	go-stream-tunnel ca init
	go-stream-tunnel ca -name laptop issue
	go-stream-tunnel ca -name server -addr tunnel.example.com issue

`

type options struct {
	caDir   string
	name    string
	addr    string
	outDir  string
	command string
}

var opts options

func Command() *flag.FlagSet {
	cmd := flag.NewFlagSet("ca", flag.ExitOnError)
	cmd.Usage = func() {
		fmt.Fprint(os.Stderr, usage1)
		cmd.PrintDefaults()
		fmt.Fprint(os.Stderr, usage2)
	}

	cmd.StringVar(&opts.caDir, "ca-dir", "ca", "Directory holding (or to write) the CA's ca.crt/ca.key")
	cmd.StringVar(&opts.name, "name", "", "Name for the issued certificate (its CommonName); required for 'issue'")
	cmd.StringVar(&opts.addr, "addr", "", "DNS name or IP address to include as a Subject Alternative Name; only needed for a server-role cert")
	cmd.StringVar(&opts.outDir, "out-dir", "", "Directory to write the issued tls.crt/tls.key to; defaults to ./<name>")

	return cmd
}

func CompleteArgs(fs *flag.FlagSet) error {
	opts.command = fs.Arg(0)
	switch opts.command {
	case "init":
		if len(fs.Args()) > 1 {
			return fmt.Errorf("init takes no arguments")
		}
	case "issue":
		if opts.name == "" {
			return fmt.Errorf("issue requires -name")
		}
		if opts.outDir == "" {
			opts.outDir = opts.name
		}
	default:
		return fmt.Errorf("unknown command %q", opts.command)
	}
	return nil
}

const validityYears = 10

func validity() time.Duration {
	return validityYears * 365 * 24 * time.Hour
}

// writeKeyPair writes certPEM to crtPath (mode 0644) then keyPEM to keyPath
// (mode 0600). If writing the key fails after the cert was already
// written, the cert is removed, so a failed init/issue doesn't leave a
// half-written, keyless certificate behind.
func writeKeyPair(crtPath, keyPath string, certPEM, keyPEM []byte) error {
	if err := os.WriteFile(crtPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %s", crtPath, err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		os.Remove(crtPath)
		return fmt.Errorf("failed to write %s: %s", keyPath, err)
	}
	return nil
}

func Execute() error {
	switch opts.command {
	case "init":
		return executeInit()
	case "issue":
		return executeIssue()
	}
	return fmt.Errorf("unknown command %q", opts.command)
}

func executeInit() error {
	caCrtPath := filepath.Join(opts.caDir, "ca.crt")
	caKeyPath := filepath.Join(opts.caDir, "ca.key")

	if _, err := os.Stat(caCrtPath); err == nil {
		return fmt.Errorf("CA already exists at %s; refusing to overwrite your root of trust", caCrtPath)
	}
	if _, err := os.Stat(caKeyPath); err == nil {
		return fmt.Errorf("CA already exists at %s; refusing to overwrite your root of trust", caKeyPath)
	}

	certPEM, keyPEM, err := capki.GenerateCA("go-stream-tunnel CA", validity())
	if err != nil {
		return fmt.Errorf("failed to generate CA: %s", err)
	}

	if err := os.MkdirAll(opts.caDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %s", opts.caDir, err)
	}
	if err := writeKeyPair(caCrtPath, caKeyPath, certPEM, keyPEM); err != nil {
		return err
	}

	fmt.Printf("CA created:\n  %s\n  %s\n", caCrtPath, caKeyPath)
	return nil
}

func executeIssue() error {
	caCrtPath := filepath.Join(opts.caDir, "ca.crt")
	caKeyPath := filepath.Join(opts.caDir, "ca.key")

	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s (run 'go-stream-tunnel ca init' first)", caCrtPath, err)
	}

	if err := tunnel.CheckPrivateKeyPermissions(caKeyPath); err != nil {
		return fmt.Errorf("failed to read %s: %s", caKeyPath, err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %s (run 'go-stream-tunnel ca init' first)", caKeyPath, err)
	}

	var sans []string
	if opts.addr != "" {
		sans = []string{opts.addr}
	}

	certPEM, keyPEM, err := capki.IssueCert(caCertPEM, caKeyPEM, opts.name, sans, validity())
	if err != nil {
		return fmt.Errorf("failed to issue certificate: %s", err)
	}

	crtPath := filepath.Join(opts.outDir, "tls.crt")
	keyPath := filepath.Join(opts.outDir, "tls.key")

	if _, err := os.Stat(crtPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", crtPath)
	}
	if _, err := os.Stat(keyPath); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite", keyPath)
	}

	if err := os.MkdirAll(opts.outDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %s", opts.outDir, err)
	}
	if err := writeKeyPair(crtPath, keyPath, certPEM, keyPEM); err != nil {
		return err
	}

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse issued certificate: %s", err)
	}
	fingerprint := id.New(cert.Raw)

	fmt.Printf("Certificate issued:\n  %s\n  %s\n\nClient ID (for -client-ids): %s\n", crtPath, keyPath, fingerprint)
	return nil
}
