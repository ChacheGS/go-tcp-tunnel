// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package ca

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM, err := GenerateCA("test CA", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatal("expected a CERTIFICATE PEM block")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if !cert.IsCA {
		t.Fatal("expected IsCA true")
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatal("expected KeyUsageCertSign")
	}
	if cert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Fatal("expected KeyUsageCRLSign")
	}
	if !cert.BasicConstraintsValid {
		t.Fatal("expected BasicConstraintsValid true")
	}
	if cert.Subject.CommonName != "test CA" {
		t.Fatalf("expected CommonName %q, got %q", "test CA", cert.Subject.CommonName)
	}

	// A self-signed root must verify against a pool containing itself.
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		t.Fatalf("expected CA to verify against itself: %v", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatal("expected an EC PRIVATE KEY PEM block")
	}
	if _, err := x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
		t.Fatalf("expected key to parse as an EC private key: %v", err)
	}
}

func TestIssueCert_ChainVerifiesAgainstCA(t *testing.T) {
	t.Parallel()

	caCertPEM, caKeyPEM, err := GenerateCA("test CA", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	certPEM, keyPEM, err := IssueCert(caCertPEM, caKeyPEM, "myapp", nil, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	leafBlock, _ := pem.Decode(certPEM)
	if leafBlock == nil || leafBlock.Type != "CERTIFICATE" {
		t.Fatal("expected a CERTIFICATE PEM block")
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if leaf.Subject.CommonName != "myapp" {
		t.Fatalf("expected CommonName %q, got %q", "myapp", leaf.Subject.CommonName)
	}
	if leaf.IsCA {
		t.Fatal("expected issued leaf to not be a CA")
	}

	wantUsages := map[x509.ExtKeyUsage]bool{
		x509.ExtKeyUsageServerAuth: false,
		x509.ExtKeyUsageClientAuth: false,
	}
	for _, u := range leaf.ExtKeyUsage {
		wantUsages[u] = true
	}
	for usage, found := range wantUsages {
		if !found {
			t.Fatalf("expected ExtKeyUsage to include %v, got %v", usage, leaf.ExtKeyUsage)
		}
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		t.Fatalf("expected leaf to chain-verify against the CA: %v", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatal("expected an EC PRIVATE KEY PEM block")
	}
	if _, err := x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
		t.Fatalf("expected key to parse as an EC private key: %v", err)
	}
}

func TestIssueCert_WithSANs(t *testing.T) {
	t.Parallel()

	caCertPEM, caKeyPEM, err := GenerateCA("test CA", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	certPEM, _, err := IssueCert(caCertPEM, caKeyPEM, "myserver", []string{"tunnel.example.com", "127.0.0.1"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(certPEM)
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "tunnel.example.com" {
		t.Fatalf("expected DNSNames [tunnel.example.com], got %v", leaf.DNSNames)
	}
	if len(leaf.IPAddresses) != 1 || leaf.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatalf("expected IPAddresses [127.0.0.1], got %v", leaf.IPAddresses)
	}
}

func TestIssueCert_InvalidCAPEM(t *testing.T) {
	t.Parallel()

	_, _, err := IssueCert([]byte("not a valid PEM"), []byte("also not valid"), "myapp", nil, time.Hour)
	if err == nil {
		t.Fatal("expected error for invalid CA PEM")
	}
}
