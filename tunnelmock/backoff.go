// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/jlandowner/go-tcp-tunnel (interfaces: Backoff)

package tunnelmock

import (
	gomock "github.com/golang/mock/gomock"
	time "time"
)

// Mock of Backoff interface
type MockBackoff struct {
	ctrl     *gomock.Controller
	recorder *_MockBackoffRecorder
}

// Recorder for MockBackoff (not exported)
type _MockBackoffRecorder struct {
	mock *MockBackoff
}

func NewMockBackoff(ctrl *gomock.Controller) *MockBackoff {
	mock := &MockBackoff{ctrl: ctrl}
	mock.recorder = &_MockBackoffRecorder{mock}
	return mock
}

func (_m *MockBackoff) EXPECT() *_MockBackoffRecorder {
	return _m.recorder
}

func (_m *MockBackoff) NextBackOff() time.Duration {
	ret := _m.ctrl.Call(_m, "NextBackOff")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

func (_mr *_MockBackoffRecorder) NextBackOff() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "NextBackOff")
}

func (_m *MockBackoff) Reset() {
	_m.ctrl.Call(_m, "Reset")
}

func (_mr *_MockBackoffRecorder) Reset() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Reset")
}
