// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"io"
	"net"
	"strings"
	"testing"
)

func TestPeekHostHeader_ExtractsHostAndReplaysBytes(t *testing.T) {
	t.Parallel()

	raw := "GET /path HTTP/1.1\r\nHost: myapp.tunnel.example.com\r\nContent-Length: 5\r\n\r\nhello"
	r := strings.NewReader(raw)

	host, replay, err := peekHostHeader(r)
	if err != nil {
		t.Fatal(err)
	}
	if host != "myapp.tunnel.example.com" {
		t.Fatalf("expected host %q, got %q", "myapp.tunnel.example.com", host)
	}

	got, err := io.ReadAll(replay)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != raw {
		t.Fatalf("expected replayed bytes to equal original request verbatim\nwant: %q\ngot:  %q", raw, string(got))
	}
}

func TestPeekHostHeader_MalformedRequest(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("this is not http\r\n\r\n")

	_, _, err := peekHostHeader(r)
	if err == nil {
		t.Fatal("expected error for malformed request")
	}
}

func TestReplayConn_ReadsReplayThenUnderlyingConn(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		client.Write([]byte("-world"))
	}()

	rc := &replayConn{
		Conn: server,
		r:    io.MultiReader(strings.NewReader("hello"), server),
	}

	buf := make([]byte, 11)
	n, err := io.ReadFull(rc, buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); got != "hello-world" {
		t.Fatalf("expected %q, got %q", "hello-world", got)
	}
}
