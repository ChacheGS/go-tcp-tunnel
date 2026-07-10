// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/jlandowner/go-tcp-tunnel/tunnelmock"
)

func TestFilterLogger_Log(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 2)
	b.EXPECT().Log("level", 0)
	f.Log("level", 0)
	b.EXPECT().Log("level", 1)
	f.Log("level", 1)
	b.EXPECT().Log("level", 2)
	f.Log("level", 2)

	f.Log("level", 3)
	f.Log("level", 4)
}

func TestFilterLogger_NoLevelKey(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 1)

	// Message without "level" key should pass through
	b.EXPECT().Log("msg", "hello")
	f.Log("msg", "hello")
}

func TestFilterLogger_NonStringKey(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 1)

	// Non-string key should be skipped, message passes through
	b.EXPECT().Log(42, "value")
	f.Log(42, "value")
}

func TestFilterLogger_NonIntLevel(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 1)

	// "level" with non-int value should pass through
	b.EXPECT().Log("level", "info")
	f.Log("level", "info")
}

func TestFilterLogger_OddKeyvals(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	b := tunnelmock.NewMockLogger(ctrl)
	f := NewFilterLogger(b, 1)

	// "level" as last key with no value — should pass through
	b.EXPECT().Log("level")
	f.Log("level")
}
