// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package id

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"regexp"
	"strings"
)

var chunkRegex = regexp.MustCompile("(.{7})")

// ID is the type representing a generated ID.
type ID [32]byte

// New generates a new ID from the given input bytes.
func New(data []byte) ID {
	var id ID

	hasher := sha256.New()
	hasher.Write(data)
	hasher.Sum(id[:0])

	return id
}

// String returns the canonical representation of the ID.
func (i ID) String() string {
	ss := base32.StdEncoding.EncodeToString(i[:])
	ss = strings.Trim(ss, "=")

	ss = chunkify(ss)

	return ss
}

// Implements the `TextUnmarshaler` interface from the encoding package.
func (i *ID) UnmarshalText(bs []byte) (err error) {
	// Convert to the canonical encoding - uppercase, no '=', no chunks, and
	// with any potential typos fixed.
	id := string(bs)
	id = strings.Trim(id, "=")
	id = strings.ToUpper(id)
	id = untypeoify(id)
	id = unchunkify(id)

	if len(id) != 52 {
		return errors.New("device ID invalid: incorrect length")
	}

	// Base32 decode
	dec, err := base32.StdEncoding.DecodeString(id + "====")
	if err != nil {
		return err
	}

	// Done!
	copy(i[:], dec)
	return nil
}

// Returns a string split into chunks of size 7.
func chunkify(s string) string {
	s = chunkRegex.ReplaceAllString(s, "$1-")
	s = strings.Trim(s, "-")
	return s
}

// Un-chunks a string by removing all hyphens and spaces.
func unchunkify(s string) string {
	s = strings.Replace(s, "-", "", -1)
	s = strings.Replace(s, " ", "", -1)
	return s
}

// We use base32 encoding, which uses 26 characters, and then the numbers
// 234567.  This is useful since the alphabet doesn't contain the numbers 0, 1,
// or 8, which means we can replace them with their letter-lookalikes.
func untypeoify(s string) string {
	s = strings.Replace(s, "0", "O", -1)
	s = strings.Replace(s, "1", "I", -1)
	s = strings.Replace(s, "8", "B", -1)
	return s
}
