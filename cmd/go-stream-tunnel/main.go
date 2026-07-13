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
	"syscall"

	"github.com/ChacheGS/go-stream-tunnel/cmd/ca"
	"github.com/ChacheGS/go-stream-tunnel/cmd/client"
	"github.com/ChacheGS/go-stream-tunnel/cmd/server"
)

const banner = `
   ______         _____ __                               __                         __
  / ____/___     / ___// /_________  ____ _____ ___     / /___  ______  ____  ___  / /
 / / __/ __ \    \__ \/ __/ ___/ _ \/ __ \/ __ \__ \   / __/ / / / __ \/ __ \/ _ \/ / 
/ /_/ / /_/ /   ___/ / /_/ /  /  __/ /_/ / / / / / /  / /_/ /_/ / / / / / / /  __/ /  
\____/\____/   /____/\__/_/   \___/\__,_/_/ /_/ /_/   \__/\__,_/_/ /_/_/ /_/\___/_/   
                                                                                      
`
const version string = "v1.0.5"

const usage1 string = `Usage: go-stream-tunnel server|client|ca [OPTIONS]
Options:
`

const usage2 string = `
Commands:
	go-stream-tunnel server --help
	go-stream-tunnel client --help
	go-stream-tunnel ca --help

Author:
	ChacheGS(https://github.com/ChacheGS)

	This project is forked from go-tcp-tunnel(https://github.com/jlandowner/go-tcp-tunnel)
	written by jlandowner(https://github.com/jlandowner)

	go-tcp-tunnel itself is forked from go-http-tunnel(https://github.com/mmatczuk/go-http-tunnel)
	written by M. Matczuk (mmatczuk@gmail.com)

Bugs:
	Submit bugs to https://github.com/ChacheGS/go-stream-tunnel/issues

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
	fmt.Printf("%s\n\n", version)

	// SIGTERM matters as much as SIGINT here: this binary is almost always
	// PID 1 inside a container (see Dockerfile -- no init/shell wrapper),
	// and the kernel does not apply a default terminate action to PID 1
	// for signals it hasn't explicitly registered a handler for. Without
	// this, `docker stop` (which sends SIGTERM first) would be silently
	// ignored, forcing Docker to wait out its full stop grace period and
	// fall back to SIGKILL every time before the container actually exits.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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

			// Execute failures are runtime errors (dial failed, cert
			// rejected, duplicate connection, ...), not usage mistakes --
			// printing the full flag/help block here just buries the
			// actual error under noise that looks like "you typed this
			// wrong" when you didn't.
			if err := client.Execute(ctx); err != nil {
				fatal("%v", err)
			}

		case "server":
			serverCmd.Parse(args[1:])

			if err := server.Execute(ctx); err != nil {
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
				fatal("ERROR: %v", err)
			}
			return
		}
	} else {
		flag.Usage()
		fatal("ERROR: neither client, server, nor ca is specified")
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
