// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/ChacheGS/go-tcp-tunnel/log"
	"github.com/ChacheGS/go-tcp-tunnel/proto"
	"github.com/ChacheGS/go-tcp-tunnel/tunnelmock"
)

func TestClient_Dial(t *testing.T) {
	t.Parallel()

	s := httptest.NewTLSServer(nil)
	defer s.Close()

	c, err := NewClient(&ClientConfig{
		ServerAddr: s.Listener.Addr().String(),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Tunnels: map[string]*proto.Tunnel{"test": {}},
		Proxy:   Proxy(ProxyFuncs{}),
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := c.dial()
	if err != nil {
		t.Fatal("Dial error", err)
	}
	if conn == nil {
		t.Fatal("Expected connection", err)
	}
	conn.Close()
}

func TestClient_DialBackoff(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockBackoff(ctrl)
	gomock.InOrder(
		b.EXPECT().NextBackOff().Return(50*time.Millisecond).Times(2),
		b.EXPECT().NextBackOff().Return(-time.Millisecond),
	)

	d := func(network, addr string, config *tls.Config) (net.Conn, error) {
		return nil, errors.New("foobar")
	}

	c, err := NewClient(&ClientConfig{
		ServerAddr:      "8.8.8.8",
		TLSClientConfig: &tls.Config{},
		DialTLS:         d,
		Backoff:         b,
		Tunnels:         map[string]*proto.Tunnel{"test": {}},
		Proxy:           Proxy(ProxyFuncs{}),
	})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	_, err = c.dial()

	if time.Since(start) < 100*time.Millisecond {
		t.Fatal("Wait mismatch", err)
	}

	if err.Error() != "backoff limit exceeded: foobar" {
		t.Fatal("Error mismatch", err)
	}
}

func TestClient_handleTunnelInfo_InvokesCallback(t *testing.T) {
	t.Parallel()

	logger := log.NewStdLogger()
	c := &Client{
		config: &ClientConfig{Logger: logger},
		logger: logger,
	}

	received := make(chan map[string]string, 1)
	c.onTunnelInfo = func(hosts map[string]string) {
		received <- hosts
	}

	body := `{"myapp":"myapp.tunnel.example.com"}`
	req := httptest.NewRequest(http.MethodConnect, "/", strings.NewReader(body))
	req.Header.Set(proto.HeaderTunnelInfo, "1")
	w := httptest.NewRecorder()

	c.serveHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case hosts := <-received:
		if hosts["myapp"] != "myapp.tunnel.example.com" {
			t.Fatalf("expected myapp -> myapp.tunnel.example.com, got %v", hosts)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("onTunnelInfo callback was not invoked")
	}
}

func TestClient_handleTunnelInfo_DecodeError(t *testing.T) {
	t.Parallel()

	logger := log.NewStdLogger()
	c := &Client{
		config: &ClientConfig{Logger: logger},
		logger: logger,
	}

	req := httptest.NewRequest(http.MethodConnect, "/", strings.NewReader("not valid json"))
	req.Header.Set(proto.HeaderTunnelInfo, "1")
	w := httptest.NewRecorder()

	c.serveHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed tunnel info body, got %d", w.Code)
	}
}
