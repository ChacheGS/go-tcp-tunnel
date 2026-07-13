// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/ChacheGS/go-stream-tunnel/log"
	"github.com/ChacheGS/go-stream-tunnel/proto"
)

// ClientConfig is configuration of the Client.
type ClientConfig struct {
	// ServerAddr specifies TCP address of the tunnel server.
	ServerAddr string
	// TLSClientConfig specifies the tls configuration to use with
	// tls.Client.
	TLSClientConfig *tls.Config
	// DialTLS specifies an optional dial function that creates a tls
	// connection to the server. If DialTLS is nil, tls.Dial is used.
	DialTLS func(network, addr string, config *tls.Config) (net.Conn, error)
	// Backoff specifies backoff policy on server connection retry. If nil
	// when dial fails it will not be retried.
	Backoff Backoff
	// Tunnels specifies the tunnels client requests to be opened on server.
	Tunnels map[string]*proto.Tunnel
	// Proxy is ProxyFunc responsible for transferring data between server
	// and local services.
	Proxy ProxyFunc
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
}

// Client is responsible for creating connection to the server, handling control
// messages. It uses ProxyFunc for transferring data between server and local
// services.
type Client struct {
	config *ClientConfig

	conn           net.Conn
	connMu         sync.Mutex
	httpServer     *http2.Server
	serverErr      error
	lastDisconnect time.Time
	logger         log.Logger

	// onTunnelInfo, if set, is invoked with resolved tunnel-name -> full
	// hostname pairs whenever the server pushes tunnel info. Exposed as a
	// field (rather than only logging) so tests can observe it directly;
	// production code leaves it nil and relies on the default log line
	// inside handleTunnelInfo.
	onTunnelInfo func(hosts map[string]string)
}

// NewClient creates a new unconnected Client based on configuration. Caller
// must invoke Start() on returned instance in order to connect server.
func NewClient(config *ClientConfig) (*Client, error) {
	if config.ServerAddr == "" {
		return nil, errors.New("missing ServerAddr")
	}
	if config.TLSClientConfig == nil {
		return nil, errors.New("missing TLSClientConfig")
	}
	if len(config.Tunnels) == 0 {
		return nil, errors.New("missing Tunnels")
	}
	if config.Proxy == nil {
		return nil, errors.New("missing Proxy")
	}

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	c := &Client{
		config:     config,
		httpServer: &http2.Server{},
		logger:     logger,
	}

	return c, nil
}

// Start connects client to the server, it returns error if there is a
// connection error, or server cannot open requested tunnels. On connection
// error a backoff policy is used to reestablish the connection. When connected
// HTTP/2 server is started to handle ControlMessages.
func (c *Client) Start(ctx context.Context) error {
	c.logger.Log(
		"level", 1,
		"action", "start",
	)

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	for {
		conn, err := c.connect()
		if err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// continue
			}
			return err
		}

		c.httpServer.ServeConn(conn, &http2.ServeConnOpts{
			Handler: http.HandlerFunc(c.serveHTTP),
		})

		c.logger.Log(
			"level", 1,
			"action", "disconnected",
		)

		// A cancelled ctx means this disconnect was requested (Stop() was
		// called by the goroutine above), not a transient network hiccup.
		// Without this check the loop can't tell the difference and just
		// redials, so Ctrl-C/SIGTERM would only ever actually stop the
		// client if a subsequent reconnect happened to fail on its own.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c.connMu.Lock()
		now := time.Now()
		err = c.serverErr

		// detect disconnect hiccup
		if err == nil && now.Sub(c.lastDisconnect).Seconds() < 5 {
			err = fmt.Errorf("connection is being cut")
		}

		c.conn = nil
		c.serverErr = nil
		c.lastDisconnect = now
		c.connMu.Unlock()

		if err != nil {
			return err
		}
	}
}

func (c *Client) connect() (net.Conn, error) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return nil, fmt.Errorf("already connected")
	}

	conn, err := c.dial()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %s", err)
	}
	c.conn = conn

	return conn, nil
}

func (c *Client) dial() (net.Conn, error) {
	var (
		network   = "tcp"
		addr      = c.config.ServerAddr
		tlsConfig = c.config.TLSClientConfig
	)

	doDial := func() (conn net.Conn, err error) {
		c.logger.Log(
			"level", 1,
			"action", "dial",
			"network", network,
			"addr", addr,
		)

		if c.config.DialTLS != nil {
			conn, err = c.config.DialTLS(network, addr, tlsConfig)
		} else {
			d := &net.Dialer{
				Timeout: DefaultTimeout,
			}
			conn, err = d.Dial(network, addr)

			if err == nil {
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					err = keepAlive(tcpConn)
				}
			}
			if err == nil {
				conn = tls.Client(conn, tlsConfig)
			}
			if err == nil {
				err = conn.(*tls.Conn).Handshake()
			}
		}

		if err != nil {
			if conn != nil {
				conn.Close()
				conn = nil
			}

			c.logger.Log(
				"level", 0,
				"msg", "dial failed",
				"network", network,
				"addr", addr,
				"err", err,
			)
		}

		return
	}

	b := c.config.Backoff
	if b == nil {
		return doDial()
	}

	for {
		conn, err := doDial()

		// success
		if err == nil {
			b.Reset()
			return conn, err
		}

		// failure
		d := b.NextBackOff()
		if d < 0 {
			return conn, fmt.Errorf("backoff limit exceeded: %s", err)
		}

		// backoff
		c.logger.Log(
			"level", 1,
			"action", "backoff",
			"sleep", d,
		)
		time.Sleep(d)
	}
}

func (c *Client) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		switch {
		case r.Header.Get(proto.HeaderError) != "":
			c.handleHandshakeError(w, r)
		case r.Header.Get(proto.HeaderTunnelInfo) != "":
			c.handleTunnelInfo(w, r)
		default:
			c.handleHandshake(w, r)
		}
		return
	}

	msg, err := proto.ReadControlMessage(r)
	if err != nil {
		c.logger.Log(
			"level", 1,
			"err", err,
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.logger.Log(
		"level", 2,
		"action", "handle",
		"ctrlMsg", msg,
	)

	switch msg.Action {
	case proto.ActionProxy:
		c.config.Proxy(w, r.Body, msg)
	default:
		c.logger.Log(
			"level", 0,
			"msg", "unknown action",
			"ctrlMsg", msg,
		)
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
	c.logger.Log(
		"level", 2,
		"action", "done",
		"ctrlMsg", msg,
	)
}

func (c *Client) handleHandshakeError(w http.ResponseWriter, r *http.Request) {
	err := errors.New(r.Header.Get(proto.HeaderError))

	c.logger.Log(
		"level", 1,
		"action", "handshake error",
		"addr", r.RemoteAddr,
		"err", err,
	)

	c.connMu.Lock()
	c.serverErr = fmt.Errorf("server error: %s", err)
	c.connMu.Unlock()
}

func (c *Client) handleHandshake(w http.ResponseWriter, r *http.Request) {
	c.logger.Log(
		"level", 1,
		"action", "handshake",
		"addr", r.RemoteAddr,
	)

	b, err := json.Marshal(c.config.Tunnels)
	if err != nil {
		c.logger.Log(
			"level", 0,
			"msg", "handshake failed",
			"err", err,
		)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func (c *Client) handleTunnelInfo(w http.ResponseWriter, r *http.Request) {
	var hosts map[string]string
	if err := json.NewDecoder(r.Body).Decode(&hosts); err != nil {
		c.logger.Log(
			"level", 1,
			"msg", "failed to decode tunnel info",
			"err", err,
		)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for name, host := range hosts {
		c.logger.Log(
			"level", 1,
			"action", "tunnel ready",
			"name", name,
			"url", "https://"+host,
		)
	}

	if c.onTunnelInfo != nil {
		c.onTunnelInfo(hosts)
	}

	w.WriteHeader(http.StatusOK)
}

// Stop disconnects client from server.
func (c *Client) Stop() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.logger.Log(
		"level", 1,
		"action", "stop",
	)

	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = nil
}

// Connected returns true if client is connected to server.
func (c *Client) Connected() bool {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	return c.conn != nil
}
