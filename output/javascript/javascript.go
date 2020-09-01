//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/layout"
	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
)

var header = `//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.
`

type javascript struct {
	tmpFile *os.File
}

func init() {
	instance := new(javascript)
	output.Registry.MustRegister("javascript", instance)
	output.Registry.MustRegister("js", instance)
}

func (js *javascript) Settings() config.PipelineSettings {
	return config.PipelineSettings{
		// Needs complex patterns split into dissect patterns.
		Dissect: true,
		// Needs payload fields stripped.
		StripPayload: true,
	}
}

func (js *javascript) Populate(lyt *layout.Generator) (err error) {
	preamble := `
- script:
    lang: javascript
    params:
      ecs: true
      rsa: {{((getvar "var_prefix"))rsa_fields}}
      tz_offset: {{((getvar "var_prefix"))tz_offset}}
      keep_raw: {{((getvar "var_prefix"))keep_raw_fields}}
      debug: {{((getvar "var_prefix"))debug}}`
	if lyt.HasDir("config.dir") {
		err = lyt.SetVar("extra_processors", preamble+`
    files:
    - ((getvar "basedir"))/((relpath "rel.dir" "config.dir"))/liblogparser.js
    - ((getvar "basedir"))/((relpath "rel.dir" "config.dir"))/pipeline.js
`)
		if err != nil {
			return err
		}
		err = lyt.AddFile("__config.dir__/pipeline.js", layout.Move{
			Path: js.tmpFile.Name(),
		})
		if err != nil {
			return err
		}
		err = lyt.AddFile("__config.dir__/liblogparser.js", layout.Copy{
			Path: "output/javascript/liblogparser.js",
		})
		if err != nil {
			return err
		}
	} else {
		err = lyt.SetVar("extra_processors", fmt.Sprintf(preamble+`
    source: |
((inline "liblogparser.js" | indent " " 6))
((inline "pipeline.js" | indent " " 6))
`))
		if err != nil {
			return err
		}
		err = lyt.AddInlineFile("pipeline.js", layout.Copy{
			Path: js.tmpFile.Name(),
		})
		if err != nil {
			return err
		}
		err = lyt.AddInlineFile("liblogparser.js", layout.Copy{
			Path: "output/javascript/liblogparser.js",
		})
	}
	return nil
}

func (js *javascript) OutputFile() string {
	return js.tmpFile.Name()
}

func (js *javascript) Generate(p parser.Parser) (err error) {
	js.tmpFile, err = ioutil.TempFile("", "pipeline-*.js")
	if err != nil {
		return err
	}
	defer js.tmpFile.Close()
	if err := p.Apply(preprocessors); err != nil {
		return err
	}
	cw := output.NewCodeWriter(js.tmpFile, "\t")
	generate(p.Root, cw)
	return cw.Finalize()
}

func generate(op parser.Operation, out *output.CodeWriter) {
	switch v := op.(type) {
	case File:
		for _, node := range v.Nodes {
			generate(node, out)
		}

	case RawJS:
		out.Raw(v.String())

	case Variable:
		out.Newline()
		out.Write("var ").Write(v.Name).Write(" = ")
		generate(v.Value[0], out)
		out.Write(";").Newline()

	case VariableReference:
		out.Write(v.Name)

	case MainProcessor:
		out.Newline()
		out.Write("function DeviceProcessor() {").Newline().Indent().
			Write("var builder = new processor.Chain();").Newline().
			Write("builder.Add(save_flags);").Newline().
			Write("builder.Add(strip_syslog_priority);").Newline().
			Write("builder.Add(")
		generate(v.inner[0], out)
		out.Write(");").Newline().
			Write("builder.Add(populate_fields);").Newline().
			Write("builder.Add(restore_flags);").Newline().
			Write("var chain = builder.Build();").Newline().
			Write("return {").Newline().
			Indent().Write("process: chain.Run,").Newline().Unindent().
			Write("}").Newline().Unindent().Write("}").Newline()

	case parser.ValueMap:
		out.Newline()
		out.Writef("var map_%s = {", v.Name).Newline().
			Indent().
			Write("keyvaluepairs: ")
		writeMapping(v.Mappings, v.Nodes, out)
		out.Write(",").Newline()
		if v.Default != nil {
			out.JS("default").Write(": ")
			generate(*v.Default, out)
			out.Write(",").Newline()
		}
		out.Unindent().Write("};").Newline()

	case parser.Constant:
		out.Write("constant(").JS(v.Value()).Write(")")

	case parser.Field:
		out.Write("field(").JS(v.Name).Write(")")

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

	case parser.MsgIdSelect:
		out.Write("msgid_select(")
		writeMapping(v.Map, v.Nodes, out)
		out.Write(")")

	case parser.Match:
		if v.TagValues.IsSet() {
			// Print sorted k: v pairs to ensure predictable code.
			keyValues := make([][2]string, 0, len(v.TagValues.Map))
			for k, v := range v.TagValues.Map {
				keyValues = append(keyValues, [2]string{k, v})
			}
			sort.Slice(keyValues, func(i, j int) bool {
				return keyValues[i][0] < keyValues[j][0]
			})
			out.Write("tagval(").JS(v.ID).Write(", ").JS(v.Input).Write(", tvm, {").Newline()
			for _, entry := range keyValues {
				out.Indent().JS(entry[0]).Write(": ").JS(entry[1]).Write(",").Unindent().Newline()
			}
			out.Write("}")
		} else {
			fn := "match"
			arg := v.Pattern.Tokenizer()
			// If this is a single capture dissect pattern, i.e. "%{fld}" or "%{}",
			// replace with a call to match_copy, which will be faster and supports
			// empty input. (Dissect always fails for empty input).
			switch len(v.Pattern) {
			case 0:
				fn = "match_copy"
				arg = ""
			case 1:
				if fld, ok := v.Pattern[0].(parser.Field); ok {
					fn = "match_copy"
					arg = fld.Name
				}
			}
			out.Writef("%s(", fn).JS(v.ID).Write(", ").JS(v.Input).Write(", ").JS(arg)
		}
		if len(v.OnSuccess) > 0 {
			out.Write(", processor_chain([").
				Indent().Newline()
			for _, act := range v.OnSuccess {
				generate(act, out)
				out.Write(",").Newline()
			}
			out.Unindent().Write("])")
		}
		out.Write(")")
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

	case SetField:
		out.Write("setf(").JS(v[0]).Write(",").JS(v[1]).Write(")")

	case SetConstant:
		out.Write("setc(").JS(v[0]).Write(",").JS(v[1]).Write(")")

	case parser.ValueMapCall:
		out.Write("lookup({").Newline().Indent().
			Write("dest: ").JS(v.Target).Write(",").Newline().
			Write("map: ").Write("map_" + v.MapName).Write(",").Newline().
			Write("key: ")
		generate(v.Key[0], out)
		out.Write(",").Newline().Unindent()
		out.Write("})")

	case parser.DateTime:
		writeDateTimeLike(v, "date_time", "d", out)

	case parser.Duration:
		writeDateTimeLike(parser.DateTime(v), "duration", "u", out)

	case parser.RemoveFields:
		out.Write("remove(").JS(v).Write(")")

	case SetProcessor:
		out.Write("set(")
		writeMapString(v, out)
		out.Write(")")

	case parser.URLExtract:
		if fn, found := urlComponentToJSFn[v.Component]; found {
			out.Write(fn).Write("(").JS(v.Target).Write(",").
				JS(v.Source).Write(")")
		} else {
			out.Err(errors.Errorf("unknown URL component to extract: %v", v.Component))
		}
	case parser.Noop:
		// Removing nodes from the tree is complicated.
		out.Write("nop")
		out.Err(errors.New("WARN: Found a Noop in the tree."))

	case MsgID1Wrapper:
		out.Write("msg(").JS(v.msgID1).Write(", ")
		generate(v.wrapped[0], out)
		out.Write(")")

	case TagValMapCfg:
		if v.TagValMapSettings == nil {
			return
		}
		out.Write("var tvm = {").Newline().Indent().
			Write("pair_separator: ").JS(v.PairSeparator).Write(",").Newline().
			Write("kv_separator: ").JS(v.KeyValueSeparator).Write(",").Newline().
			Write("open_quote: ").JS(v.OpenQuote).Write(",").Newline().
			Write("close_quote: ").JS(v.CloseQuote).Write(",").Newline().
			Unindent().Write("};").Newline()

	default:
		out.Writef("/* TODO: here goes a %T */", v)
		out.Err(errors.Errorf("unknown type to serialize %T", v))
	}
}

func writeMapping(m map[string]int, nodes []parser.Operation, out *output.CodeWriter) {
	out.Write("{").Newline().Indent()
	keys := make([]string, len(m))
	pos := 0
	for key := range m {
		keys[pos] = key
		pos++
	}
	sort.Strings(keys)
	for _, key := range keys {
		idx := m[key]
		value := nodes[idx]
		out.JS(key).Write(": ")
		generate(value, out)
		out.Write(",").Newline()
	}
	out.Unindent().Write("}")
}

func writeMapString(m map[string]string, out *output.CodeWriter) {
	out.Write("{").Newline().Indent()
	keys := make([]string, len(m))
	pos := 0
	for key := range m {
		keys[pos] = key
		pos++
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.JS(key).Write(": ").JS(m[key]).Write(",").Newline()
	}
	out.Unindent().Write("}")
}

func writeDateTimeLike(dt parser.DateTime, name, fnPrefix string, out *output.CodeWriter) {
	out.Write(name).Write("({").Newline().Indent().
		Write("dest: ").JS(dt.Target).Write(",").Newline().
		Write("args: ").JS(dt.Fields).Write(",").Newline()
	if dt.IsUTC {
		out.Write("tz: 'Z',").Newline()
	}
	out.Write("fmts: [").Newline().Indent()
	for fmtIdx := range dt.Formats {
		out.Write("[")
		for idx, fmt := range dt.Formats[fmtIdx] {
			if idx > 0 {
				out.Write(",")
			}
			if spec := fmt.Spec(); spec != parser.DateTimeConstant {
				out.Writef("%s%c", fnPrefix, spec)
			} else {
				out.Writef("%sc(", fnPrefix).JS(fmt.Value()).Write(")")
			}
		}
		out.Write("],").Newline()
	}
	out.Unindent().Write("],").Newline().Unindent().Write("})")
}

var urlComponentToJSFn = map[parser.URLComponent]string{
	parser.URLComponentDomain: "domain",
	parser.URLComponentExt:    "ext",
	parser.URLComponentFqdn:   "fqdn",
	parser.URLComponentPage:   "page",
	parser.URLComponentPath:   "path",
	parser.URLComponentPort:   "port",
	parser.URLComponentQuery:  "query",
	parser.URLComponentRoot:   "root",
}
