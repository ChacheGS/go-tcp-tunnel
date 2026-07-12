// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

// Package ca provides self-signed CA generation and CA-signed leaf
// certificate issuance for go-stream-tunnel's mTLS setup. It performs no file
// I/O; callers (see cmd/ca) are responsible for reading and writing the
// PEM-encoded bytes this package produces and consumes.
package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"strings"
	"time"
)

// GenerateCA creates a self-signed CA root: an ECDSA P256 keypair and a
// certificate with IsCA true, KeyUsageCertSign|KeyUsageCRLSign, and
// BasicConstraintsValid true, valid for the given duration starting now.
// Returns the certificate and key both PEM-encoded.
func GenerateCA(subject string, validity time.Duration) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA key: %s", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: subject},
		NotBefore:             now,
		NotAfter:              now.Add(validity),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %s", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal CA key: %s", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %s", err)
	}
	return serial, nil
}

// IssueCert signs a new leaf keypair (ECDSA P256) using the given CA's
// certificate and key (both PEM-encoded, as produced by GenerateCA). sans
// may be empty for a client-role cert; a server-role cert needs at least
// one DNS name or IP address matching the address clients will dial —
// entries that parse as an IP become an IP SAN, everything else becomes a
// DNS SAN. Every issued leaf carries both ExtKeyUsageServerAuth and
// ExtKeyUsageClientAuth, so one certificate works for either role.
func IssueCert(caCertPEM, caKeyPEM []byte, name string, sans []string, validity time.Duration) (certPEM, keyPEM []byte, err error) {
	caCert, caKey, err := parseCAKeyPair(caCertPEM, caKeyPEM)
	if err != nil {
		return nil, nil, err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate certificate key: %s", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now,
		NotAfter:     now.Add(validity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	for _, san := range sans {
		if ip := net.ParseIP(san); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		if !isValidDNSName(san) {
			return nil, nil, fmt.Errorf("invalid SAN %q: not a valid IP address or DNS name", san)
		}
		template.DNSNames = append(template.DNSNames, san)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal certificate key: %s", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// dnsLabelRE matches a single valid DNS label: letters, digits, and
// hyphens, 1-63 chars, no leading/trailing hyphen.
var dnsLabelRE = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// isValidDNSName reports whether name is a syntactically valid DNS name (a
// dot-separated sequence of valid labels), catching typos in -addr before
// they end up silently embedded in an otherwise "successfully" issued
// certificate that will only fail much later, at TLS handshake time.
func isValidDNSName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if !dnsLabelRE.MatchString(label) {
			return false
		}
	}
	return true
}

func parseCAKeyPair(caCertPEM, caKeyPEM []byte) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certBlock, _ := pem.Decode(caCertPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %s", err)
	}
	if !caCert.IsCA {
		return nil, nil, fmt.Errorf("certificate is not a CA (IsCA false); did you point -ca-dir at an issued leaf cert instead of the CA?")
	}

	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("failed to decode CA key PEM")
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA key: %s", err)
	}

	return caCert, caKey, nil
}
