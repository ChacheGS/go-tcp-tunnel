// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"io"

	"github.com/ChacheGS/go-stream-tunnel/proto"
)

// ProxyFunc is responsible for forwarding a remote connection to local server
// and writing the response.
type ProxyFunc func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage)

// ProxyFuncs is a collection of ProxyFunc.
type ProxyFuncs struct {
	// Stream is the proxying implementation for tcp/tcp4/tcp6 tunnels, and
	// also for http tunnels: both need nothing more than an opaque byte
	// pipe to a local address, since routing to the right client already
	// happened server-side by the time a connection reaches this func.
	Stream ProxyFunc
}

// Proxy returns a ProxyFunc that uses custom function if provided.
func Proxy(p ProxyFuncs) ProxyFunc {
	return func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
		var f ProxyFunc
		switch msg.ForwardedProto {
		case proto.TCP, proto.TCP4, proto.TCP6, proto.HTTP:
			f = p.Stream
		}

		if f == nil {
			return
		}

		f(w, r, msg)
	}
}
