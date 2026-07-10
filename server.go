// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/jlandowner/go-tcp-tunnel/id"
	"github.com/jlandowner/go-tcp-tunnel/log"
	"github.com/jlandowner/go-tcp-tunnel/proto"
)

// ServerConfig defines configuration for the Server.
type ServerConfig struct {
	// Addr is TCP address to listen for client connections. If empty ":0"
	// is used.
	Addr string
	// AutoSubscribe if enabled will automatically subscribe new clients on
	// first call.
	AutoSubscribe bool
	// TLSConfig specifies the tls configuration to use with tls.Listener.
	TLSConfig *tls.Config
	// Listener specifies optional listener for client connections. If nil
	// tls.Listen("tcp", Addr, TLSConfig) is used.
	Listener net.Listener
	// BaseDomain, if set, enables subdomain-routed http tunnels. A client
	// tunnel with Host "myapp" becomes reachable at myapp.<BaseDomain>.
	// Leave empty to disable http tunnels entirely.
	BaseDomain string
	// HTTPAddr is the internal address the server listens on for
	// subdomain-routed http tunnel traffic, e.g. from a reverse proxy that
	// terminates public TLS for *.<BaseDomain>. Only used if BaseDomain is
	// also set.
	HTTPAddr string
	// Logger is optional logger. If nil logging is disabled.
	Logger log.Logger
}

// Server is responsible for proxying public connections to the client over a
// tunnel connection.
type Server struct {
	*registry
	config *ServerConfig

	listener     net.Listener
	httpListener net.Listener
	connPool     *connPool
	httpClient   *http.Client
	logger       log.Logger
}

// NewServer creates a new Server.
func NewServer(config *ServerConfig) (*Server, error) {
	listener, err := listener(config)
	if err != nil {
		return nil, fmt.Errorf("listener failed: %s", err)
	}

	logger := config.Logger
	if logger == nil {
		logger = log.NewNopLogger()
	}

	s := &Server{
		registry: newRegistry(logger),
		config:   config,
		listener: listener,
		logger:   logger,
	}

	t := &http2.Transport{}
	pool := newConnPool(t, s.disconnected)
	t.ConnPool = pool
	s.connPool = pool
	s.httpClient = &http.Client{
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return s, nil
}

func listener(config *ServerConfig) (net.Listener, error) {
	if config.Listener != nil {
		return config.Listener, nil
	}

	if config.Addr == "" {
		return nil, errors.New("missing Addr")
	}
	if config.TLSConfig == nil {
		return nil, errors.New("missing TLSConfig")
	}

	return net.Listen("tcp", config.Addr)
}

// disconnected clears resources used by client, it's invoked by connection pool
// when client goes away.
func (s *Server) disconnected(identifier id.ID) {
	s.logger.Log(
		"level", 1,
		"action", "disconnected",
		"identifier", identifier,
	)

	i := s.registry.clear(identifier)
	if i == nil {
		return
	}
	for _, l := range i.Listeners {
		s.logger.Log(
			"level", 2,
			"action", "close listener",
			"identifier", identifier,
			"addr", l.Addr(),
		)
		l.Close()
	}
}

// Start starts accepting connections form clients. For accepting http traffic
// from end users server must be run as handler on http server.
func (s *Server) Start(ctx context.Context) error {
	addr := s.listener.Addr().String()

	s.logger.Log(
		"level", 1,
		"action", "start",
		"addr", addr,
	)

	if s.config.BaseDomain != "" && s.config.HTTPAddr != "" {
		httpLn, err := net.Listen("tcp", s.config.HTTPAddr)
		if err != nil {
			// s.listener is already open at this point (bound in NewServer,
			// before Start is ever called); it must be closed here or its
			// file descriptor leaks, since nothing else will close it if we
			// return before the ctx-cancellation/Stop() goroutine is spawned.
			s.listener.Close()
			return fmt.Errorf("failed to start http listener: %s", err)
		}
		s.httpListener = httpLn

		s.logger.Log(
			"level", 1,
			"action", "start http listener",
			"addr", httpLn.Addr().String(),
			"base_domain", s.config.BaseDomain,
		)

		go s.listenHTTP(httpLn)
	}

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.logger.Log(
					"level", 1,
					"action", "control connection listener closed",
					"addr", addr,
				)
				return err
			}

			s.logger.Log(
				"level", 0,
				"msg", "accept of control connection failed",
				"addr", addr,
				"err", err,
			)
			continue
		}

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := keepAlive(tcpConn); err != nil {
				s.logger.Log(
					"level", 0,
					"msg", "TCP keepalive for control connection failed",
					"addr", addr,
					"err", err,
				)
			}
		}

		go s.handleClient(tls.Server(conn, s.config.TLSConfig))
	}
}

func (s *Server) handleClient(conn net.Conn) {
	logger := log.NewContext(s.logger).With("addr", conn.RemoteAddr())

	logger.Log(
		"level", 1,
		"action", "try connect",
	)

	var (
		identifier id.ID
		req        *http.Request
		resp       *http.Response
		tunnels    map[string]*proto.Tunnel
		err        error
		ok         bool

		inConnPool bool
	)

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		logger.Log(
			"level", 0,
			"msg", "invalid connection type",
			"err", fmt.Errorf("expected TLS conn, got %T", conn),
		)
		goto reject
	}

	identifier, err = id.PeerID(tlsConn)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "certificate error",
			"err", err,
		)
		goto reject
	}

	logger = logger.With("identifier", identifier)

	if s.config.AutoSubscribe {
		s.Subscribe(identifier)
	} else if !s.IsSubscribed(identifier) {
		logger.Log(
			"level", 2,
			"msg", "unknown client",
		)
		goto reject
	}

	if err = conn.SetDeadline(time.Time{}); err != nil {
		logger.Log(
			"level", 2,
			"msg", "setting infinite deadline failed",
			"err", err,
		)
		goto reject
	}

	if err := s.connPool.AddConn(conn, identifier); err != nil {
		logger.Log(
			"level", 2,
			"msg", "adding connection failed",
			"err", err,
		)
		goto reject
	}
	inConnPool = true

	req, err = http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake request creation failed",
			"err", err,
		)
		goto reject
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err = s.httpClient.Do(req)
	if err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("status %s", resp.Status)
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if resp.ContentLength == 0 {
		err = fmt.Errorf("tunnels Content-Length: 0")
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if err = json.NewDecoder(&io.LimitedReader{R: resp.Body, N: 126976}).Decode(&tunnels); err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if len(tunnels) == 0 {
		err = fmt.Errorf("no tunnels")
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	if err = s.addTunnels(tunnels, identifier); err != nil {
		logger.Log(
			"level", 2,
			"msg", "handshake failed",
			"err", err,
		)
		goto reject
	}

	{
		hosts := make(map[string]string)
		for name, t := range tunnels {
			if t.Protocol == proto.HTTP {
				hosts[name] = t.Host + "." + s.config.BaseDomain
			}
		}
		s.notifyTunnelInfo(hosts, identifier)
	}

	logger.Log(
		"level", 1,
		"action", "connected",
	)

	return

reject:
	logger.Log(
		"level", 1,
		"action", "rejected",
	)

	if inConnPool {
		s.notifyError(err, identifier)
		s.connPool.DeleteConn(identifier)
	} else {
		conn.Close()
	}
}

// notifyError tries to send error to client.
func (s *Server) notifyError(serverError error, identifier id.ID) {
	if serverError == nil {
		return
	}

	req, err := http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), nil)
	if err != nil {
		s.logger.Log(
			"level", 2,
			"action", "client error notification failed",
			"identifier", identifier,
			"err", err,
		)
		return
	}

	req.Header.Set(proto.HeaderError, serverError.Error())

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	if resp, err := s.httpClient.Do(req.WithContext(ctx)); err == nil {
		resp.Body.Close()
	}
}

// notifyTunnelInfo sends resolved public hostnames for a client's http
// tunnels back down to the client, so it can display real, usable URLs to
// the developer instead of just the local addr it's forwarding to.
func (s *Server) notifyTunnelInfo(hosts map[string]string, identifier id.ID) {
	if len(hosts) == 0 {
		return
	}

	b, err := json.Marshal(hosts)
	if err != nil {
		s.logger.Log(
			"level", 1,
			"action", "tunnel info notification failed",
			"identifier", identifier,
			"err", err,
		)
		return
	}

	req, err := http.NewRequest(http.MethodConnect, s.connPool.URL(identifier), bytes.NewReader(b))
	if err != nil {
		s.logger.Log(
			"level", 2,
			"action", "tunnel info notification failed",
			"identifier", identifier,
			"err", err,
		)
		return
	}
	req.Header.Set(proto.HeaderTunnelInfo, "1")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	if resp, err := s.httpClient.Do(req.WithContext(ctx)); err == nil {
		resp.Body.Close()
	}
}

// addTunnels invokes addHost or addListener based on data from proto.Tunnel. If
// a tunnel cannot be added whole batch is reverted.
func (s *Server) addTunnels(tunnels map[string]*proto.Tunnel, identifier id.ID) error {
	i := &RegistryItem{
		Hosts:     []string{},
		Listeners: []net.Listener{},
	}

	var err error
	for name, t := range tunnels {
		switch t.Protocol {
		case proto.TCP, proto.TCP4, proto.TCP6:
			var l net.Listener
			l, err = net.Listen(t.Protocol, t.Addr)
			if err != nil {
				goto rollback
			}

			s.logger.Log(
				"level", 2,
				"action", "open listener",
				"identifier", identifier,
				"addr", l.Addr(),
			)

			i.Listeners = append(i.Listeners, l)
		case proto.HTTP:
			if s.config.BaseDomain == "" {
				err = fmt.Errorf("tunnel %s: server has no base domain configured for http tunnels", name)
				goto rollback
			}
			if t.Host == "" {
				err = fmt.Errorf("tunnel %s: missing host", name)
				goto rollback
			}
			if !proto.ValidSubdomainLabel(t.Host) {
				err = fmt.Errorf("tunnel %s: %q is not a valid DNS label", name, t.Host)
				goto rollback
			}

			fullHost := t.Host + "." + s.config.BaseDomain

			s.logger.Log(
				"level", 2,
				"action", "register host",
				"identifier", identifier,
				"host", fullHost,
			)

			i.Hosts = append(i.Hosts, fullHost)
		default:
			err = fmt.Errorf("unsupported protocol for tunnel %s: %s", name, t.Protocol)
			goto rollback
		}
	}

	err = s.set(i, identifier)
	if err != nil {
		goto rollback
	}

	for _, l := range i.Listeners {
		go s.listen(l, identifier)
	}

	return nil

rollback:
	for _, l := range i.Listeners {
		l.Close()
	}

	return err
}

// Unsubscribe removes client from registry, disconnects client if already
// connected and returns it's RegistryItem.
func (s *Server) Unsubscribe(identifier id.ID) *RegistryItem {
	s.connPool.DeleteConn(identifier)
	return s.registry.Unsubscribe(identifier)
}

// Ping measures the RTT response time.
func (s *Server) Ping(identifier id.ID) (time.Duration, error) {
	return s.connPool.Ping(identifier)
}

func (s *Server) listen(l net.Listener, identifier id.ID) {
	addr := l.Addr().String()

	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.logger.Log(
					"level", 2,
					"action", "listener closed",
					"identifier", identifier,
					"addr", addr,
				)
				return
			}

			s.logger.Log(
				"level", 0,
				"msg", "accept of connection failed",
				"identifier", identifier,
				"addr", addr,
				"err", err,
			)
			continue
		}

		msg := &proto.ControlMessage{
			Action:         proto.ActionProxy,
			ForwardedProto: l.Addr().Network(),
		}

		msg.ForwardedHost = l.Addr().String()

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := keepAlive(tcpConn); err != nil {
				s.logger.Log(
					"level", 1,
					"msg", "TCP keepalive for tunneled connection failed",
					"identifier", identifier,
					"ctrlMsg", msg,
					"err", err,
				)
			}
		}

		go func() {
			if err := s.proxyConn(identifier, conn, msg); err != nil {
				s.logger.Log(
					"level", 0,
					"msg", "proxy error",
					"identifier", identifier,
					"ctrlMsg", msg,
					"err", err,
				)
			}
		}()
	}
}

// listenHTTP accepts connections for subdomain-routed http tunnels, shared
// across every connected client (unlike listen, which serves one client's
// dedicated TCP listener).
func (s *Server) listenHTTP(l net.Listener) {
	addr := l.Addr().String()

	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.logger.Log(
					"level", 2,
					"action", "http listener closed",
					"addr", addr,
				)
				return
			}

			s.logger.Log(
				"level", 0,
				"msg", "accept of http connection failed",
				"addr", addr,
				"err", err,
			)
			continue
		}

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := keepAlive(tcpConn); err != nil {
				s.logger.Log(
					"level", 1,
					"msg", "TCP keepalive for tunneled http connection failed",
					"addr", addr,
					"err", err,
				)
			}
		}

		go s.handleHTTPConn(conn)
	}
}

// handleHTTPConn reads the Host header off a freshly accepted connection,
// finds which client registered that subdomain, and hands the connection to
// proxyConn with the already-consumed bytes replayed first so the client
// receives the exact original request.
func (s *Server) handleHTTPConn(conn net.Conn) {
	if err := conn.SetReadDeadline(time.Now().Add(DefaultTimeout)); err != nil {
		s.logger.Log(
			"level", 1,
			"msg", "failed to set read deadline for host sniffing",
			"err", err,
		)
		conn.Close()
		return
	}

	host, replay, err := peekHostHeader(conn)
	if err != nil {
		s.logger.Log(
			"level", 1,
			"msg", "failed to read request host",
			"err", err,
		)
		conn.Close()
		return
	}

	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		s.logger.Log(
			"level", 1,
			"msg", "failed to clear read deadline after host sniffing",
			"err", err,
		)
		conn.Close()
		return
	}

	// Host headers are case-insensitive (RFC 9110 §4.2.3), but registered
	// subdomains are always lowercase (proto.ValidSubdomainLabel rejects
	// uppercase), so the incoming value must be folded to match.
	fullHost := strings.ToLower(trimPort(host))

	identifier, ok := s.registry.Subscriber(fullHost)
	if !ok {
		s.logger.Log(
			"level", 1,
			"msg", "no tunnel registered for host",
			"host", fullHost,
		)
		io.WriteString(conn, "HTTP/1.1 404 Not Found\r\nConnection: close\r\nContent-Length: 0\r\n\r\n")
		conn.Close()
		return
	}

	slug := strings.TrimSuffix(fullHost, "."+s.config.BaseDomain)

	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedHost:  slug,
		ForwardedProto: proto.HTTP,
	}

	rc := &replayConn{Conn: conn, r: replay}

	if err := s.proxyConn(identifier, rc, msg); err != nil {
		s.logger.Log(
			"level", 0,
			"msg", "http proxy error",
			"identifier", identifier,
			"ctrlMsg", msg,
			"err", err,
		)
	}
}

func (s *Server) proxyConn(identifier id.ID, conn net.Conn, msg *proto.ControlMessage) error {
	s.logger.Log(
		"level", 2,
		"action", "proxy conn",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	defer conn.Close()

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	req, err := s.connectRequest(identifier, msg, pr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		transfer(pw, conn, log.NewContext(s.logger).With(
			"dir", "user to client",
			"dst", identifier,
			"src", conn.RemoteAddr(),
		))
		cancel()
		close(done)
	}()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("io error: %s", err)
	}
	defer resp.Body.Close()

	transfer(conn, resp.Body, log.NewContext(s.logger).With(
		"dir", "client to user",
		"dst", conn.RemoteAddr(),
		"src", identifier,
	))

	select {
	case <-done:
	case <-time.After(DefaultTimeout):
	}

	s.logger.Log(
		"level", 2,
		"action", "proxy conn done",
		"identifier", identifier,
		"ctrlMsg", msg,
	)

	return nil
}

// connectRequest creates HTTP request to client with a given identifier having
// control message and data input stream, output data stream results from
// response the created request.
func (s *Server) connectRequest(identifier id.ID, msg *proto.ControlMessage, r io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPut, s.connPool.URL(identifier), r)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %s", err)
	}
	msg.WriteToHeader(req.Header)

	return req, nil
}

// Addr returns network address clients connect to.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// HTTPAddr returns the address of the internal subdomain-routing listener,
// or "" if it is not running.
func (s *Server) HTTPAddr() string {
	if s.httpListener == nil {
		return ""
	}
	return s.httpListener.Addr().String()
}

// Stop closes the server.
func (s *Server) Stop() {
	s.logger.Log(
		"level", 1,
		"action", "stop",
	)

	if s.listener != nil {
		s.listener.Close()
	}
	if s.httpListener != nil {
		s.httpListener.Close()
	}
}
