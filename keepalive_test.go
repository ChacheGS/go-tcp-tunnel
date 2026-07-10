package tunnel

import (
	"net"
	"testing"
)

func TestKeepAlive_Success(t *testing.T) {
	t.Parallel()

	// Create a real TCP listener and connection to test keepAlive
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			conn.Close()
		}
		close(done)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatal("expected *net.TCPConn")
	}

	if err := keepAlive(tcpConn); err != nil {
		t.Fatalf("keepAlive failed: %v", err)
	}

	<-done
}

func TestKeepAlive_ClosedConnection(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatal("expected *net.TCPConn")
	}
	tcpConn.Close()

	if err := keepAlive(tcpConn); err == nil {
		t.Fatal("expected error from keepAlive on a closed connection")
	}
}
