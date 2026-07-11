// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestCommand_Defaults(t *testing.T) {
	cmd := Command()
	if err := cmd.Parse(nil); err != nil {
		t.Fatal(err)
	}

	if opts.caDir != "ca" {
		t.Fatalf("expected default ca-dir 'ca', got %s", opts.caDir)
	}
	if opts.name != "" {
		t.Fatalf("expected default name empty, got %s", opts.name)
	}
	if opts.addr != "" {
		t.Fatalf("expected default addr empty, got %s", opts.addr)
	}
	if opts.outDir != "" {
		t.Fatalf("expected default out-dir empty, got %s", opts.outDir)
	}
}

func TestCommand_CustomFlags(t *testing.T) {
	cmd := Command()
	args := []string{
		"-ca-dir", "custom-ca",
		"-name", "laptop",
		"-addr", "tunnel.example.com",
		"-out-dir", "custom-out",
	}
	if err := cmd.Parse(args); err != nil {
		t.Fatal(err)
	}

	if opts.caDir != "custom-ca" {
		t.Fatalf("expected ca-dir custom-ca, got %s", opts.caDir)
	}
	if opts.name != "laptop" {
		t.Fatalf("expected name laptop, got %s", opts.name)
	}
	if opts.addr != "tunnel.example.com" {
		t.Fatalf("expected addr tunnel.example.com, got %s", opts.addr)
	}
	if opts.outDir != "custom-out" {
		t.Fatalf("expected out-dir custom-out, got %s", opts.outDir)
	}
}

func TestCompleteArgs_Init(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"init"})

	if err := CompleteArgs(cmd); err != nil {
		t.Fatal(err)
	}
	if opts.command != "init" {
		t.Fatalf("expected command init, got %s", opts.command)
	}
}

func TestCompleteArgs_InitRejectsExtraArgs(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"init", "extra"})

	if err := CompleteArgs(cmd); err == nil {
		t.Fatal("expected error for init with extra arguments")
	}
}

func TestCompleteArgs_IssueRequiresName(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"issue"})

	if err := CompleteArgs(cmd); err == nil {
		t.Fatal("expected error when issue is missing -name")
	}
}

func TestCompleteArgs_IssueDefaultsOutDirToName(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"-name", "laptop", "issue"})

	if err := CompleteArgs(cmd); err != nil {
		t.Fatal(err)
	}
	if opts.outDir != "laptop" {
		t.Fatalf("expected out-dir to default to 'laptop', got %s", opts.outDir)
	}
}

func TestCompleteArgs_IssueRespectsExplicitOutDir(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"-name", "laptop", "-out-dir", "somewhere-else", "issue"})

	if err := CompleteArgs(cmd); err != nil {
		t.Fatal(err)
	}
	if opts.outDir != "somewhere-else" {
		t.Fatalf("expected out-dir somewhere-else, got %s", opts.outDir)
	}
}

func TestCompleteArgs_UnknownCommand(t *testing.T) {
	cmd := Command()
	cmd.Parse([]string{"bogus"})

	if err := CompleteArgs(cmd); err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestExecuteInit_CreatesCAFiles(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"

	Command()
	opts.caDir = caDir
	opts.command = "init"

	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	crtInfo, err := os.Stat(caDir + "/ca.crt")
	if err != nil {
		t.Fatalf("expected ca.crt to be created: %v", err)
	}
	if perm := crtInfo.Mode().Perm(); perm != 0644 {
		t.Fatalf("expected ca.crt to be mode 0644, got %o", perm)
	}

	keyInfo, err := os.Stat(caDir + "/ca.key")
	if err != nil {
		t.Fatalf("expected ca.key to be created: %v", err)
	}
	if perm := keyInfo.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected ca.key to be mode 0600, got %o", perm)
	}
}

func TestExecuteInit_CleansUpCertIfKeyWriteFails(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"

	if err := os.MkdirAll(caDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create ca.key as a directory so os.WriteFile fails writing the key,
	// while ca.crt (a different filename) writes successfully first.
	if err := os.Mkdir(caDir+"/ca.key", 0755); err != nil {
		t.Fatal(err)
	}

	Command()
	opts.caDir = caDir
	opts.command = "init"

	if err := Execute(); err == nil {
		t.Fatal("expected error when the key file can't be written")
	}

	if _, err := os.Stat(caDir + "/ca.crt"); !os.IsNotExist(err) {
		t.Fatalf("expected ca.crt to be cleaned up after the key write failed, stat error: %v", err)
	}
}

func TestExecuteInit_RefusesToOverwriteExistingCert(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"

	Command()
	opts.caDir = caDir
	opts.command = "init"

	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	// Second init in the same directory must fail rather than regenerate.
	err := Execute()
	if err == nil {
		t.Fatal("expected error when ca.crt already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}
}

func TestExecuteIssue_CreatesCertFiles(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"
	outDir := dir + "/laptop"

	Command()
	opts.caDir = caDir
	opts.command = "init"
	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	opts.command = "issue"
	opts.name = "laptop"
	opts.outDir = outDir
	opts.addr = ""

	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	crtInfo, err := os.Stat(outDir + "/tls.crt")
	if err != nil {
		t.Fatalf("expected tls.crt to be created: %v", err)
	}
	if perm := crtInfo.Mode().Perm(); perm != 0644 {
		t.Fatalf("expected tls.crt to be mode 0644, got %o", perm)
	}

	keyInfo, err := os.Stat(outDir + "/tls.key")
	if err != nil {
		t.Fatalf("expected tls.key to be created: %v", err)
	}
	if perm := keyInfo.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected tls.key to be mode 0600, got %o", perm)
	}
}

func TestExecuteIssue_CleansUpCertIfKeyWriteFails(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"
	outDir := dir + "/laptop"

	Command()
	opts.caDir = caDir
	opts.command = "init"
	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create tls.key as a directory so os.WriteFile fails writing the key,
	// while tls.crt (a different filename) writes successfully first.
	if err := os.Mkdir(outDir+"/tls.key", 0755); err != nil {
		t.Fatal(err)
	}

	opts.command = "issue"
	opts.name = "laptop"
	opts.outDir = outDir
	opts.addr = ""

	if err := Execute(); err == nil {
		t.Fatal("expected error when the key file can't be written")
	}

	if _, err := os.Stat(outDir + "/tls.crt"); !os.IsNotExist(err) {
		t.Fatalf("expected tls.crt to be cleaned up after the key write failed, stat error: %v", err)
	}
}

func TestExecuteIssue_MissingCADirectory(t *testing.T) {
	dir := t.TempDir()

	Command()
	opts.caDir = dir + "/nonexistent-ca"
	opts.command = "issue"
	opts.name = "laptop"
	opts.outDir = dir + "/laptop"

	err := Execute()
	if err == nil {
		t.Fatal("expected error when CA directory doesn't exist")
	}
	if !strings.Contains(err.Error(), "ca init") {
		t.Fatalf("expected error to point at 'ca init', got: %v", err)
	}
}

func TestExecuteIssue_RefusesToOverwriteExistingCert(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"
	outDir := dir + "/laptop"

	Command()
	opts.caDir = caDir
	opts.command = "init"
	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	opts.command = "issue"
	opts.name = "laptop"
	opts.outDir = outDir
	opts.addr = ""

	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	// Second issue into the same out-dir must fail rather than overwrite.
	err := Execute()
	if err == nil {
		t.Fatal("expected error when tls.crt already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' error, got: %v", err)
	}
}

func TestExecuteIssue_PrintsClientIDFingerprint(t *testing.T) {
	dir := t.TempDir()
	caDir := dir + "/ca"
	outDir := dir + "/laptop"

	Command()
	opts.caDir = caDir
	opts.command = "init"
	if err := Execute(); err != nil {
		t.Fatal(err)
	}

	opts.command = "issue"
	opts.name = "laptop"
	opts.outDir = outDir
	opts.addr = ""

	stdout := captureStdout(t, func() {
		if err := Execute(); err != nil {
			t.Fatal(err)
		}
	})

	if !strings.Contains(stdout, "Client ID (for -client-ids):") {
		t.Fatalf("expected output to contain the client ID fingerprint, got: %s", stdout)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever was written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
