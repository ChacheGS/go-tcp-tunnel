package id

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	data := []byte("test data")
	id := New(data)

	expected := sha256.Sum256(data)
	if id != ID(expected) {
		t.Fatalf("expected %x, got %x", expected, id)
	}
}

func TestNew_DifferentInput(t *testing.T) {
	t.Parallel()

	id1 := New([]byte("input1"))
	id2 := New([]byte("input2"))

	if id1 == id2 {
		t.Fatal("different inputs should produce different IDs")
	}
}

func TestNew_SameInput(t *testing.T) {
	t.Parallel()

	id1 := New([]byte("same"))
	id2 := New([]byte("same"))

	if id1 != id2 {
		t.Fatal("same inputs should produce same IDs")
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	id := New([]byte("test"))
	s := id.String()

	// Should not contain padding
	if strings.Contains(s, "=") {
		t.Fatal("string should not contain padding")
	}

	// Should be chunked with hyphens
	if !strings.Contains(s, "-") {
		t.Fatal("string should contain hyphens")
	}

	// Each chunk should be at most 7 characters
	chunks := strings.Split(s, "-")
	for i, chunk := range chunks {
		if i < len(chunks)-1 && len(chunk) != 7 {
			t.Fatalf("chunk %d has length %d, expected 7", i, len(chunk))
		}
		if len(chunk) > 7 {
			t.Fatalf("chunk %d has length %d, exceeds 7", i, len(chunk))
		}
	}
}

func TestUnmarshalText_Valid(t *testing.T) {
	t.Parallel()

	original := New([]byte("test"))
	s := original.String()

	var parsed ID
	if err := parsed.UnmarshalText([]byte(s)); err != nil {
		t.Fatal("UnmarshalText error:", err)
	}

	if original != parsed {
		t.Fatalf("expected %v, got %v", original, parsed)
	}
}

func TestUnmarshalText_TypoCorrections(t *testing.T) {
	t.Parallel()

	original := New([]byte("test"))
	s := original.String()

	// Replace O with 0, I with 1, B with 8
	typo := strings.ReplaceAll(s, "O", "0")
	typo = strings.ReplaceAll(typo, "I", "1")
	typo = strings.ReplaceAll(typo, "B", "8")

	var parsed ID
	if err := parsed.UnmarshalText([]byte(typo)); err != nil {
		t.Fatal("UnmarshalText error:", err)
	}

	if original != parsed {
		t.Fatalf("typo correction failed: expected %v, got %v", original, parsed)
	}
}

func TestUnmarshalText_Lowercase(t *testing.T) {
	t.Parallel()

	original := New([]byte("test"))
	s := strings.ToLower(original.String())

	var parsed ID
	if err := parsed.UnmarshalText([]byte(s)); err != nil {
		t.Fatal("UnmarshalText error:", err)
	}

	if original != parsed {
		t.Fatalf("lowercase parsing failed: expected %v, got %v", original, parsed)
	}
}

func TestUnmarshalText_InvalidLength(t *testing.T) {
	t.Parallel()

	var id ID
	err := id.UnmarshalText([]byte("TOOSHORT"))
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
	if !strings.Contains(err.Error(), "incorrect length") {
		t.Fatal("expected 'incorrect length' error, got:", err)
	}
}

func TestUnmarshalText_InvalidBase32(t *testing.T) {
	t.Parallel()

	// 52 characters but invalid base32 (contains '9' which is not in base32)
	var id ID
	err := id.UnmarshalText([]byte("9999999-9999999-9999999-9999999-9999999-9999999-999999"))
	// After untypeoify, 9 remains as 9, which is invalid base32
	// But 9 is not a valid base32 character, so it should fail at decode
	if err == nil {
		t.Fatal("expected error for invalid base32")
	}
}

func TestStringRoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"hello world",
		"",
		"a very long string that should still work correctly for testing",
		string([]byte{0, 1, 2, 3, 4, 5}),
	}

	for _, input := range inputs {
		original := New([]byte(input))
		s := original.String()

		var parsed ID
		if err := parsed.UnmarshalText([]byte(s)); err != nil {
			t.Fatalf("round trip failed for %q: %v", input, err)
		}

		if original != parsed {
			t.Fatalf("round trip mismatch for %q", input)
		}
	}
}

func TestChunkify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"ABCDEFGHIJKLMN", "ABCDEFG-HIJKLMN"},
		{"ABCDEFG", "ABCDEFG"},
		{"ABCDEFGHIJ", "ABCDEFG-HIJ"},
		{"", ""},
	}

	for _, tt := range tests {
		result := chunkify(tt.input)
		if result != tt.expected {
			t.Errorf("chunkify(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestUnchunkify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"ABCDEFG-HIJKLMN", "ABCDEFGHIJKLMN"},
		{"ABC DEF", "ABCDEF"},
		{"ABC-DEF-GHI", "ABCDEFGHI"},
		{"NOCHUNKS", "NOCHUNKS"},
	}

	for _, tt := range tests {
		result := unchunkify(tt.input)
		if result != tt.expected {
			t.Errorf("unchunkify(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestUntypeoify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"0", "O"},
		{"1", "I"},
		{"8", "B"},
		{"018", "OIB"},
		{"ABC", "ABC"},
		{"A0B1C8", "AOBICB"},
	}

	for _, tt := range tests {
		result := untypeoify(tt.input)
		if result != tt.expected {
			t.Errorf("untypeoify(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
