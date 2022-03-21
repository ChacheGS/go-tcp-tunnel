// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/jlandowner/go-http-tunnel/log"
)

func transfer(dst io.Writer, src io.Reader, logger log.Logger) {
	n, err := io.Copy(dst, src)
	if err != nil {
		if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "CANCEL") {
			logger.Log(
				"level", 2,
				"msg", "copy error",
				"err", err,
			)
		}
	}

	logger.Log(
		"level", 3,
		"action", "transferred",
		"bytes", n,
	)
}

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func NormalizeAddress(addr string) (string, error) {
	// normalize port to addr
	if _, err := strconv.Atoi(addr); err == nil {
		addr = ":" + addr
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}

	if host == "" {
		host = "127.0.0.1"
	}

	return fmt.Sprintf("%s:%s", host, port), nil
}
