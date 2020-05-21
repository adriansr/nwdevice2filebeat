//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

type CodeWriter struct {
	//buf bytes.Buffer
	dest        io.Writer
	errors      []error
	prefix      []byte
	indent      []byte
	bytes       uint64
	writeFailed bool
	newline     bool
}

func NewCodeWriter(target io.Writer, indent string) *CodeWriter {
	return &CodeWriter{
		dest:   target,
		indent: []byte(indent),
	}
}

func (c *CodeWriter) write(data []byte) *CodeWriter {
	total := len(data)
	if total == 0 || c.writeFailed {
		return c
	}
	written, err := c.dest.Write(data)
	if err != nil || written != total {
		if err == nil {
			err = errors.New("short write")
		}
		c.Err(errors.Wrap(err, "error writing output"))
	}
	c.bytes += uint64(total)
	return c
}

func (c *CodeWriter) Raw(raw string) *CodeWriter {
	return c.RawBytes([]byte(raw))
}

func (c *CodeWriter) RawBytes(raw []byte) *CodeWriter {
	c.newline = false
	return c.write(raw)
}

func (c *CodeWriter) Err(err error) *CodeWriter {
	if err != nil {
		c.errors = append(c.errors, err)
	}
	return c
}

func (c *CodeWriter) Newline() *CodeWriter {
	c.newline = true
	return c.write([]byte{'\n'})
}

var (
	escapedClosingAngleBracket = []byte("\\u003e")
	angleBracket               = []byte(">")
)

func (c *CodeWriter) JS(v interface{}) *CodeWriter {

	b, err := json.Marshal(v)
	c.Err(err)
	// The pain with json.Marshal is that it wants to escape > which is
	// wildly used in our patterns.
	// The alternative is to use json.NewEncoder, which in turns adds a
	// newline after the entity.
	// So let's just strip those annoying characters:
	return c.WriteBytes(bytes.ReplaceAll(b, escapedClosingAngleBracket, angleBracket))
}

func (c *CodeWriter) Write(s string) *CodeWriter {
	return c.WriteBytes([]byte(s))
}

func (c *CodeWriter) WriteBytes(s []byte) *CodeWriter {
	if c.newline {
		c.newline = false
		c.write(c.prefix)
	}
	return c.write(s)
}

func (c *CodeWriter) Writef(format string, args ...interface{}) *CodeWriter {
	return c.Write(fmt.Sprintf(format, args...))
}

func (c *CodeWriter) Indent() *CodeWriter {
	c.prefix = append(c.prefix, c.indent...)
	return c
}

func (c *CodeWriter) Unindent() *CodeWriter {
	if a, b := len(c.prefix), len(c.indent); a >= b {
		c.prefix = c.prefix[:a-b]
	} else {
		c.Err(errors.New("indent below zero"))
	}
	return c
}

func (c *CodeWriter) Finalize() (count uint64, err error) {
	if n := len(c.errors); n > 0 {
		limit := n
		if limit > 10 {
			limit = 10
		}
		msg := []string{
			fmt.Sprintf("found %d errors while generating code:\n", n),
		}
		for i := 0; i < limit; i++ {
			msg = append(msg, "    "+c.errors[i].Error())
		}
		if limit != n {
			msg = append(msg, fmt.Sprintf("    ... (and %d more)", n-limit))
		}
		err = errors.New(strings.Join(msg, "\n"))
	}
	return c.bytes, err
}
