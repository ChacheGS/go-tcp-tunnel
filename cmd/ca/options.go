// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"flag"
	"fmt"
	"os"
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
