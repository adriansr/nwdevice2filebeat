//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

type CodeWriter struct {
	//buf bytes.Buffer
	dest io.Writer
	errors []error
	prefix  []byte
	indent []byte
	bytes  uint64
	writeFailed bool
	newline bool
}

func NewCodeWriter(target io.Writer, indent string) *CodeWriter {
	return &CodeWriter{
		dest:    target,
		indent:  []byte(indent),
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

func (c *CodeWriter) AddRaw(raw string) *CodeWriter {
	c.newline = false
	return c.write([]byte(raw))
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

func (c *CodeWriter) JS(v interface{}) *CodeWriter {
	b, err := json.Marshal(v)
	c.Err(err)
	return c.Write(string(b))
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

func (c *CodeWriter) Writef(format string, args... interface{}) *CodeWriter {
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
			fmt.Sprintf("found %d errors while generating javascript:\n", n),
		}
		for i := 0; i < limit; i++ {
			msg = append(msg, "    " + c.errors[i].Error())
		}
		if limit != n {
			msg = append(msg, fmt.Sprintf("    ... (and %d more)", n - limit))
		}
		err = errors.New(strings.Join(msg, "\n"))
	}
	return c.bytes, err
}
