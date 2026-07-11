// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	capki "github.com/ChacheGS/go-tcp-tunnel/ca"
)

const usage1 string = `Usage: go-tcp-tunnel ca <command> [OPTIONS]
options:
`

const usage2 string = `
Commands:
	go-tcp-tunnel ca init [-ca-dir ./ca]
	go-tcp-tunnel ca issue -name <label> [-addr <host>] [-ca-dir ./ca] [-out-dir ...]

Examples:
	go-tcp-tunnel ca init
	go-tcp-tunnel ca issue -name laptop
	go-tcp-tunnel ca issue -name server -addr tunnel.example.com

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

	certPEM, keyPEM, err := capki.GenerateCA("go-tcp-tunnel CA", validity())
	if err != nil {
		return fmt.Errorf("failed to generate CA: %s", err)
	}

	if err := os.MkdirAll(opts.caDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %s", opts.caDir, err)
	}
	if err := os.WriteFile(caCrtPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %s", caCrtPath, err)
	}
	if err := os.WriteFile(caKeyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %s", caKeyPath, err)
	}

	fmt.Printf("CA created:\n  %s\n  %s\n", caCrtPath, caKeyPath)
	return nil
}

func executeIssue() error {
	return fmt.Errorf("not implemented yet")
}
