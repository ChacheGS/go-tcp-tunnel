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
