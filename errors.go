// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import "errors"

var (
	errClientNotSubscribed    = errors.New("client not subscribed")
	errClientNotConnected     = errors.New("client not connected")
	errClientAlreadyConnected = errors.New("client already connected")
)
