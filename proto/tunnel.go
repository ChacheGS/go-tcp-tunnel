// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

import "regexp"

// Tunnel describes a single tunnel between client and server. When connecting
// client sends tunnels to server. If client gets connected server proxies
// connections to given Host and Addr to the client.
type Tunnel struct {
	// Protocol specifies tunnel protocol, must be one of protocols known
	// by the server.
	Protocol string
	// Host specified HTTP request host, it's required for HTTP and WS
	// tunnels.
	Host string
	// Auth specifies HTTP basic auth credentials in form "user:password",
	// if set server would protect HTTP and WS tunnels with basic auth.
	Auth string
	// Addr specifies TCP address server would listen on, it's required
	// for TCP tunnels.
	Addr string
}

// subdomainLabelRE matches a single valid DNS label: lowercase letters,
// digits and hyphens, 1-63 chars, no leading/trailing hyphen. This is
// intentionally stricter than a full DNS name (no dots allowed) since a
// Tunnel's Host is meant to be one subdomain label, not a nested hostname.
var subdomainLabelRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidSubdomainLabel reports whether s is a valid single DNS label suitable
// for use as a Tunnel's Host (subdomain slug). It rejects empty strings,
// uppercase letters, dots, and other characters that are not valid in a DNS
// label, catching config mistakes (e.g. a stray dot nesting an unintended
// subdomain) before they're accepted by client or server.
func ValidSubdomainLabel(s string) bool {
	return subdomainLabelRE.MatchString(s)
}
