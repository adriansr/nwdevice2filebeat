//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"
)

type JSMarshaler interface {
	MarshalJS(indent int) (out []byte, err error)
}

type Comment []string

func (c Comment) MarshalJS(indent int) (out []byte, err error) {
	var buf bytes.Buffer
	prefix := makeIndent(indent + 3)
	prefix[indent] = '/'
	prefix[indent+1] = '/'
	for _, line := range c {
		buf.Write(prefix)
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

func makeIndent(size int) []byte {
	out := make([]byte, size)
	for i := range out {
		out[i] = ' '
	}
	return out
}

type Require string

func (c Require) MarshalJS(indent int) (out []byte, err error) {
	if indent != 0 {
		return nil, errors.New("require with indent different than zero")
	}
	line := fmt.Sprintf(`var %s = require("%s");`, c, c)
	return []byte(line), nil
}

type Raw []byte

func (c Raw) MarshalJS(indent int) (out []byte, err error) {
	return c, nil
}

var Newline = Raw("\n")

type Function struct {
	Name string
	Args []string
	Content Components
}

func (f Function) MarshalJS(indent int) (out []byte, err error) {
	prefix := makeIndent(indent)
	var buf bytes.Buffer
	buf.Write(prefix)
	buf.WriteString(fmt.Sprintf("function %s(%s) {\n", f.Name, strings.Join(f.Args, ", ") ))
	for _, elem := range f.Content {
		b, err := elem.MarshalJS(indent + 4)
		buf.Write(b)
		if err != nil {
			return buf.Bytes(), err
		}
	}
	buf.Write(prefix)
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

type Components []JSMarshaler

func (c Components) MarshalJS(indent int) (out []byte, err error) {
	var buf bytes.Buffer
	var b []byte
	for _, elem := range c {
		b, err = elem.MarshalJS(indent)
		buf.Write(b)
		if err != nil {
			break
		}
	}
	return buf.Bytes(), err
}

func (c *Components) Add(content... JSMarshaler) *Components {
	*c = append(*c, content...)
	return c
}

type Var struct {
	Name string
	Value interface{}
}

func (v Var) MarshalJS(indent int) (out []byte, err error) {
	var buf bytes.Buffer
	prefix := makeIndent(indent)
	buf.Write(prefix)
	buf.WriteString(fmt.Sprintf("var %s = ", v.Name))
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent(string(prefix), "    ")
	err = encoder.Encode(v.Value)
	return buf.Bytes(), err
}

type FunctionCall struct {
	Fn string
	Args []interface{}
}

func (f FunctionCall) MarshalJSON() (out []byte, err error) {
	var buf bytes.Buffer
	buf.WriteString(f.Fn)
	buf.WriteString("(\n    ")
	for idx, arg := range f.Args {
		jsArg, err := json.MarshalIndent(arg, "    ", "    ")
		if err != nil {
			return nil, errors.Wrapf(err, "failed serializing argument %d of function call %s", idx, f.Fn)
		}
		buf.Write(jsArg)
	}
	buf.WriteString(")\n")
	return buf.Bytes(), nil
}

func (f FunctionCall) MarshalJS(indent int) (out []byte, err error) {
	log.Printf("XXX FunctionCall MarshalJS called!\n")
	return nil, nil
}

type indentChange int

func (indentChange) MarshalJS(int) (out []byte, err error) {
	return nil, nil
}

var (
	Indent indentChange = 1
	Unindent indentChange = -1
)
