# TCP tunnel [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](https://pkg.go.dev/github.com/jlandowner/tcptunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/jlandowner/tcptunnel)](https://goreportcard.com/report/github.com/jlandowner/tcptunnel) [![Github All Releases](https://img.shields.io/github/downloads/jlandowner/tcptunnel/total.svg)](https://github.com/jlandowner/tcptunnel/releases)

TCP tunnel is a TCP reverse tunnel proxy to expose your private backends behind a firewall to the public.
The reverse tunnel is based on HTTP/2 with mutual TLS (mTLS). It enables you to share your localhost when you don't have a public IP.

Features:

* TCP proxy
* Secure tunnel
* Dynamic listeners on server by client commands

Common use cases:

* Exposing your local server behind a firewall to the public
* Hosting a game server from home
* Developing webhook integrations

## Getting started

TODO

## Client configuration

The tunnel Client requires configuration file, by default it will try reading `tunnel.yml` in your current working directory. If you want to specify other file use `-config` flag.

Server do not have any configurations without TLS.
But Client configuration is propagated to the Server and it configures the server to create TCP listeners and proxies dynamically.

Here is a sample configuration:

```yaml
server_addr: SERVER_IP:5223
tunnels:
  ssh:
    proto: tcp
    addr: 192.168.0.5:22
  www:
    proto: tcp
    addr: localhost:8080
    remote_addr: 80
```

This creates 2 tunnels:

* Server exposes port 22, which proxies to the Client local address `192.168.0.5:22`
* Server exposes port 80, which proxies to the Client local address `localhost:8080`

Configuration options:

* `server_addr`: server's tunnel listener TCP address, i.e. `54.12.12.45:5223`. default port is `5223`
* `tunnels / [name]`
    * `proto`: proxy listener protocol, currently only `tcp` can be set
    * `addr`: forward traffic to this local port number or network address, i.e. `localhost:22`
    * `remote_addr`: server listener TCP address, *default:* `same as local port`
* `backoff`
    * `interval`: how long client would wait before redialing the server if connection was lost, exponential backoff initial interval, *default:* `500ms`
    * `multiplier`: interval multiplier if reconnect failed, *default:* `1.5`
    * `max_interval`: maximal time client would wait before redialing the server, *default:* `1m`
    * `max_time`: maximal time client would try to reconnect to the server if connection was lost, set `0` to never stop trying, *default:* `15m`

## How it works

A client opens TLS connection to a server. The server accepts connections from known clients only. The client is recognized by its TLS certificate ID. The server is publicly available and proxies incoming connections to the client. Then the connection is further proxied in the client's network.

The tunnel is based HTTP/2 for speed and security. There is a single TCP connection between client and server and all the proxied connections are multiplexed using HTTP/2.

## License

Copyright (C) 2017 Michał Matczuk
Copyright (C) 2022 jlandowner

This project is distributed under the AGPL-3 license. See the [LICENSE](https://github.com/jlandowner/tcptunnel/blob/master/LICENSE) file for details. If you need an enterprice license contact me directly.
