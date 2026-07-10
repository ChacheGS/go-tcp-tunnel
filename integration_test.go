// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	tunnel "github.com/ChacheGS/go-tcp-tunnel"
	"github.com/ChacheGS/go-tcp-tunnel/log"
	"github.com/ChacheGS/go-tcp-tunnel/proto"
	"golang.org/x/net/websocket"
)

const (
	payloadInitialSize = 512
	payloadLen         = 10
)

// echoTCP accepts connections and copies back received bytes.
func echoTCP(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			io.Copy(conn, conn)
		}()
	}
}

func makeEcho(t testing.TB) (tcp net.Listener) {
	var err error

	// TCP echo
	tcp, err = net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	go echoTCP(tcp)

	return
}

func makeTunnelServer(t testing.TB) *tunnel.Server {
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          ":0",
		AutoSubscribe: true,
		TLSConfig:     tlsConfig(),
		Logger:        log.NewStdLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start(context.Background())

	return s
}

func makeTunnelClient(t testing.TB, serverAddr string, tcpLocalAddr, tcpAddr net.Addr) *tunnel.Client {
	tcpProxy := tunnel.NewMultiStreamProxy(map[string]string{
		port(tcpLocalAddr): tcpAddr.String(),
	}, log.NewStdLogger())

	tunnels := map[string]*proto.Tunnel{
		proto.TCP: {
			Protocol: proto.TCP,
			Addr:     tcpLocalAddr.String(),
		},
	}

	c, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      serverAddr,
		TLSClientConfig: tlsConfig(),
		Tunnels:         tunnels,
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			Stream: tcpProxy.Proxy,
		}),
		Logger: log.NewStdLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		if err := c.Start(ctx); err != nil {
			t.Log(err)
		}
	}()

	return c
}

// waitConnected polls c.Connected() until it's true or timeout elapses,
// failing the test in the latter case.
func waitConnected(t *testing.T, c *tunnel.Client, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	for !c.Connected() {
		select {
		case <-deadline:
			t.Fatal("client did not connect within timeout")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestIntegration(t *testing.T) {
	// local services
	tcp := makeEcho(t)
	defer tcp.Close()

	// server
	s := makeTunnelServer(t)
	defer s.Stop()

	tcpLocalAddr := freeAddr()

	// client
	c := makeTunnelClient(t, s.Addr(),
		tcpLocalAddr, tcp.Addr(),
	)
	// Wait for client to connect
	for i := 0; i < 10; i++ {
		if c.Connected() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer c.Stop()

	payload := randPayload(payloadInitialSize, payloadLen)
	table := []struct {
		S []uint
	}{
		{[]uint{200, 160, 120, 80, 40, 20}},
		{[]uint{40, 80, 120, 160, 200}},
		{[]uint{0, 0, 0, 0, 0, 0, 0, 0, 0, 200}},
	}

	var wg sync.WaitGroup
	for _, test := range table {
		for i, repeat := range test.S {
			p := payload[i]
			r := repeat

			wg.Add(1)
			go func() {
				testTCP(t, tcpLocalAddr, p, r)
				wg.Done()
			}()
		}
	}
	wg.Wait()
}

func testTCP(t testing.TB, addr net.Addr, payload []byte, repeat uint) {
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatal("Dial failed", err)
	}
	defer conn.Close()

	var buf = make([]byte, 10*1024*1024)
	var read, write int
	for repeat > 0 {
		m, err := conn.Write(payload)
		if err != nil {
			t.Error("Write failed", err)
		}
		if m != len(payload) {
			t.Log("Write mismatch", m, len(payload))
		}
		write += m

		n, err := conn.Read(buf)
		if err != nil {
			t.Error("Read failed", err)
		}
		read += n
		repeat--
	}

	for read < write {
		t.Log("No yet read everything", "write", write, "read", read)
		time.Sleep(50 * time.Millisecond)
		n, err := conn.Read(buf)
		if err != nil {
			t.Error("Read failed", err)
		}
		read += n
	}

	if read != write {
		t.Fatal("Write read mismatch", read, write)
	}
}

//
// helpers
//

// randPayload returns slice of randomly initialised data buffers.
func randPayload(initialSize, n int) [][]byte {
	payload := make([][]byte, n)
	l := initialSize
	for i := 0; i < n; i++ {
		payload[i] = randBytes(l)
		l *= 2
	}
	return payload
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	read, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	if read != n {
		panic("read did not fill whole slice")
	}
	return b
}

func freeAddr() net.Addr {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr()
}

func port(addr net.Addr) string {
	return fmt.Sprint(addr.(*net.TCPAddr).Port)
}

func TestIntegration_HTTPSubdomainTunnel(t *testing.T) {
	// local HTTP echo server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from local app"))
	})
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer localLn.Close()
	go http.Serve(localLn, mux)

	// tunnel server with base domain configured
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          ":0",
		AutoSubscribe: true,
		TLSConfig:     tlsConfig(),
		Logger:        log.NewStdLogger(),
		BaseDomain:    "tunnel.example.com",
		HTTPAddr:      "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start(context.Background())
	defer s.Stop()

	// give the server a moment to open both listeners
	time.Sleep(50 * time.Millisecond)

	tcpProxy := tunnel.NewMultiStreamProxy(map[string]string{
		"myapp": localLn.Addr().String(),
	}, log.NewStdLogger())

	c, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: tlsConfig(),
		Tunnels: map[string]*proto.Tunnel{
			"myapp": {Protocol: proto.HTTP, Host: "myapp"},
		},
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			Stream: tcpProxy.Proxy,
		}),
		Logger: log.NewStdLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Start(ctx)

	waitConnected(t, c, 5*time.Second)

	req, err := http.NewRequest(http.MethodGet, "http://myapp.tunnel.example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("tcp", s.HTTPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello from local app" {
		t.Fatalf("expected body %q, got %q", "hello from local app", string(body))
	}
}

// TestIntegration_HTTPSubdomainTunnel_WebSocket proves that a WebSocket
// upgrade handshake and subsequent frames survive the subdomain-routed HTTP
// tunnel path untouched, including across an idle period, since the server
// only ever reads the Host header before handing the connection off to the
// opaque byte-pipe proxy.
func TestIntegration_HTTPSubdomainTunnel_WebSocket(t *testing.T) {
	// local WebSocket echo server
	wsHandler := websocket.Handler(func(ws *websocket.Conn) {
		io.Copy(ws, ws)
	})
	localLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer localLn.Close()
	go http.Serve(localLn, wsHandler)

	// tunnel server with base domain configured
	s, err := tunnel.NewServer(&tunnel.ServerConfig{
		Addr:          ":0",
		AutoSubscribe: true,
		TLSConfig:     tlsConfig(),
		Logger:        log.NewStdLogger(),
		BaseDomain:    "tunnel.example.com",
		HTTPAddr:      "127.0.0.1:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	go s.Start(context.Background())
	defer s.Stop()

	// give the server a moment to open both listeners
	time.Sleep(50 * time.Millisecond)

	tcpProxy := tunnel.NewMultiStreamProxy(map[string]string{
		"myapp": localLn.Addr().String(),
	}, log.NewStdLogger())

	c, err := tunnel.NewClient(&tunnel.ClientConfig{
		ServerAddr:      s.Addr(),
		TLSClientConfig: tlsConfig(),
		Tunnels: map[string]*proto.Tunnel{
			"myapp": {Protocol: proto.HTTP, Host: "myapp"},
		},
		Proxy: tunnel.Proxy(tunnel.ProxyFuncs{
			Stream: tcpProxy.Proxy,
		}),
		Logger: log.NewStdLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Start(ctx)

	waitConnected(t, c, 5*time.Second)

	conn, err := net.Dial("tcp", s.HTTPAddr())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	wsConfig, err := websocket.NewConfig("ws://myapp.tunnel.example.com/", "http://myapp.tunnel.example.com/")
	if err != nil {
		t.Fatal(err)
	}

	ws, err := websocket.NewClient(wsConfig, conn)
	if err != nil {
		t.Fatalf("websocket upgrade failed: %v", err)
	}
	defer ws.Close()

	// first message, immediately after the handshake
	if err := sendAndExpectEcho(ws, "hello over subdomain tunnel"); err != nil {
		t.Fatal(err)
	}

	// idle period to prove the connection isn't torn down or corrupted by
	// any timeout/buffering logic in the new routing path
	time.Sleep(250 * time.Millisecond)

	// second message, after the idle period
	if err := sendAndExpectEcho(ws, "still alive after idle"); err != nil {
		t.Fatal(err)
	}
}

func sendAndExpectEcho(ws *websocket.Conn, msg string) error {
	if err := websocket.Message.Send(ws, msg); err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	var reply string
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := websocket.Message.Receive(ws, &reply); err != nil {
		return fmt.Errorf("receive failed: %w", err)
	}
	if reply != msg {
		return fmt.Errorf("expected echo %q, got %q", msg, reply)
	}
	return nil
}

func tlsConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("./testdata/selfsigned.crt", "./testdata/selfsigned.key")
	if err != nil {
		panic(err)
	}

	f, err := os.Open("./testdata/selfsigned.crt")
	if err != nil {
		panic(err)
	}

	ca, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(ca); !ok {
		panic("failed to append cert")
	}

	c := &tls.Config{
		ServerName:               "localhost",
		Certificates:             []tls.Certificate{cert},
		ClientAuth:               tls.RequireAndVerifyClientCert,
		ClientCAs:                caPool,
		RootCAs:                  caPool,
		SessionTicketsDisabled:   true,
		MinVersion:               tls.VersionTLS12,
		CipherSuites:             []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		PreferServerCipherSuites: true,
		NextProtos:               []string{"h2"},
	}
	return c
}
