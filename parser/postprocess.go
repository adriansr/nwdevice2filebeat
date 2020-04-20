//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"
)

type Action struct {
	Name string
	Run  func(parser *Parser) error
}

type PostprocessGroup struct {
	Title   string
	Actions []Action
}

// These actions check that the parser extracted from the XML is well-formed.
// They run before the parser tree is built.
var prechecks = PostprocessGroup{
	Title: "pre-checks",
	Actions: []Action{
		// It's an error for a message to contain a payload field.
		{"check for payload in messages", checkPayloadInMessages},

		// Consider an error for a HEADER to contain a payload in a position other
		// than last.
		{"check payload position in headers", checkPayloadPositionInHeaders},

		// This checks that if a header uses a payload field (the payload
		// overlaps part of the header), then this field must appear once.
		{"check overlapped payload fields", checkPayloadOverlap},
	},
}

var preactions = PostprocessGroup{
	Title: "pre-actions",
	Actions: []Action{
		{"adjust payload field", setPayloadField},
	},
}

var transforms = PostprocessGroup{
	Title: "transforms",
	Actions: []Action{
		// Replaces a Call() to a MalueMap with a ValueMapCall.
		{"translate valuemap references", convertValueMapReferences},
		{"translate PARMVAL calls", translateParmval},

		// Remove unnecessary $MSG and $HDR as first argument to calls
		{"remove special fields from some calls", removeSpecialFields},
		{"log special field usage", checkSpecialFields},

		// Convert EVNTTIME calls
		{"convert EVNTTIME calls", convertEventTime},
		// TODO:
		// Replace SYSVAL references with fields from headers (id1, messageid, etc.)

		{"split alternatives into dissect patterns", splitDissect},
	},
}

var optimizations = PostprocessGroup{
	Title: "optimizations",
	Actions: []Action{
		{"evaluate constant functions", evalConstantFunctions},
	},
}

func checkPayloadInMessages(parser *Parser) error {
	for _, msg := range parser.Messages {
		if _, err := msg.content.PayloadField(); err == nil {
			return errors.Errorf("at %s: MESSAGE with payload field", msg.pos)
		}
	}
	return nil
}

func checkPayloadPositionInHeaders(parser *Parser) error {
	for _, hdr := range parser.Headers {
		// Skip headers without a payload.
		if _, err := hdr.content.PayloadField(); err != nil {
			continue
		}
		last := hdr.content[len(hdr.content)-1]
		if _, ok := last.(Payload); !ok {
			return errors.Errorf("at %s: HEADER payload is not the last field", hdr.pos)
		}
	}
	return nil
}

func setPayloadField(parser *Parser) error {
	for idx, hdr := range parser.Headers {
		payload, err := hdr.content.PayloadField()
		if err != nil {
			return errors.Wrapf(err, "at %s", hdr.pos)
		}
		if payload != "" {
			parser.Headers[idx].payloadField = payload
		}
		n := len(hdr.content)
		last := hdr.content[n-1]
		if _, ok := last.(Payload); !ok {
			return errors.New("expected payload as last field")
		}
		hdr.content[n-1] = Field("payload")
	}
	return nil
}

func checkPayloadOverlap(parser *Parser) (err error) {
	var payload string
	for _, hdr := range parser.Headers {
		if payload, err = hdr.content.PayloadField(); err != nil {
			return errors.Wrapf(err, "at %s", hdr.pos)
		}
		if payload == "" || payload == "$START" {
			continue
		}
		count := 0
		for _, elem := range hdr.content {
			if fld, ok := elem.(Field); ok && fld.Name() == payload {
				count++
			}
		}
		if count != 1 {
			return errors.Errorf("at %s: payload field '%s' appears %d times. Expected 1.", hdr.pos, payload, count)
		}
	}
	return nil
}

func removeSpecialFields(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		switch call := node.(type) {
		case Call:
			if len(call.Args) > 0 {
				if fld, ok := call.Args[0].(Field); ok && (fld.Name() == "$MSG" || fld.Name() == "$HDR") {
					call.Args = call.Args[1:]
					return WalkReplace, call
				}
			}
		}
		return WalkContinue, nil
	})
	return err
}
func checkSpecialFields(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		switch call := node.(type) {
		case Call:
			for pos, arg := range call.Args {
				if fld, ok := arg.(Field); ok && len(fld.Name()) > 0 && fld.Name()[0] == '$' {
					log.Printf("INFO at %s: special field %s at position %d in call %s\n", call.Source(), fld.Name(), pos, call.String())
				}
			}
		}
		return WalkContinue, nil
	})
	return err
}

func convertValueMapReferences(parser *Parser) error {
	var errs multierror.Errors
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		switch call := node.(type) {
		case Call:
			if _, found := parser.ValueMapsByName[call.Function]; found {
				if len(call.Args) != 1 {
					errs = append(errs, errors.Errorf("at %s: call to VALUEMAP must have exactly 1 argument but has %d",
						call.Source(), len(call.Args)))
				}
				return WalkReplace, ValueMapCall{
					SourceContext: call.SourceContext,
					MapName:       call.Function,
					Target:        call.Target,
					Key:           []Operation{call.Args[0]},
				}
			}
		}
		return WalkContinue, nil
	})
	return errs.Err()
}

func translateParmval(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok && call.Function == "PARMVAL" {
			if n := len(call.Args); n != 1 {
				err = errors.Errorf("at %s: expected 1 argument in PARMVAL call, got %d", call.Source(), n)
				return WalkCancel, nil
			}
			repl := SetField{
				SourceContext: call.SourceContext,
				Target:        call.Target,
				Value:         []Operation{call.Args[0]},
			}
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func convertEventTime(p *Parser) (err error) {
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok && call.Function == "EVNTTIME" {
			numArgs := len(call.Args)
			if numArgs < 2 {
				err = errors.Errorf("at %s: EVNTTIME call has too little arguments: %d", call.Source(), numArgs)
				return WalkCancel, nil
			}
			var numFormats int
			for numFormats = range call.Args {
				if _, ok := call.Args[numFormats].(Constant); !ok {
					break
				}
			}
			if numFormats == 0 {
				err = errors.Errorf("at %s: EVNTTIME call format is not a constant: %s", call.Source(), call.Args[0])
				return WalkCancel, nil
			} else if numFormats == numArgs {
				err = errors.Errorf("at %s: EVNTTIME has no fields", call.Source())
				return WalkCancel, nil
			}

			fields := make([]string, numArgs-numFormats)
			for idx, arg := range call.Args[numFormats:] {
				if field, ok := arg.(Field); ok {
					fields[idx] = field.Name()
				} else {
					err = errors.Errorf("at %s: EVNTTIME call argument %d is not a field", call.Source(), idx+numFormats)
					return WalkCancel, nil
				}
			}
			repl := DateTime{
				SourceContext: call.SourceContext,
				Target:        call.Target,
				Fields:        fields,
			}
			repl.Formats = make([][]DateTimeItem, numFormats)
			for idx, value := range call.Args[:numFormats] {
				ct, _ := value.(Constant)
				fmt, err := parseDateTimeFormat(ct.Value())
				if err != nil {
					err = errors.Wrapf(err, "at %s: failed to parse EVNTTIME format '%s'", call.Source(), ct.Value())
					return WalkCancel, nil
				}
				repl.Formats[idx] = fmt
			}
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func dupPattern(in []Value) (out []Value) {
	out = make([]Value, len(in))
	copy(out, in)
	return
}

func splitDissect(p *Parser) (err error) {
	// TODO:
	// Control with a flag.
	p.WalkPostOrder(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok && match.Pattern.HasAlternatives() {
			var repl AllMatch
			input, partCounter := match.Input, 0
			for pos := 0; ; pos++ {
				end := pos
				var found bool
				var alt Alternatives
				for ; end < len(match.Pattern); end++ {
					if alt, found = match.Pattern[end].(Alternatives); found {
						break
					}
				}
				if pos < end {
					node := Match{
						SourceContext: match.SourceContext,
						Input:         input,
						Pattern:       dupPattern(match.Pattern[pos:end]),
						PayloadField:  "", // TODO
					}
					if found {
						input = fmt.Sprintf("p%d", partCounter)
						partCounter++
						node.Pattern = append(node.Pattern, Field(input))
					}
					repl.Nodes = append(repl.Nodes, node)
				}
				if !found {
					break
				}
				pos = end
				sel := LinearSelect{SourceContext: match.SourceContext}
				curInput := input
				var part Value
				if pos < len(match.Pattern) {
					input = fmt.Sprintf("p%d", partCounter)
					part = Field(input)
					partCounter++
				}
				for _, pattern := range alt {
					m := Match{
						SourceContext: match.SourceContext,
						Input:         curInput,
						PayloadField:  "", // TODO
					}
					m.Pattern = pattern
					if part != nil {
						m.Pattern = append(m.Pattern, part)
					}
					sel.Nodes = append(sel.Nodes, m)
				}
				repl.Nodes = append(repl.Nodes, sel)
			}
			repl.onSuccessPos = len(repl.Nodes)
			repl.Nodes = append(repl.Nodes, match.OnSuccess...)
			repl.onFailurePos = len(repl.Nodes)
			// TODO: cleanup on failure
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func evalFn(name string, params []string) (string, error) {
	switch name {
	case "STRCAT":
		return strings.Join(params, ""), nil
	}
	return "", errors.Errorf("unsupported function %s", name)
}

func evalConstantFunctions(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok {
			var params = make([]string, len(call.Args))
			for idx, arg := range call.Args {
				if ct, ok := arg.(Constant); ok {
					params[idx] = string(ct)
				} else {
					return WalkContinue, nil
				}
			}
			var constant string
			constant, err = evalFn(call.Function, params)
			if err != nil {
				err = errors.Wrapf(err, "at %s", call.Source())
				return WalkCancel, nil
			}
			repl := SetField{
				SourceContext: call.SourceContext,
				Target:        call.Target,
				Value:         []Operation{Constant(constant)},
			}
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func (parser *Parser) Apply(group PostprocessGroup) error {
	log.Printf("Running %d %s\n", len(group.Actions), group.Title)
	for _, act := range group.Actions {
		if err := act.Run(parser); err != nil {
			return errors.Wrapf(err, "error applying %s/%s", group.Title, act.Name)
		}
	}
	return nil
}

func validate(parser *Parser) (err error) {
	const OpLimit = 50000000
	count := 0
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		if count++; count == OpLimit {
			err = errors.Errorf("bug or device definition too large: tree traversal exceeded limit of %d nodes", OpLimit)
			return WalkCancel, nil
		}
		switch v := node.(type) {
		case Call:
			if v.Target == "" && v.Function != "SYSVAL" {
				err = errors.Errorf("at %s: call to %s function doesn't have a target", v.Source(), v.Function)
				return WalkCancel, nil
			}
		}
		return WalkContinue, nil
	})
	log.Printf("Validated tree of %d nodes\n", count)
	return
}
