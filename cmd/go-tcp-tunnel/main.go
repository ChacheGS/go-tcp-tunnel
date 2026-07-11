// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/ChacheGS/go-tcp-tunnel/cmd/ca"
	"github.com/ChacheGS/go-tcp-tunnel/cmd/client"
	"github.com/ChacheGS/go-tcp-tunnel/cmd/server"
)

const banner = `
   ______       ________________     __                         __
  / ____/___   /_  __/ ____/ __ \   / /___  ______  ____  ___  / /
 / / __/ __ \   / / / /   / /_/ /  / __/ / / / __ \/ __ \/ _ \/ /
/ /_/ / /_/ /  / / / /___/ ____/  / /_/ /_/ / / / / / / /  __/ /
\____/\____/  /_/  \____/_/       \__/\__,_/_/ /_/_/ /_/\___/_/

`
const version string = "v0.0.1"

const usage1 string = `Usage: go-tcp-tunnel server|client|ca [OPTIONS]
Options:
`

const usage2 string = `
Commands:
	go-tcp-tunnel server --help
	go-tcp-tunnel client --help
	go-tcp-tunnel ca --help

Author:
	ChacheGS(https://github.com/ChacheGS)

	This project is forked from go-tcp-tunnel(https://github.com/jlandowner/go-tcp-tunnel)
	written by jlandowner(https://github.com/jlandowner)

	go-tcp-tunnel itself is forked from go-http-tunnel(https://github.com/mmatczuk/go-http-tunnel)
	written by M. Matczuk (mmatczuk@gmail.com)

Bugs:
	Submit bugs to https://github.com/ChacheGS/go-tcp-tunnel/issues

`

type globalOptions struct {
	version bool
}

var o globalOptions

func main() {
	flag.CommandLine.Init("command", flag.ExitOnError)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage1)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usage2)
	}
	flag.BoolVar(&o.version, "version", false, "Prints tunnel version")
	flag.Parse()

	clientCmd := client.Command()
	serverCmd := server.Command()
	caCmd := ca.Command()

	if o.version {
		fmt.Println(version)
		return
	}
	fmt.Print(banner)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if flag.NArg() > 0 {
		args := flag.Args()
		switch args[0] {
		case "client":
			clientCmd.Parse(args[1:])
			if err := client.CompleteArgs(clientCmd); err != nil {
				clientCmd.Usage()
				fatal("%v", err)
			}

			if err := client.Execute(ctx); err != nil {
				clientCmd.Usage()
				fatal("%v", err)
			}

		case "server":
			serverCmd.Parse(args[1:])

			if err := server.Execute(ctx); err != nil {
				serverCmd.Usage()
				fatal("ERROR: %v", err)
			}
			return

		case "ca":
			caCmd.Parse(args[1:])
			if err := ca.CompleteArgs(caCmd); err != nil {
				caCmd.Usage()
				fatal("%v", err)
			}

			if err := ca.Execute(); err != nil {
				caCmd.Usage()
				fatal("ERROR: %v", err)
			}
			return
		}
	} else {
		flag.Usage()
		fatal("ERROR: neither client, server, nor ca is specified")
	}
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
