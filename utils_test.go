// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestCheckPrivateKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningfully enforced on windows")
	}

	tests := []struct {
		name    string
		mode    os.FileMode
		wantErr bool
	}{
		{name: "0600 is fine", mode: 0600, wantErr: false},
		{name: "0400 is fine", mode: 0400, wantErr: false},
		{name: "0644 is too open (world-readable)", mode: 0644, wantErr: true},
		{name: "0640 is too open (group-readable)", mode: 0640, wantErr: true},
		{name: "0666 is too open", mode: 0666, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tls.key")
			if err := os.WriteFile(path, []byte("fake key material"), tt.mode); err != nil {
				t.Fatal(err)
			}

			err := CheckPrivateKeyPermissions(path)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for mode %04o, got nil", tt.mode)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error for mode %04o, got: %v", tt.mode, err)
			}
		})
	}
}

func TestCheckPrivateKeyPermissions_MissingFile(t *testing.T) {
	err := CheckPrivateKeyPermissions("/nonexistent/tls.key")
	if err == nil {
		t.Fatal("expected error for a missing file")
	}
}
