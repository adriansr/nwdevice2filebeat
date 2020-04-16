//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

var header = `//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

var processor = require("processor");
var console   = require("console");

var device;

// Register params from configuration.
function register(params) {
    device = new DeviceProcessor();
}

function process(evt) {
    return device.process(evt);
}

`

type codeWriter struct {
	//buf bytes.Buffer
	dest io.Writer
	errors []error
	prefix  []byte
	indent []byte
	bytes  uint64
	writeFailed bool
	newline bool
}

func newCodeWriter(target io.Writer, indent string) *codeWriter {
	return &codeWriter{
		dest:    target,
		indent:  []byte(indent),
	}
}

func (c *codeWriter) write(data []byte) *codeWriter {
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

func (c *codeWriter) AddRaw(raw string) *codeWriter {
	c.newline = false
	return c.write([]byte(raw))
}

func (c *codeWriter) Err(err error) *codeWriter {
	if err != nil {
		c.errors = append(c.errors, err)
	}
	return c
}

func (c *codeWriter) Newline() *codeWriter {
	c.newline = true
	return c.write([]byte{'\n'})
}

func (c *codeWriter) JS(v interface{}) *codeWriter {
	b, err := json.Marshal(v)
	c.Err(err)
	return c.Write(string(b))
}

func (c *codeWriter) Write(s string) *codeWriter {
	return c.WriteBytes([]byte(s))
}

func (c *codeWriter) WriteBytes(s []byte) *codeWriter {
	if c.newline {
		c.newline = false
		c.write(c.prefix)
	}
	return c.write(s)
}

func (c *codeWriter) Writef(format string, args... interface{}) *codeWriter {
	return c.Write(fmt.Sprintf(format, args...))
}

func (c *codeWriter) Indent() *codeWriter {
	c.prefix = append(c.prefix, c.indent...)
	return c
}

func (c *codeWriter) Unindent() *codeWriter {
	if a, b := len(c.prefix), len(c.indent); a >= b {
		c.prefix = c.prefix[:a-b]
	} else {
		c.Err(errors.New("indent below zero"))
	}
	return c
}

func (c *codeWriter) Finalize() (count uint64, err error) {
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

func Generate(p parser.Parser, dest io.Writer) (bytes uint64, err error) {
	cw := newCodeWriter(dest, "\t")
	cw.AddRaw(header)
	for _, vm := range p.ValueMaps {
		generate(vm, cw)
		cw.Newline()
	}
	cw.Write("function DeviceProcessor() {").Newline().Indent().
		Write("var builder = new processor.Chain();").Newline().
		Write("builder.Add(save_flags);").Newline().
		Write("builder.Add(").Newline().Indent()
	generate(p.Root, cw)
	cw.Unindent().Write(");").Newline().
		Write("builder.Add(restore_flags);").Newline().
		Write("var chain = builder.Build();").Newline().
		Write("return {").Newline().
			Indent().Write("process: chain.Run,").Newline().Unindent().
		Write("}").Newline().Unindent().Write("}").Newline()
	return cw.Finalize()
}

func generate(op parser.Operation, out *codeWriter) {
	switch v := op.(type) {
	case parser.ValueMap:
		out.Writef("var map_%s = {", v.Name).Newline().
			Indent().
				JS("keyvaluepairs").Write(": {").Newline().
				Indent()

		for key, idx := range v.Mappings {
			value := v.Nodes[idx]
			out.JS(key).Write(": ")
			generate(value, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("},").Newline()
		if v.Default != nil {
			out.JS("default").Write(": ")
			generate(*v.Default, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("};").Newline()

	case parser.Constant:
		out.Write("constant(").JS(v).Write(")")

	case parser.Field:
		out.Write("field(").JS(v).Write(")")

	case parser.Chain:
		out.Write("processor_chain([").Newline().Indent()
		for _, node := range v.Nodes {
			generate(node, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("])")

	case parser.LinearSelect:
		out.Write("linear_select([").Newline().Indent()
		for _, node := range v.Nodes {
			generate(node, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("])")

	case parser.Match:
		out.Write("match({").Newline().
			Indent().Write("dissect: {").Newline().
				Indent().Write("tokenizer: ").JS(v.Pattern.Tokenizer()).Write(",").Newline().
				// TODO: Why Input is Field and not string
						 Write("field: ").JS(string(v.Input)).Write(",").Newline().
						 Write("target_prefix: ").JS("nwparser").Write(",").Newline().
						 Write("ignore_failure: ").JS(true).Write(",").Newline().
			    Unindent().Write("},").Newline()
		if len(v.OnSuccess) > 0 {
			out.Write("on_success: processor_chain([").
				Indent().Newline()
			for _, act := range v.OnSuccess {
				generate(act, out)
				out.Write(",").Newline()
			}
			out.Unindent().Write("]),").Newline()
		}
		out.Unindent().Write("})")

	case parser.Call:
		out.Write("call({").Newline().Indent().
			Write("dest: ").JS(v.Target).Write(",").Newline().
			Write("fn: ").Write(v.Function).Write(",").Newline().
			Write("args: [").Newline().Indent()
		for _, arg := range v.Args {
			generate(arg, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("],").Unindent().Newline().Write("})")

	case *parser.Call:
		// TODO: Get rid of pointers sneaking in
		generate(*v, out)

	case parser.SetField:
		out.Write("set_field({").Newline().Indent().
			Write("dest: ").JS(v.Target).Write(",").Newline().
			Write("value: ")
		generate(v.Value[0], out)
		out.Write(",").Newline().Unindent()
		out.Write("})")

	case parser.ValueMapCall:
		out.Write("lookup({").Newline().Indent().
			Write("dest: ").JS(v.Target).Write(",").Newline().
			Write("map: ").Write("map_" + v.MapName).Write(",").Newline().
			Write("key: ")
		generate(v.Key[0], out)
		out.Write(",").Newline().Unindent()
		out.Write("})")

	default:
		out.Writef("/* TODO: here goes a %T */", v)
		// TODO: return nil, errors.Errorf("unsupported type %T", v)
		out.Err(errors.Errorf("unknown type to serialize %T", v))
	}
}
