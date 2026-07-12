// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ChacheGS/go-stream-tunnel/tunnelmock"
)

func TestNopLogger_Log(t *testing.T) {
	t.Parallel()

	logger := NewNopLogger()
	if err := logger.Log("key", "val"); err != nil {
		t.Fatal("NopLogger.Log should return nil, got:", err)
	}
}

func TestStdLogger_Log(t *testing.T) {
	t.Parallel()

	logger := NewStdLogger()
	// Should not panic or error
	if err := logger.Log("key", "val", "num", 42); err != nil {
		t.Fatal("StdLogger.Log should return nil, got:", err)
	}
}

func TestContext_Log(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	b.EXPECT().Log("key", "val", "sufix", "")
	NewContext(b).With("sufix", "").Log("key", "val")

	b.EXPECT().Log("prefix", "", "key", "val")
	NewContext(b).WithPrefix("prefix", "").Log("key", "val")

	b.EXPECT().Log("prefix", "", "key", "val", "sufix", "")
	NewContext(b).With("sufix", "").WithPrefix("prefix", "").Log("key", "val")
}
