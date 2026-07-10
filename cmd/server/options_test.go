// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommand_Defaults(t *testing.T) {
	cmd := Command()
	if err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}

	if opts.tunnelAddr != ":5223" {
		t.Fatalf("expected default addr :5223, got %s", opts.tunnelAddr)
	}
	if opts.tlsCrt != "tls.crt" {
		t.Fatalf("expected default tls-crt tls.crt, got %s", opts.tlsCrt)
	}
	if opts.tlsKey != "tls.key" {
		t.Fatalf("expected default tls-key tls.key, got %s", opts.tlsKey)
	}
	if opts.clientCA != "ca.crt" {
		t.Fatalf("expected default ca-crt ca.crt, got %s", opts.clientCA)
	}
	if opts.clientIDs != "" {
		t.Fatalf("expected default client-ids empty, got %s", opts.clientIDs)
	}
	if opts.baseDomain != "" {
		t.Fatalf("expected default base-domain empty, got %s", opts.baseDomain)
	}
	if opts.httpAddr != "127.0.0.1:9000" {
		t.Fatalf("expected default http-addr 127.0.0.1:9000, got %s", opts.httpAddr)
	}
	if opts.logLevel != 1 {
		t.Fatalf("expected default log-level 1, got %d", opts.logLevel)
	}
}

func TestCommand_CustomFlags(t *testing.T) {
	cmd := Command()
	args := []string{
		"-addr", "127.0.0.1:1234",
		"-tls-crt", "custom.crt",
		"-tls-key", "custom.key",
		"-ca-crt", "custom-ca.crt",
		"-client-ids", "ID1,ID2",
		"-base-domain", "tunnel.example.com",
		"-http-addr", "127.0.0.1:9001",
		"-log-level", "3",
	}
	if err := cmd.Parse(args); err != nil {
		t.Fatal(err)
	}

	if opts.tunnelAddr != "127.0.0.1:1234" {
		t.Fatalf("expected addr 127.0.0.1:1234, got %s", opts.tunnelAddr)
	}
	if opts.tlsCrt != "custom.crt" {
		t.Fatalf("expected tls-crt custom.crt, got %s", opts.tlsCrt)
	}
	if opts.tlsKey != "custom.key" {
		t.Fatalf("expected tls-key custom.key, got %s", opts.tlsKey)
	}
	if opts.clientCA != "custom-ca.crt" {
		t.Fatalf("expected ca-crt custom-ca.crt, got %s", opts.clientCA)
	}
	if opts.clientIDs != "ID1,ID2" {
		t.Fatalf("expected client-ids ID1,ID2, got %s", opts.clientIDs)
	}
	if opts.baseDomain != "tunnel.example.com" {
		t.Fatalf("expected base-domain tunnel.example.com, got %s", opts.baseDomain)
	}
	if opts.httpAddr != "127.0.0.1:9001" {
		t.Fatalf("expected http-addr 127.0.0.1:9001, got %s", opts.httpAddr)
	}
	if opts.logLevel != 3 {
		t.Fatalf("expected log-level 3, got %d", opts.logLevel)
	}
}

func TestTLSConfig_MissingCertFile(t *testing.T) {
	Command()
	opts.tlsCrt = "/nonexistent/tls.crt"
	opts.tlsKey = "/nonexistent/tls.key"

	_, err := tlsConfig()
	if err == nil {
		t.Fatal("expected error for missing cert file")
	}
}

func TestTLSConfig_MissingClientCA(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = ""

	_, err := tlsConfig()
	if err == nil {
		t.Fatal("expected error for missing client CA")
	}
	if !strings.Contains(err.Error(), "no client CA is given") {
		t.Fatalf("expected 'no client CA is given' error, got: %v", err)
	}
}

func TestTLSConfig_CACertFileNotFound(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = "/nonexistent/ca.crt"

	_, err := tlsConfig()
	if err == nil {
		t.Fatal("expected error for missing CA file")
	}
}

func TestTLSConfig_InvalidCAPEM(t *testing.T) {
	dir := t.TempDir()
	badCA := filepath.Join(dir, "bad-ca.crt")
	if err := os.WriteFile(badCA, []byte("not a valid PEM certificate"), 0644); err != nil {
		t.Fatal(err)
	}

	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = badCA

	_, err := tlsConfig()
	if err == nil {
		t.Fatal("expected error for invalid CA PEM")
	}
	if !strings.Contains(err.Error(), "failed to parse CA certificate PEM") {
		t.Fatalf("expected PEM parse error, got: %v", err)
	}
}

func TestTLSConfig_Success(t *testing.T) {
	Command()
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = "../../testdata/selfsigned.crt"

	conf, err := tlsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if conf == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if len(conf.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(conf.Certificates))
	}
	if conf.ClientCAs == nil {
		t.Fatal("expected non-nil ClientCAs pool")
	}
	if len(conf.NextProtos) != 1 || conf.NextProtos[0] != "h2" {
		t.Fatalf("expected NextProtos [h2], got %v", conf.NextProtos)
	}
}

func TestExecute_TLSConfigError(t *testing.T) {
	Command()
	opts.tlsCrt = "/nonexistent/tls.crt"
	opts.tlsKey = "/nonexistent/tls.key"

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error when tls config fails")
	}
	if !strings.Contains(err.Error(), "failed to configure tls") {
		t.Fatalf("expected tls configuration error, got: %v", err)
	}
}

func TestExecute_EmptyClientID(t *testing.T) {
	Command()
	opts.tunnelAddr = "127.0.0.1:0"
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = "../../testdata/selfsigned.crt"
	opts.clientIDs = ","

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for empty client id in list")
	}
	if !strings.Contains(err.Error(), "empty client id") {
		t.Fatalf("expected 'empty client id' error, got: %v", err)
	}
}

func TestExecute_InvalidClientID(t *testing.T) {
	Command()
	opts.tunnelAddr = "127.0.0.1:0"
	opts.tlsCrt = "../../testdata/selfsigned.crt"
	opts.tlsKey = "../../testdata/selfsigned.key"
	opts.clientCA = "../../testdata/selfsigned.crt"
	opts.clientIDs = "not-a-valid-id"

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid client id")
	}
	if !strings.Contains(err.Error(), "invalid identifier") {
		t.Fatalf("expected 'invalid identifier' error, got: %v", err)
	}
}
