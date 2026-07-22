// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"net"
	"time"
)

var (
	// DefaultKeepAliveIdleTime specifies how long a connection can be idle
	// before the first keepalive probe is sent.
	DefaultKeepAliveIdleTime = 5 * time.Second
	// DefaultKeepAliveCount specifies the maximum number of keepalive
	// probes that can go unanswered before the connection is dropped.
	DefaultKeepAliveCount = 8
	// DefaultKeepAliveInterval specifies the time between keepalive
	// probes once the idle period has elapsed.
	DefaultKeepAliveInterval = 5 * time.Second
)

// keepAlive configures OS-level TCP keepalive on conn using explicit
// idle/interval/count values (via SetKeepAliveConfig, not the older
// SetKeepAlivePeriod) so control connections left idle for long stretches
// still probe often enough to refresh NAT/firewall connection state and
// notice a silently dropped peer within roughly
// DefaultKeepAliveIdleTime + DefaultKeepAliveCount*DefaultKeepAliveInterval
// (about 45s with the defaults above), rather than relying on the OS's own
// defaults, which on Linux take on the order of minutes.
func keepAlive(conn *net.TCPConn) error {
	return conn.SetKeepAliveConfig(net.KeepAliveConfig{
		Enable:   true,
		Idle:     DefaultKeepAliveIdleTime,
		Interval: DefaultKeepAliveInterval,
		Count:    DefaultKeepAliveCount,
	})
}
