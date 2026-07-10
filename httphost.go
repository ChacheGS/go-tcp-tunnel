// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
)

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
