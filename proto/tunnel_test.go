// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

import "testing"

func TestValidSubdomainLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		label string
		want  bool
	}{
		{"simple", "myapp", true},
		{"with digits", "app123", true},
		{"with hyphen", "my-app", true},
		{"single char", "a", true},
		{"single digit", "1", true},
		{"empty", "", false},
		{"uppercase", "MyApp", false},
		{"contains dot", "my.app", false},
		{"leading hyphen", "-myapp", false},
		{"trailing hyphen", "myapp-", false},
		{"underscore", "my_app", false},
		{"space", "my app", false},
		{"too long", string(make([]byte, 64)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidSubdomainLabel(tt.label)
			if got != tt.want {
				t.Fatalf("ValidSubdomainLabel(%q) = %v, want %v", tt.label, got, tt.want)
			}
		})
	}
}
