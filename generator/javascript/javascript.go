//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"io"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/generator"
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

func Generate(p parser.Parser, dest io.Writer) (bytes uint64, err error) {
	if err := p.Apply(preprocessors); err != nil {
		return 0, err
	}
	cw := generator.NewCodeWriter(dest, "\t")
	generate(p.Root, cw)
	return cw.Finalize()
}

func generate(op parser.Operation, out *generator.CodeWriter) {
	switch v := op.(type) {
	case File:
		for _, node := range v.Nodes {
			generate(node, out)
		}

	case RawJS:
		out.AddRaw(v.String())

	case Variable:
		out.Newline()
		out.Write("var ").Write(v.Name).Write(" = ")
		generate(v.Value, out)
		out.Write(";").Newline()

	case VariableReference:
		out.Write(v.Name)

	case MainProcessor:
		out.Newline()
		out.Write("function DeviceProcessor() {").Newline().Indent().
			Write("var builder = new processor.Chain();").Newline().
			Write("builder.Add(save_flags);").Newline().
			Write("builder.Add(")
		generate(v.inner[0], out)
		out.Write(");").Newline().
			Write("builder.Add(restore_flags);").Newline().
			Write("var chain = builder.Build();").Newline().
			Write("return {").Newline().
			Indent().Write("process: chain.Run,").Newline().Unindent().
			Write("}").Newline().Unindent().Write("}").Newline()

	case parser.ValueMap:
		out.Newline()
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
			Write("field: ").JS(v.Input).Write(",").Newline().
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

	case parser.AllMatch:
		out.Write("all_match({").Newline().Indent().
			Write("processors: [").Newline().Indent()
		for _, proc := range v.Processors() {
			generate(proc, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("],").Newline()
		if len(v.OnSuccess()) > 0 {
			out.Write("on_success: processor_chain([").
				Indent().Newline()
			for _, act := range v.OnSuccess() {
				generate(act, out)
				out.Write(",").Newline()
			}
			out.Unindent().Write("]),").Newline()
		}
		if len(v.OnFailure()) > 0 {
			out.Write("on_failure: processor_chain([").
				Indent().Newline()
			for _, act := range v.OnFailure() {
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

	case parser.DateTime:
		out.Write("date_time({").Newline().Indent().
			Write("dest: ").JS(v.Target).Write(",").Newline().
			Write("args: ").JS(v.Fields).Write(",").Newline().
			Write("fmt: [")
		for idx, fmt := range v.Format {
			if idx > 0 {
				out.Write(",")
			}
			if spec := fmt.Spec(); spec != parser.DateTimeConstant {
				out.Writef("d%c", spec)
			} else {
				out.Write("dc(").JS(fmt.Value()).Write(")")
			}
		}
		out.Write("],").Newline().Unindent().Write("})")

	default:
		out.Writef("/* TODO: here goes a %T */", v)
		out.Err(errors.Errorf("unknown type to serialize %T", v))
	}
}
