// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package proto

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestControlMessage_AllHeadersMissing(t *testing.T) {
	t.Parallel()

	r := http.Request{}
	r.Header = http.Header{}

	_, err := ReadControlMessage(&r)
	if err == nil {
		t.Fatal("expected error for all missing headers")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, HeaderAction) {
		t.Errorf("expected error to mention %s", HeaderAction)
	}
	if !strings.Contains(errStr, HeaderForwardedHost) {
		t.Errorf("expected error to mention %s", HeaderForwardedHost)
	}
	if !strings.Contains(errStr, HeaderForwardedProto) {
		t.Errorf("expected error to mention %s", HeaderForwardedProto)
	}
}

func TestControlMessageWriteRead(t *testing.T) {
	t.Parallel()

	data := []struct {
		msg *ControlMessage
		err error
	}{
		{
			&ControlMessage{
				Action:         "action",
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			nil,
		},
		{
			&ControlMessage{
				ForwardedHost:  "forwarded_host",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Action]"),
		},
		{
			&ControlMessage{
				Action:        "action",
				ForwardedHost: "forwarded_host",
			},
			errors.New("missing headers: [X-Forwarded-Proto]"),
		},
		{
			&ControlMessage{
				Action:         "action",
				ForwardedProto: "forwarded_proto",
			},
			errors.New("missing headers: [X-Forwarded-Host]"),
		},
	}

	for i, tt := range data {
		r := http.Request{}
		r.Header = http.Header{}
		tt.msg.WriteToHeader(r.Header)

		actual, err := ReadControlMessage(&r)
		if tt.err != nil {
			if err == nil {
				t.Error(i, "expected error")
			} else if tt.err.Error() != err.Error() {
				t.Error(i, tt.err, err)
			}
		} else {
			if !reflect.DeepEqual(tt.msg, actual) {
				t.Error(i, tt.msg, actual)
			}
		}
	}
}

func TestHTTPProtocolConstant(t *testing.T) {
	t.Parallel()

	if HTTP != "http" {
		t.Fatalf(`expected HTTP constant to equal "http", got %q`, HTTP)
	}
}

func TestHeaderTunnelInfoConstant(t *testing.T) {
	t.Parallel()

	if HeaderTunnelInfo != "X-Tunnel-Info" {
		t.Fatalf(`expected HeaderTunnelInfo to equal "X-Tunnel-Info", got %q`, HeaderTunnelInfo)
	}
}
