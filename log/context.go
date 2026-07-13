// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Copyright (C) 2026 ChacheGS
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package log

// Context is simplified version of go-kit log Context
// https://godoc.org/github.com/go-kit/kit/log#Context.
type Context struct {
	prefix []any
	suffix []any
	logger Logger
}

// NewContext returns a logger that adds prefix before keyvals.
func NewContext(logger Logger) *Context {
	return &Context{
		prefix: make([]any, 0),
		suffix: make([]any, 0),
		logger: logger,
	}
}

// With returns a new Context with keyvals appended to those of the receiver.
func (c *Context) With(keyvals ...any) *Context {
	return &Context{
		prefix: c.prefix,
		suffix: append(c.suffix, keyvals...),
		logger: c.logger,
	}
}

// WithPrefix returns a new Context with keyvals prepended to those of the
// receiver.
func (c *Context) WithPrefix(keyvals ...any) *Context {
	return &Context{
		prefix: append(c.prefix, keyvals...),
		suffix: c.suffix,
		logger: c.logger,
	}
}

// Log adds prefix and suffix to keyvals and calls internal logger.
func (c *Context) Log(keyvals ...any) error {
	s := append(c.prefix, keyvals...)
	s = append(s, c.suffix...)
	return c.logger.Log(s...)
}
