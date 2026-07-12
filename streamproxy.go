// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"fmt"
	"io"
	"net"

	"github.com/ChacheGS/go-stream-tunnel/log"
	"github.com/ChacheGS/go-stream-tunnel/proto"
)

// StreamProxy forwards opaque byte streams by dialing a local address and
// copying bytes in both directions. It handles both tcp/tcp4/tcp6 and http
// tunnels: an http tunnel needs nothing more than this same byte-pipe
// behavior, since the server has already routed the connection to the right
// client by the time it reaches here (see ControlMessage.ForwardedProto).
type StreamProxy struct {
	// localAddr specifies default TCP address of the local server.
	localAddr string
	// localAddrMap specifies mapping from ControlMessage.ForwardedHost to
	// local server address, keys may contain host and port, only host or
	// only port. The order of precedence is the following
	// * host and port
	// * port
	// * host
	localAddrMap map[string]string
	// logger is the proxy logger.
	logger log.Logger
}

// NewStreamProxy creates new direct StreamProxy, everything will be proxied
// to localAddr.
func NewStreamProxy(localAddr string, logger log.Logger) *StreamProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &StreamProxy{
		localAddr: localAddr,
		logger:    logger,
	}
}

// NewMultiStreamProxy creates a new dispatching StreamProxy, connections may
// go to different backends based on localAddrMap.
func NewMultiStreamProxy(localAddrMap map[string]string, logger log.Logger) *StreamProxy {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &StreamProxy{
		localAddrMap: localAddrMap,
		logger:       logger,
	}
}

// Proxy is a ProxyFunc.
func (p *StreamProxy) Proxy(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
	switch msg.ForwardedProto {
	case proto.TCP, proto.TCP4, proto.TCP6, proto.HTTP:
		// ok
	default:
		p.logger.Log(
			"level", 0,
			"msg", "unsupported protocol",
			"ctrlMsg", msg,
		)
		return
	}

	target := p.localAddrFor(msg.ForwardedHost)
	if target == "" {
		p.logger.Log(
			"level", 1,
			"msg", "no target",
			"ctrlMsg", msg,
		)
		return
	}

	local, err := net.DialTimeout("tcp", target, DefaultTimeout)
	if err != nil {
		p.logger.Log(
			"level", 0,
			"msg", "dial failed",
			"target", target,
			"ctrlMsg", msg,
			"err", err,
		)
		return
	}
	defer local.Close()

	if err := keepAlive(local.(*net.TCPConn)); err != nil {
		p.logger.Log(
			"level", 1,
			"msg", "TCP keepalive for tunneled connection failed",
			"target", target,
			"ctrlMsg", msg,
			"err", err,
		)
	}

	done := make(chan struct{})
	go func() {
		transfer(flushWriter{w}, local, log.NewContext(p.logger).With(
			"dst", msg.ForwardedHost,
			"src", target,
		))
		close(done)
	}()

	transfer(local, r, log.NewContext(p.logger).With(
		"dst", target,
		"src", msg.ForwardedHost,
	))

	<-done
}

func (p *StreamProxy) localAddrFor(hostPort string) string {
	if len(p.localAddrMap) == 0 {
		return p.localAddr
	}

	// try hostPort
	if addr := p.localAddrMap[hostPort]; addr != "" {
		return addr
	}

	// try port
	host, port, _ := net.SplitHostPort(hostPort)
	if addr := p.localAddrMap[port]; addr != "" {
		return addr
	}

	// try 0.0.0.0:port
	if addr := p.localAddrMap[fmt.Sprintf("0.0.0.0:%s", port)]; addr != "" {
		return addr
	}

	// try host
	if addr := p.localAddrMap[host]; addr != "" {
		return addr
	}

	return p.localAddr
}
