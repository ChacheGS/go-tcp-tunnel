// Copyright (C) 2017 Michał Matczuk
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"io"
	"net/http"
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
