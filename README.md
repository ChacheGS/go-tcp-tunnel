# Go TCP tunnel
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](https://pkg.go.dev/github.com/ChacheGS/go-tcp-tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/ChacheGS/go-tcp-tunnel)](https://goreportcard.com/report/github.com/ChacheGS/go-tcp-tunnel) [![Container](http://img.shields.io/badge/container-ready-orange.svg)](https://github.com/ChacheGS/go-tcp-tunnel/pkgs/container/go-tcp-tunnel)

Go TCP tunnel is a TCP reverse tunnel proxy to expose your local backends behind a firewall to the public.
The reverse tunnel is based on HTTP/2 with mutual TLS (mTLS). It enables you to share your localhost when you don't have a public IP.

Features:

* Easily expose a local server to the public
* Secure TCP tunnel
* Dynamic listeners on server by client commands

Common use cases:

* Exposing your local server behind a firewall to the public
* Hosting a game server from home
* Developing webhook integrations

> NOTE:
> 
> This project is forked from https://github.com/mmatczuk/go-http-tunnel
> 
> Here are some of the updates from the original
> * Focus on TCP proxy
> * Package as Docker image and support Kubernetes
> * Update Go version
> * Remove some old dependencies
>
> This repository is activly maintained 


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

### Subdomain-routed HTTP tunnels

Instead of a dedicated public port, a tunnel can request a subdomain:

```yaml
server_addr: SERVER_IP:5223
tunnels:
  myapp:
    proto: http
    addr: localhost:8080
    subdomain: myapp
```

This requires the server to be started with `-base-domain tunnel.example.com`
(and `-http-addr`, default `127.0.0.1:9000`) and a reverse proxy in front of
the server holding a wildcard TLS certificate for `*.tunnel.example.com`,
forwarding all traffic for that vhost to the server's `-http-addr`. The
tunnel then becomes reachable at `https://myapp.tunnel.example.com` — no
infrastructure changes needed to add further tunnels, only `tunnels.yaml`.

Example Caddyfile (ACME/DNS-01 provider config omitted, see Caddy's docs for
your DNS provider):

```
*.tunnel.example.com {
    tls {
        dns <provider> ...
    }
    reverse_proxy 127.0.0.1:9000 {
        transport http {
            keepalive -1s
        }
    }
}
```

The `keepalive -1s` disables backend connection reuse, which matters because
the server routes each connection to a tunnel once, at accept time, based on
its `Host` header — a connection reused across different subdomains would be
routed incorrectly. WebSocket connections are unaffected either way, since
Caddy takes an upgraded connection out of its reuse pool automatically.

## How it works

A client opens TLS connection to a server. The server accepts connections from known clients only. The client is recognized by its TLS certificate ID. The server is publicly available and proxies incoming connections to the client. Then the connection is further proxied in the client's network.

The tunnel is based HTTP/2 for speed and security. There is a single TCP connection between client and server and all the proxied connections are multiplexed using HTTP/2.

## License

Copyright (C) 2017 Michał Matczuk

Copyright (C) 2022 jlandowner

Copyright (C) 2026 ChacheGS

This project is distributed under the AGPL-3 license. See the [LICENSE](https://github.com/ChacheGS/go-tcp-tunnel/blob/master/LICENSE) file for details. If you need an enterprice license contact me directly.
