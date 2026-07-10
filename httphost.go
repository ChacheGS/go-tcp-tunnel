// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
)

// httpFullHost composes the full public hostname for an http tunnel's
// subdomain slug and a server's base domain. This is the single place that
// defines the composition rule; httpSlugFromHost is its exact inverse.
func httpFullHost(baseDomain, slug string) string {
	return slug + "." + baseDomain
}

// httpSlugFromHost recovers the subdomain slug from a full hostname produced
// by httpFullHost. It assumes fullHost was in fact composed by httpFullHost
// for the same baseDomain, which holds for every host the registry can ever
// return from a lookup, since addTunnels is the only writer of registered
// hosts and always uses httpFullHost.
func httpSlugFromHost(baseDomain, fullHost string) string {
	return strings.TrimSuffix(fullHost, "."+baseDomain)
}

// peekHostHeader reads an HTTP request's start-line and headers from r,
// returning the Host header value and a reader that reproduces the exact
// bytes already consumed during parsing followed by the remainder of r. It
// does not alter, re-serialize, or otherwise touch the request in any way —
// this is what keeps arbitrary payloads that follow (request bodies,
// WebSocket frames after a 101 response) byte-for-byte intact for
// downstream proxying.
func peekHostHeader(r io.Reader) (host string, replay io.Reader, err error) {
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	br := bufio.NewReader(tee)

	req, err := http.ReadRequest(br)
	if err != nil {
		return "", nil, err
	}

	return req.Host, io.MultiReader(&buf, r), nil
}

// replayConn wraps a net.Conn so that Read is served from r first (typically
// the byte-exact replay produced by peekHostHeader) before falling back to
// whatever r's tail reader provides. Write, Close, and the address/deadline
// methods all delegate to the embedded net.Conn unchanged.
type replayConn struct {
	net.Conn
	r io.Reader
}

func (c *replayConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
