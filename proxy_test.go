package tunnel

import (
	"bytes"
	"io"
	"testing"

	"github.com/jlandowner/go-tcp-tunnel/proto"
)

func TestProxy_TCP(t *testing.T) {
	t.Parallel()

	protos := []string{proto.TCP, proto.TCP4, proto.TCP6}

	for _, p := range protos {
		called := false
		pf := Proxy(ProxyFuncs{
			Stream: func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
				called = true
			},
		})

		msg := &proto.ControlMessage{
			Action:         proto.ActionProxy,
			ForwardedHost:  "localhost:80",
			ForwardedProto: p,
		}

		pf(&bytes.Buffer{}, io.NopCloser(&bytes.Buffer{}), msg)
		if !called {
			t.Errorf("TCP handler not called for proto %q", p)
		}
	}
}

func TestProxy_NilHandler(t *testing.T) {
	t.Parallel()

	pf := Proxy(ProxyFuncs{
		Stream: nil,
	})

	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedHost:  "localhost:80",
		ForwardedProto: proto.TCP,
	}

	// Should not panic
	pf(&bytes.Buffer{}, io.NopCloser(&bytes.Buffer{}), msg)
}

func TestProxy_UnknownProtocol(t *testing.T) {
	t.Parallel()

	called := false
	pf := Proxy(ProxyFuncs{
		Stream: func(w io.Writer, r io.ReadCloser, msg *proto.ControlMessage) {
			called = true
		},
	})

	msg := &proto.ControlMessage{
		Action:         proto.ActionProxy,
		ForwardedHost:  "localhost:80",
		ForwardedProto: "udp",
	}

	pf(&bytes.Buffer{}, io.NopCloser(&bytes.Buffer{}), msg)
	if called {
		t.Error("TCP handler should not be called for unknown protocol")
	}
}
