// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"

	"github.com/ChacheGS/go-stream-tunnel/id"
)

type connPair struct {
	conn       net.Conn
	clientConn *http2.ClientConn
}

type connPool struct {
	t     *http2.Transport
	conns map[string]connPair // key is host:port
	free  func(identifier id.ID)
	mu    sync.RWMutex
}

func newConnPool(t *http2.Transport, f func(identifier id.ID)) *connPool {
	return &connPool{
		t:     t,
		free:  f,
		conns: make(map[string]connPair),
	}
}

func (p *connPool) URL(identifier id.ID) string {
	return fmt.Sprint("https://", identifier)
}

func (p *connPool) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	p.mu.RLock()
	cp, ok := p.conns[addr]
	p.mu.RUnlock()

	if !ok {
		return nil, errClientNotConnected
	}
	if cp.clientConn.CanTakeNewRequest() {
		return cp.clientConn, nil
	}

	// cp is still registered but can no longer accept new streams (e.g.
	// it received a GOAWAY, or a stream was never cleaned up and it's
	// stuck at its peer's concurrent-stream limit). Left alone, every
	// future request for this identifier would hit the same error
	// forever, since nothing else periodically re-checks a cached
	// connection's health once it's in the pool. Evict it now: closing
	// cp.conn also closes the tunnel client's end of the same physical
	// socket, so the client notices and reconnects on its own instead of
	// requiring a human to restart it.
	p.mu.Lock()
	if cur, ok := p.conns[addr]; ok && cur.clientConn == cp.clientConn {
		p.close(cur, addr)
	}
	p.mu.Unlock()

	return nil, errClientNotConnected
}

func (p *connPool) MarkDead(c *http2.ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for addr, cp := range p.conns {
		if cp.clientConn == c {
			p.close(cp, addr)
			return
		}
	}
}

func (p *connPool) AddConn(conn net.Conn, identifier id.ID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		if err := p.ping(cp); err != nil {
			p.close(cp, addr)
		} else {
			return errClientAlreadyConnected
		}
	}

	c, err := p.t.NewClientConn(conn)
	if err != nil {
		conn.Close()
		return err
	}
	p.conns[addr] = connPair{
		conn:       conn,
		clientConn: c,
	}

	return nil
}

func (p *connPool) DeleteConn(identifier id.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		p.close(cp, addr)
	}
}

func (p *connPool) Ping(identifier id.ID) (time.Duration, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	addr := p.addr(identifier)

	if cp, ok := p.conns[addr]; ok {
		start := time.Now()
		err := p.ping(cp)
		return time.Since(start), err
	}

	return 0, errClientNotConnected
}

func (p *connPool) ping(cp connPair) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPingTimeout)
	defer cancel()

	return cp.clientConn.Ping(ctx)
}

func (p *connPool) close(cp connPair, addr string) {
	cp.conn.Close()
	delete(p.conns, addr)
	if p.free != nil {
		p.free(p.identifier(addr))
	}
}

func (p *connPool) addr(identifier id.ID) string {
	return fmt.Sprint(identifier.String(), ":443")
}

func (p *connPool) identifier(addr string) id.ID {
	var identifier id.ID
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return identifier
	}

	if err := identifier.UnmarshalText([]byte(host)); err != nil {
		return identifier
	}
	return identifier
}
