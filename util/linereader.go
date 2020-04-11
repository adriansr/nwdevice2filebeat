//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import (
	"bufio"
	"io"
)

type LineReader struct {
	inner               *bufio.Reader
	line                uint64
	offset, startOffset uint64
	buf                 []byte
	err                 error
}

func (lc *LineReader) Read(p []byte) (n int, err error) {
	if len(lc.buf) == 0 {
		if lc.err != nil {
			return 0, lc.err
		}
		lc.buf, lc.err = lc.inner.ReadBytes('\n')
		if lc.err != nil && len(lc.buf) == 0 {
			return 0, lc.err
		}
		lc.line ++
		lc.startOffset = lc.offset
		lc.offset += uint64(len(lc.buf))
	}
	fit := len(p)
	if fit > len(lc.buf) {
		fit = len(lc.buf)
	}
	copy(p, lc.buf[:fit])
	lc.buf = lc.buf[fit:]
	return fit, nil
}

func (lc *LineReader) Line() uint64 {
	return lc.line
}

func (lc *LineReader) Offset() uint64 {
	return lc.offset
}

func (lc *LineReader) Position(offset uint64) (line uint64, col uint64) {
	if offset >= lc.startOffset && offset <= lc.offset {
		return lc.line, 1 + offset - lc.startOffset
	}
	return lc.line, 0
}

func NewLineReader(inner io.Reader) *LineReader {
	return &LineReader{
		inner: bufio.NewReader(inner),
	}
}
