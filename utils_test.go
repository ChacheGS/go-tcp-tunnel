// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"strings"
	"testing"
)

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr     string
		expected string
		error    string
	}{
		{
			addr:     "22",
			expected: "127.0.0.1:22",
		},
		{
			addr:     ":22",
			expected: "127.0.0.1:22",
		},
		{
			addr:     "0.0.0.0:22",
			expected: "0.0.0.0:22",
		},
		{
			addr:  "0.0.0.0",
			error: "missing port",
		},
		{
			addr:  "",
			error: "missing port",
		},
	}

	for i, tt := range tests {
		actual, err := NormalizeAddress(tt.addr)
		if actual != tt.expected {
			t.Errorf("[%d] expected %q got %q err: %s", i, tt.expected, actual, err)
		}
		if tt.error != "" && err == nil {
			t.Errorf("[%d] expected error", i)
		}
		if err != nil && (tt.error == "" || !strings.Contains(err.Error(), tt.error)) {
			t.Errorf("[%d] expected error contains %q, got %q", i, tt.error, err)
		}
	}
}
