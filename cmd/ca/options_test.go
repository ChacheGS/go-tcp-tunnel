// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
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
