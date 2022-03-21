// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jlandowner/go-http-tunnel/cmd/client"
	"github.com/jlandowner/go-http-tunnel/cmd/server"
)

const banner = `
  ________________     __                         __
 /_  __/ ____/ __ \   / /___  ______  ____  ___  / /
  / / / /   / /_/ /  / __/ / / / __ \/ __ \/ _ \/ /
 / / / /___/ ____/  / /_/ /_/ / / / / / / /  __/ /
/_/  \____/_/       \__/\__,_/_/ /_/_/ /_/\___/_/

`
const version string = "snapshot"

const usage1 string = `Usage: tcptunnel server|client [OPTIONS]
Options:
`

const usage2 string = `
Example:
	tcptunnel server --help
	tcptunnel client --help

Author:
	jlandowner(https://github.com/jlandowner)
	
	Original go-http-tunnel(https://github.com/mmatczuk/go-http-tunnel) is written by M. Matczuk (mmatczuk@gmail.com)

Bugs:
	Submit bugs to https://github.com/jlandowner/tcptunnel/issues

`

type globalOptions struct {
	logLevel int
	version  bool
}

var o globalOptions

func main() {
	flag.CommandLine.Init("command", flag.ExitOnError)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage1)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, usage2)
	}
	flag.IntVar(&o.logLevel, "log-level", 1, "Level of messages to log, 0-3")
	flag.BoolVar(&o.version, "version", false, "Prints tunnel version")
	flag.Parse()

	clientCmd := client.Command()
	serverCmd := server.Command()

	if o.version {
		fmt.Println(version)
		return
	}
	fmt.Print(banner)

	if flag.NArg() > 0 {
		args := flag.Args()
		switch args[0] {
		case "client":
			clientCmd.Parse(args[1:])

			if err := client.Execute(o.logLevel); err != nil {
				clientCmd.Usage()
				fatal("%v", err)
			}

		case "server":
			serverCmd.Parse(args[1:])

			if err := server.Execute(o.logLevel); err != nil {
				serverCmd.Usage()
				fatal("ERROR: %v", err)
			}
		}
	} else {
		flag.Usage()
		fatal("ERROR: nor client or server is specified")
	}
}

func fatal(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprint(os.Stderr, "\n")
	os.Exit(1)
}
