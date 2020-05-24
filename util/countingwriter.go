//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import "io"

// CountingWriter wraps an io.Writer adding a Count() method to get the total
// number of bytes written so far.
type CountingWriter struct {
	inner io.Writer
	count uint64
}

// Write writes the bytes to the underlying io.Writer.
func (w *CountingWriter) Write(p []byte) (n int, err error) {
	if n, err = w.inner.Write(p); n > 0 {
		w.count += uint64(n)
	}
	return n, err
}

// Count return the total number of bytes written so far.
func (w *CountingWriter) Count() uint64 {
	return w.count
}

// NewCountingWriter returns a new CountingWriter.
func NewCountingWriter(inner io.Writer) *CountingWriter {
	return &CountingWriter{
		inner: inner,
	}
}
