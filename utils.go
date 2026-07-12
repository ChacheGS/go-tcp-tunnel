// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/ChacheGS/go-stream-tunnel/log"
)

func transfer(dst io.Writer, src io.Reader, logger log.Logger) {
	n, err := io.Copy(dst, src)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
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

// CheckPrivateKeyPermissions verifies that the file at path is not
// readable or writable by group or other, returning an error describing
// the problem if it is. This guards against using a private key file
// whose permissions have been loosened after the fact — by a backup
// restore, a misconfigured deployment, or a stray chmod — mirroring the
// same class of check OpenSSH performs before using a private key.
//
// Not enforced on Windows: Go's os.FileMode there doesn't reliably map
// to the ACL-based permissions Windows actually uses, so a Unix-style
// bitmask check would produce false positives (or false confidence)
// rather than a meaningful guarantee.
func CheckPrivateKeyPermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if perm := info.Mode().Perm(); perm&0077 != 0 {
		return fmt.Errorf("permissions %04o for %q are too open: private key files must not be readable or writable by group or other (try: chmod 600 %s)", perm, path, path)
	}

	return nil
}
