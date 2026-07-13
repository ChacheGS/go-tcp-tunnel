# Go Stream Tunnel
[![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg)](https://pkg.go.dev/github.com/ChacheGS/go-stream-tunnel) [![Go Report Card](https://goreportcard.com/badge/github.com/ChacheGS/go-stream-tunnel)](https://goreportcard.com/report/github.com/ChacheGS/go-stream-tunnel) [![Container](http://img.shields.io/badge/container-ready-orange.svg)](https://github.com/ChacheGS/go-stream-tunnel/pkgs/container/go-stream-tunnel)

Go Stream Tunnel is a reverse tunnel proxy to expose your local backends behind a firewall to the public — raw TCP streams or Host-routed HTTP tunnels, over the same mTLS-secured connection.
The reverse tunnel is based on HTTP/2 with mutual TLS (mTLS). It enables you to share your localhost when you don't have a public IP.

Features:

* Easily expose a local server to the public
* Secure tunnel, authenticated with mutual TLS
* Dynamic listeners on server by client commands
* Subdomain-routed HTTP tunnels, so each service gets its own public hostname
* A small CA toolchain (`go-stream-tunnel ca`) to issue a distinct certificate per client, instead of sharing one cert everywhere

Common use cases:

* Exposing your local server behind a firewall to the public
* Testing webhook integrations against a local server under a real, publicly reachable HTTPS URL
* Hosting a game server from home

## Getting started

1. Generate a CA and issue certificates for the server and each client (see [Certificate setup](#certificate-setup) below):

   ```sh
   go-stream-tunnel ca init
   go-stream-tunnel ca -name server -addr tunnel.example.com issue
   go-stream-tunnel ca -name laptop issue
   ```

2. Write a `tunnels.yaml` describing what to expose:

   ```yaml
   server_addr: tunnel.example.com:5223
   tunnels:
     www:
       proto: tcp
       addr: localhost:8080
       remote_addr: 80
   ```

3. Start the server on your public host:

   ```sh
   go-stream-tunnel server -tls-crt server/tls.crt -tls-key server/tls.key -ca-crt ca/ca.crt
   ```

4. Start the client on your machine, pointing at its own cert and the same CA:

   ```sh
   go-stream-tunnel client -tls-crt laptop/tls.crt -tls-key laptop/tls.key -ca-crt ca/ca.crt -config tunnels.yaml start-all
   ```

Your local `localhost:8080` is now reachable at `tunnel.example.com:80`. See [Subdomain-routed HTTP tunnels](#subdomain-routed-http-tunnels) if you'd rather expose services under `https://<name>.tunnel.example.com` behind a reverse proxy.

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
    * `proto`: proxy listener protocol, `tcp` or `http` (see [Subdomain-routed HTTP tunnels](#subdomain-routed-http-tunnels) for `http`)
    * `addr`: forward traffic to this local port number or network address, i.e. `localhost:22`
    * `remote_addr`: server listener TCP address, *default:* `same as local port`
* `backoff`
    * `interval`: how long client would wait before redialing the server if connection was lost, exponential backoff initial interval, *default:* `500ms`
    * `multiplier`: interval multiplier if reconnect failed, *default:* `1.5`
    * `max_interval`: maximal time client would wait before redialing the server, *default:* `1m`
    * `max_time`: maximal time client would try to reconnect to the server if connection was lost, set `0` to never stop trying, *default:* `15m`

### Subdomain-routed HTTP tunnels

The original mmatczuk/go-http-tunnel already routed `proto: http` tunnels by
Host header, so multiple services behind one entrypoint isn't new. It never
shipped working WebSocket support though: its HTTP handler copies the
response body into the `ResponseWriter` instead of hijacking the connection,
so a 101 upgrade never gets the raw, bidirectional pipe it needs. jlandowner's
fork kept that model and focused on TCP proxying instead. This fork takes a
different approach for HTTP tunnels: the server peeks the `Host` header at
accept time and routes the raw connection byte-for-byte, so WebSocket
upgrades pass through correctly.

```yaml
server_addr: SERVER_IP:5223
tunnels:
  myapp:
    proto: http
    addr: localhost:8080
```

`subdomain` defaults to the tunnel's own name (`myapp` above) if omitted, so
you don't have to repeat it — set it explicitly only if you want the public
subdomain to differ from the tunnel's name in `tunnels.yaml`.

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

## Certificate setup

Client and server authenticate each other with mutual TLS. Rather than
sharing one certificate between the server and every client, generate a CA
once and issue a distinct certificate per role:

```sh
# once, ever — keep ca/ca.key somewhere safe (a secret manager, not source
# control); ca/ca.crt is not sensitive and is what -ca-crt points at
go-stream-tunnel ca init

# issue the server's own identity cert — -addr must match what clients will
# put in their server_addr / dial
go-stream-tunnel ca -name server -addr tunnel.example.com issue

# issue one cert per client device
go-stream-tunnel ca -name laptop issue
go-stream-tunnel ca -name desktop issue
```

Each `ca issue` prints the new certificate's client ID — the same fingerprint
`go-stream-tunnel client id` would print for that certificate, and the value
you'd put in the server's `-client-ids` flag if you're using an explicit
allowlist instead of auto-subscribe.

Point the server at its issued cert and the CA:

```sh
go-stream-tunnel server -tls-crt server/tls.crt -tls-key server/tls.key -ca-crt ca/ca.crt
```

Copy each client's `tls.crt`/`tls.key` to that device, and point the client
at them plus the same CA cert:

```sh
go-stream-tunnel client -tls-crt laptop/tls.crt -tls-key laptop/tls.key -ca-crt ca/ca.crt -config tunnels.yaml start-all
```

Adding a new client from then on is just another `ca issue` and copying two
files — the CA and the server's own cert never need to change.

## How it works

A client opens TLS connection to a server. The server accepts connections from known clients only. The client is recognized by its TLS certificate ID. The server is publicly available and proxies incoming connections to the client. Then the connection is further proxied in the client's network.

The tunnel is based HTTP/2 for speed and security. There is a single TCP connection between client and server and all the proxied connections are multiplexed using HTTP/2.

> NOTE: lineage
>
> This project descends from https://github.com/mmatczuk/go-http-tunnel via
> https://github.com/jlandowner/go-tcp-tunnel. jlandowner's fork focused the
> original tool on TCP proxying, packaged it as a Docker image, and added a
> Kubernetes Helm chart (`kubernetes/`). This fork builds on that with
> subdomain-routed HTTP tunnels and the CA-based certificate issuance
> described above — and was renamed from `go-tcp-tunnel` to `go-stream-tunnel`
> to match: it tunnels arbitrary byte streams (TCP, and Host-routed HTTP)
> rather than being TCP-only, and `go-http-tunnel` was already taken by the
> project this one differs from.
>
> The Kubernetes chart under `kubernetes/` predates this fork's CA tooling
> and still documents the older shared-self-signed-certificate flow
> (`certutil.sh`). It's untested and unmaintained here — left in place in
> case it's still useful to someone, not as a supported deployment path.

## License

Copyright (C) 2017 Michał Matczuk

Copyright (C) 2022 jlandowner

Copyright (C) 2026 ChacheGS

This project is distributed under the AGPL-3 license. See the [LICENSE](https://github.com/ChacheGS/go-stream-tunnel/blob/master/LICENSE) file for details.
