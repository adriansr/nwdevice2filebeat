//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/util"
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

		{"check function arity", checkFunctionsArity},
	},
}

var preactions = PostprocessGroup{
	Title: "pre-actions",
	Actions: []Action{
		{"strip leading space in messages", stripLeadingSpace},
		{"adjust payload field", setPayloadField},
	},
}

var transforms = PostprocessGroup{
	Title: "transforms",
	Actions: []Action{
		// Replaces a Call() to a MalueMap with a ValueMapCall.
		{"translate VALUEMAP references", convertValueMapReferences},
		{"translate calls", translateCalls},
		{"strip REGX calls", stripRegex},
		{"strip SYSVAL calls", pruneSysval},
		{"strip same-field copy", pruneUnnecessaryCopies},
		{"remove no-ops in function lists", removeNoops},

		// Remove unnecessary $MSG and $HDR as first argument to calls
		{"remove special fields from some calls", removeSpecialFields},

		// Convert EVNTTIME calls
		{"convert EVNTTIME and DUR calls", convertEventTime},

		// TODO:
		// Replace SYSVAL references with fields from headers (id1, messageid, etc.)

		{"fix consecutive dissect captures", fixDissectCaptures},

		{"fix repetition at edge of alternatives", fixAlternativesEdgeSpace},

		{"fix consecutive dissect captures in alternatives", fixAlternativesEndingInCapture},

		{"fix extra leading space in constants (1)", fixExtraLeadingSpaceInConstants},

		// Reaching this point, if we have:
		// constant<field>{alternative...}
		// we will generate broken dissect due to the trailer {pN} used to
		// terminate the pattern before the alternative. Inject the <field>
		// into the alternatives to avoid this.
		{"inject leading captures into alternatives", injectCapturesInAlts},

		{"split alternatives into dissect patterns", splitDissect},

		//{"fix extra leading space in constants (2)", fixExtraLeadingSpaceInConstants},
		{"fix extra space in constants", fixExtraSpaceInConstants},

		{"make dissect captures greedy", makeDissectGreedy},
	},
}

var optimizations = PostprocessGroup{
	Title: "optimizations",
	Actions: []Action{
		{"evaluate constant functions", evalConstantFunctions},
	},
}

var validations = PostprocessGroup{
	Title: "validations",
	Actions: []Action{
		{"generic validations", validate},
		{"detect bad dissect patterns", detectBrokenDissectPatterns},
		{"detect unknown function calls", detectUnknownFunctionCalls},
		{"detect unnamed matches", detectUnnamedMatches},
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
	if !parser.Config.StripPayload {
		return nil
	}
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
		hdr.content[n-1] = Field{Name: "payload"}
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
			if fld, ok := elem.(Field); ok && fld.Name == payload {
				count++
			}
		}
		if count != 1 {
			return errors.Errorf("at %s: payload field '%s' appears %d times. Expected 1.", hdr.pos, payload, count)
		}
	}
	return nil
}

func checkFunctionsArity(parser *Parser) (err error) {
	var errs multierror.Errors

	checkFnArity := func(parser *Parser, op Operation) error {
		displayRange := func(min, max int) string {
			if max == 0 {
				return fmt.Sprintf("%d or more", min)
			}
			if min == max {
				return fmt.Sprintf("%d", min)
			}
			return fmt.Sprintf("between %d and %d", min, max)
		}
		if call, ok := op.(Call); ok {
			n := len(call.Args)
			fn := call.Function
			min, max := 0, 0
			if info, ok := KnownFunctions[fn]; ok {
				min, max = info.MinArgs, info.MaxArgs
			} else if _, ok := parser.RegexsByName[fn]; ok {
				min, max = 1, 1
			} else if _, ok := parser.ValueMapsByName[fn]; ok {
				min, max = 1, 1
			} else {
				return errors.Errorf("at %s: can't check arguments on unknown function: %s (%d)", call.Source(), fn, n)
			}
			warn := ""
			if n < min {
				warn = "few"
			} else if max > 0 && n > max {
				warn = "many"
			}
			if warn != "" {
				return errors.Errorf("at %s: too %s arguments for '%s' call. Got: %d wanted: %s", call.Source(), warn, fn, n, displayRange(min, max))
			}
		}
		return nil
	}

	checkFnListArity := func(parser *Parser, ops []Operation) {
		for _, op := range ops {
			if err := checkFnArity(parser, op); err != nil {
				errs = append(errs, err)
			}
		}
	}

	for _, hdr := range parser.Headers {
		checkFnListArity(parser, hdr.functions)
	}
	for _, msg := range parser.Messages {
		checkFnListArity(parser, msg.functions)
	}
	return errs.Err()
}

func pruneSysval(parser *Parser) (err error) {
	parser.WalkPostOrder(func(node Operation) (WalkAction, Operation) {
		if match, ok := node.(Match); ok {
			for idx, op := range match.OnSuccess {
				if call, ok := op.(Call); ok && call.Function == "SYSVAL" {
					match.OnSuccess[idx] = Noop{}
				}
			}
		}
		return WalkContinue, nil
	})
	return err
}

// Prune actions like SetField{Target: myfield, Value: Field(myfield)}
// These appear a lot as myfield=*HDR(myfield)
func pruneUnnecessaryCopies(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		if match, ok := node.(Match); ok {
			for idx, op := range match.OnSuccess {
				if setf, ok := op.(SetField); ok {
					if argf, ok := setf.Value[0].(Field); ok && setf.Target == argf.Name {
						match.OnSuccess[idx] = Noop{}
					}
				}
			}
		}
		return WalkContinue, nil
	})
	return err
}

func removeSpecialFields(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (WalkAction, Operation) {
		switch call := node.(type) {
		case Call:
			if len(call.Args) > 0 {
				if fld, ok := call.Args[0].(Field); ok && (fld.Name == "$MSG" || fld.Name == "$HDR") {
					call.Args = call.Args[1:]
					return WalkReplace, call
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

func translateCalls(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok {
			switch call.Function {
			case "PARMVAL":
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

			case "HDR":
				if len(call.Args) != 1 {
					err = errors.Errorf("at %s: HDR call has more than one argument: %v", call.Source(), call.Hashable())
					return WalkCancel, nil
				}
				return WalkReplace, SetField{
					SourceContext: call.SourceContext,
					Target:        call.Target,
					Value:         []Operation{call.Args[0]},
				}

			case "URL":
				if len(call.Args) != 2 {
					err = errors.Errorf("at %s: URL call has more than 2 arguments: %v", call.Source(), call.Hashable())
					return WalkCancel, nil
				}
				// Extract field names from arguments
				var param [2]string
				for idx, op := range call.Args {
					fld, found := op.(Field)
					if !found {
						err = errors.Errorf("at %s: URL call argument %d is not a field: %v", call.Source(), idx+1, call.Hashable())
						return WalkCancel, nil
					}
					param[idx] = fld.Name
				}
				// Arg 1 is URL component identifier.
				cp, found := VarNameToURLComponent[param[0]]
				if !found {
					err = errors.Errorf("at %s: URL call argument %s is not understood: %v", call.Source(), param[0], call.Hashable())
					return WalkCancel, nil
				}
				return WalkReplace, URLExtract{
					Target:    call.Target,
					Source:    param[1],
					Component: cp,
				}
			}
		}
		return WalkContinue, nil
	})
	return err
}

func stripRegex(parser *Parser) error {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok {
			if _, found := parser.RegexsByName[call.Function]; found {
				return WalkReplace, Noop{}
			}
		}
		return WalkContinue, nil
	})
	return nil
}

func removeNoops(parser *Parser) error {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			var remove []int
			for pos, act := range match.OnSuccess {
				if _, isNop := act.(Noop); isNop {
					remove = append(remove, pos)
				}
			}
			if len(remove) > 0 {
				match.OnSuccess = OpList(match.OnSuccess).Remove(remove)
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return nil
}

func convertEventTime(p *Parser) (err error) {
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok && call.Function == "EVNTTIME" || call.Function == "DUR" {
			numArgs := len(call.Args)
			if numArgs < 2 {
				err = errors.Errorf("at %s: %s call has too little arguments: %d", call.Source(), call.Function, numArgs)
				return WalkCancel, nil
			}
			var numFormats int
			for numFormats = range call.Args {
				if _, ok := call.Args[numFormats].(Constant); !ok {
					break
				}
			}
			if numFormats == 0 {
				err = errors.Errorf("at %s: %s call format is not a constant: %s", call.Source(), call.Function, call.Args[0])
				return WalkCancel, nil
			} else if numFormats == numArgs {
				err = errors.Errorf("at %s: %s has no fields", call.Source(), call.Function)
				return WalkCancel, nil
			}

			fields := make([]string, numArgs-numFormats)
			for idx, arg := range call.Args[numFormats:] {
				if field, ok := arg.(Field); ok {
					fields[idx] = field.Name
				} else {
					err = errors.Errorf("at %s: %s call argument %d is not a field", call.Source(), call.Function, idx+numFormats)
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
					err = errors.Wrapf(err, "at %s: failed to parse %s format '%s'", call.Source(), call.Function, ct.Value())
					return WalkCancel, nil
				}
				repl.Formats[idx] = fmt
			}

			var result Operation = repl
			switch call.Function {
			case "DUR":
				result = Duration(repl)
			case "UTC":
				// UTC call is the same as EVNTTIME but without timezone conversion.
				repl.IsUTC = true
			}
			return WalkReplace, result
		}
		return WalkContinue, nil
	})
	return err
}

func convertDuration(p *Parser) (err error) {
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok && call.Function == "DUR" {
			numArgs := len(call.Args)
			if numArgs < 2 {
				err = errors.Errorf("at %s: DUR call has too little arguments: %d", call.Source(), numArgs)
				return WalkCancel, nil
			}
			var numFormats int
			for numFormats = range call.Args {
				if _, ok := call.Args[numFormats].(Constant); !ok {
					break
				}
			}
			if numFormats != 1 {
				err = errors.Errorf("at %s: DUR call format is not a constant: %s", call.Source(), call.Args[0])
				return WalkCancel, nil
			}
			if numFormats == 0 {
				err = errors.Errorf("at %s: DUR call format is not a constant: %s", call.Source(), call.Args[0])
				return WalkCancel, nil
			} else if numFormats == numArgs {
				err = errors.Errorf("at %s: DUR has no fields", call.Source())
				return WalkCancel, nil
			}

			fields := make([]string, numArgs-numFormats)
			for idx, arg := range call.Args[numFormats:] {
				if field, ok := arg.(Field); ok {
					fields[idx] = field.Name
				} else {
					err = errors.Errorf("at %s: DUR call argument %d is not a field", call.Source(), idx+numFormats)
					return WalkCancel, nil
				}
			}
			repl := Duration{
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
	if !p.Config.Dissect {
		return nil
	}
	p.WalkPostOrder(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok && match.Pattern.HasAlternatives() {
			repl := AllMatch{
				SourceContext: match.SourceContext,
			}
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
						ID:            fmt.Sprintf("%s/%d", match.ID, partCounter),
						Input:         input,
						Pattern:       dupPattern(match.Pattern[pos:end]),
						PayloadField:  "", // TODO
					}
					if found {
						input = "p0" //fmt.Sprintf("p%d", partCounter)
						partCounter++
						node.Pattern = append(node.Pattern, Field{Name: input})
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
				displayCounter := partCounter
				if pos < len(match.Pattern)-1 {
					input = "p0" // fmt.Sprintf("p%d", partCounter)
					part = Field{Name: input}
					partCounter++
				}
				for idx, pattern := range alt {
					m := Match{
						SourceContext: match.SourceContext,
						ID:            fmt.Sprintf("%s/%d_%d", match.ID, displayCounter, idx),
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

			var tmpFields RemoveFields
			// TODO: Remove fields or not?
			/*for i := 0; i < partCounter; i++ {
				tmpFields = append(tmpFields, fmt.Sprintf("p%d", i))
			}*/
			if len(tmpFields) > 0 {
				repl.Nodes = append(repl.Nodes, tmpFields)
			}
			repl.onFailurePos = len(repl.Nodes)
			if len(tmpFields) > 0 {
				repl.Nodes = append(repl.Nodes, tmpFields)
			}
			// TODO: cleanup on failure
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func makeGreedyCaptures(pattern Pattern) (err error) {
	n := len(pattern)
	space := false
	for i := n - 1; i >= 0; i-- {
		switch v := pattern[i].(type) {
		case Constant:
			space = len(v.Value()) > 0 && v.Value()[0] == ' '
		case Field:
			if space {
				pattern[i] = Field{Name: v.Name, Greedy: true}
				space = false
			}
		default:
			// Only constants and fields are expected at the stage this transformation
			// is run. Alternatives have already been stripped.
			return errors.Errorf("unexpected type in pattern: %T", v)
		}
	}
	return nil
}

func makeDissectGreedy(p *Parser) (err error) {
	// When a capture is followed by a space, replace it with a greedy capture.
	p.Walk(func(node Operation) (WalkAction, Operation) {
		if match, ok := node.(Match); ok {
			makeGreedyCaptures(match.Pattern)
		}
		return WalkContinue, nil
	})
	return err
}

func injectSpaceBetweenConsecutiveCaptures(pattern Pattern, loc util.XMLPos, warnings *util.Warnings) Pattern {
	var changed bool
	var fixes []int
	var lastCapture Value
	for idx, op := range pattern {
		switch v := op.(type) {
		case Field:
			if lastCapture != nil {
				// This happens a lot. I think the proper thing is to add
				// a space in between.
				// TODO: Revisit decision and check rsa2elk
				fixes = append(fixes, idx)
				warnings.Addf(loc, "consecutive captures in pattern: %s and %s (injected space)",
					lastCapture, v.Name)
			}
			lastCapture = op
		case Payload:
			lastCapture = op
		case Alternatives:
			for altID, alt := range v {
				if newP := injectSpaceBetweenConsecutiveCaptures(alt, loc, warnings); newP != nil {
					v[altID] = newP
					changed = true
				}
			}
			lastCapture = nil
		default:
			lastCapture = nil
		}
	}
	if len(fixes) > 0 {
		changed = true
		for offset, pos := range fixes {
			newP := make([]Value, 0, len(pattern)+len(fixes))
			newP = append(newP, pattern[:pos+offset]...)
			newP = append(newP, Constant(" "))
			newP = append(newP, pattern[pos+offset:]...)
			pattern = newP
		}
	}
	if changed {
		return pattern
	}
	return nil
}

func fixDissectCaptures(p *Parser) (err error) {
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			if newP := injectSpaceBetweenConsecutiveCaptures(match.Pattern, match.Source(), p.warnings); newP != nil {
				match.Pattern = newP
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return err
}

func getLastOp(pattern Pattern) Value {
	n := len(pattern)
	if n == 0 {
		return nil
	}
	return pattern[n-1]
}

func tryFixAlternativeAtPos(alt Alternatives, pos int, parent Pattern) (Pattern, bool, error) {
	if pos+1 == len(parent) {
		// Alternatives at the end of the pattern don't need adjustment.
		return nil, false, nil
	}
	var endInField []int
	for idx, pattern := range alt {
		lastOp := getLastOp(pattern)
		if lastOp == nil {
			continue
		}
		switch v := lastOp.(type) {
		case Constant:
		case Field:
			endInField = append(endInField, idx)
		case Payload:
			return nil, false, errors.New("payload inside alternative")
		default:
			return nil, false, errors.Errorf("unsupported type inside alternative: %T", v)
		}
	}
	if len(endInField) == 0 {
		// No alternatives end in a field capture: Nothing to fix.
		return nil, false, nil
	}

	// From here there's alternatives ending in field (endInField)
	// and potentially others ending in a constant.

	// Value after the alternative. Only need if it is a constant.
	ct, nextIsConstant := parent[pos+1].(Constant)

	if nextIsConstant {
		// If we have a constant after the alternative, inject into the alternatives.
		alt = alt.InjectRight(ct)
		// Two consecutive constants in some alternatives can cause problems
		// somewhere else.
		for idx, pattern := range alt {
			alt[idx] = pattern.SquashConstants()
		}
		// Remove it from the parent.
		parent = append(parent[:pos+1], parent[pos+2:]...)

		return parent, false, nil
	}
	// We have two field captures in sequence, insert whitespace in between.
	for _, altIdx := range endInField {
		alt[altIdx] = alt[altIdx].InjectRight(Constant(" "))
	}
	return nil, true, nil
}

func extractLeadingConstantPrefix(alt Alternatives) (string, Alternatives) {
	return extractEdgeConstant(alt,
		func(p Pattern) *Value {
			return &p[0]
		},
		func(s string, idx int) byte {
			return s[idx]
		},
		Pattern.StripLeft,
		func(s string, prefix int) string {
			return s[:prefix]
		},
		func(s string, prefix int) string {
			return s[prefix:]
		})
}

func extractTrailingConstantPrefix(alt Alternatives) (string, Alternatives) {
	return extractEdgeConstant(alt,
		func(p Pattern) *Value {
			return &p[len(p)-1]
		},
		func(s string, idx int) byte {
			return s[len(s)-1-idx]
		},
		Pattern.StripRight,
		func(s string, prefix int) string {
			return s[len(s)-prefix:]
		},
		func(s string, prefix int) string {
			return s[:len(s)-prefix]
		})
}

func extractEdgeConstant(
	alt Alternatives,
	patternAccessor func(p Pattern) *Value,
	stringAccessor func(s string, idx int) byte,
	stripFn func(Pattern) Pattern,
	edgeFn func(string, int) string,
	stripEdgeFn func(string, int) string,
) (string, Alternatives) {

	if len(alt) == 0 {
		return "", alt
	}

	// First get a list of all the constant pre/su/fixes
	cts := make([]string, len(alt))
	minLen := -1
	for idx, pattern := range alt {
		if len(pattern) == 0 {
			return "", alt
		}
		ct, ok := (*patternAccessor(pattern)).(Constant)
		if !ok {
			return "", alt
		}
		cts[idx] = ct.Value()
		if n := len(ct.Value()); minLen == -1 || n < minLen {
			minLen = n
		}
	}

	// Then find the common prefix
	prefixLen := 0
outer:
	for ; prefixLen < minLen; prefixLen++ {
		chr := stringAccessor(cts[0], prefixLen)
		for _, str := range cts[1:] {
			if stringAccessor(str, prefixLen) != chr {
				break outer
			}
		}
	}

	var remove []int
	// Then strip all the leading constants
	for idx, pattern := range alt {
		if asCt, ok := (*patternAccessor(pattern)).(Constant); ok {
			if len(asCt.Value()) > prefixLen {
				*patternAccessor(pattern) = Constant(stripEdgeFn(asCt.Value(), prefixLen))
			} else {
				if alt[idx] = stripFn(pattern); len(alt[idx]) == 0 {
					remove = append(remove, idx)
				}
			}
		}
	}
	// Remove patterns that become empty
	if len(remove) > 0 {
		for shift, idx := range remove {
			alt = append(alt[:idx-shift], alt[idx+1-shift:]...)
		}
		// Need to keep an empty pattern otherwise it's messing with the parsing:
		// {JAVA URL|URL} -> {JAVA|} URL
		alt = append(alt, Pattern{})
	}
	// Return the common prefix
	return edgeFn(cts[0], prefixLen), alt
}

func fixAlternativesEdgeSpace(p *Parser) (err error) {
	if !p.Config.Dissect {
		return nil
	}
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok && match.Pattern.HasAlternatives() {
			type insert struct {
				pos int
				ct  Constant
			}
			var inserts []insert
			modified := false
			for pos, item := range match.Pattern {
				alt, ok := item.(Alternatives)
				if !ok {
					continue
				}
				prefix, newAlt := extractLeadingConstantPrefix(alt)
				if prefix == "" {
					continue
				}
				match.Pattern[pos] = newAlt
				if pos > 0 {
					if ct, ok := match.Pattern[pos-1].(Constant); ok {
						match.Pattern[pos-1] = Constant(ct.Value() + prefix)
						continue
					}
				}
				inserts = append(inserts, insert{pos, Constant(prefix)})
			}
			if len(inserts) > 0 {
				match.Pattern = append(match.Pattern, make(Pattern, len(inserts))...)
				for shift, is := range inserts {
					copy(match.Pattern[is.pos+shift+1:], match.Pattern[is.pos+shift:])
					match.Pattern[is.pos+shift] = is.ct
				}
				modified = true
				inserts = inserts[:0]
			}

			for pos, item := range match.Pattern {
				alt, ok := item.(Alternatives)
				if !ok {
					continue
				}

				//prefix, newAlt := extractTrailingConstantPrefix(alt)
				var prefix string
				var newAlt Alternatives
				if false {
					prefix, newAlt = extractTrailingConstantPrefix(alt)
				} else {
					prefix, newAlt = "", Alternatives{}
				}

				if prefix == "" {
					continue
				}
				match.Pattern[pos] = newAlt
				if pos < len(match.Pattern)-1 {
					if ct, ok := match.Pattern[pos+1].(Constant); ok {
						match.Pattern[pos+1] = Constant(prefix + ct.Value())
						continue
					}
				}
				inserts = append(inserts, insert{pos + 1, Constant(prefix)})
			}
			if len(inserts) > 0 {
				match.Pattern = append(match.Pattern, make(Pattern, len(inserts))...)
				for shift, is := range inserts {
					copy(match.Pattern[is.pos+shift+1:], match.Pattern[is.pos+shift:])
					match.Pattern[is.pos+shift] = is.ct
				}
				modified = true
				inserts = inserts[:0]
			}

			if modified {
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return err
}

func fixAlternativesEndingInCapture(p *Parser) (err error) {
	var injected, moved int
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok && match.Pattern.HasAlternatives() {
			// The alternatives may be modified without the enclosing Match
			// needing to be modified.
			modified := false
			n := len(match.Pattern)
			// Alternatives at the end of the pattern are no problem.
			for pos := 0; pos < n-1; pos++ {
				if alt, ok := match.Pattern[pos].(Alternatives); ok {
					var newPattern Pattern
					var isInject bool
					newPattern, isInject, err = tryFixAlternativeAtPos(alt, pos, match.Pattern)
					if err != nil {
						err = errors.Wrapf(err, "at %s", match.Source())
						return WalkCancel, nil
					}
					if newPattern != nil {
						if isInject {
							injected++
						} else {
							moved++
						}
						// This modifies the actual pattern it's looping on,
						// if a new element has been added to next pos.
						// In this case it'll also be necessary to replace
						// the match in the tree with an updated version.
						match.Pattern = newPattern
						n = len(newPattern)
						modified = true
					}
				}
			}
			if modified {
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	if injected+moved > 0 {
		log.Printf("INFO - Fixed field collisions (%d constant moved, %d spaces injected)", moved, injected)
	}
	return err
}

func fixExtraLeadingSpaceInConstants(parser *Parser) error {
	if !parser.Config.Dissect {
		return nil
	}
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		var inserts []int
		if match, ok := node.(Match); ok {
			prevIsField := false
			for pos, elem := range match.Pattern {
				isField := false
				switch v := elem.(type) {
				case Constant:
					if str := strings.TrimLeft(v.Value(), " "); str != v.Value() {
						if prevIsField {
							// EDIT: Not needed anymore since dissect was modified
							//       to strip extra whitespace.
							// - If the previous is a field, always keep one space
							// - otherwise the greedy -> option in dissect will not
							// - consume whitespace and it'll be added to the previous
							// - captured value.

							// This is generating weird patterns
							// v = Constant( " " + str)
							if len(str) == 0 {
								v = Constant(" ")
							} else {
								v = Constant(str)
							}
						} else {
							// If the previous is not a field, then we're at the
							// start of a pattern. Keep the constant (can't be
							// empty) and add a dummy capture to consume any
							// spaces.
							if len(str) == 0 {
								v = Constant(" ")
							} else {
								v = Constant(str)
							}
							inserts = append(inserts, pos)
						}
						match.Pattern[pos] = v
					}
				case Field:
					isField = true
				case Alternatives:
				}
				prevIsField = isField
			}
			if len(inserts) > 0 {
				match.Pattern = append(match.Pattern, make(Pattern, len(inserts))...)
				for shift, pos := range inserts {
					copy(match.Pattern[pos+shift+1:], match.Pattern[pos+shift:])
					match.Pattern[pos+shift] = Field{}
				}
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return nil
}

var extraSpace = regexp.MustCompile(" +")

func removeExtraSpace(str string) string {
	return extraSpace.ReplaceAllString(str, " ")
}

func removeExtraSpaceInPattern(pattern Pattern) (modified bool) {
	for pos, elem := range pattern {
		switch v := elem.(type) {
		case Constant:
			if repl := removeExtraSpace(v.Value()); repl != v.Value() {
				pattern[pos] = Constant(repl)
				modified = true
			}
		case Alternatives:
			for _, p := range v {
				modified = removeExtraSpaceInPattern(p) || modified
			}
		}
	}
	return
}

func fixExtraSpaceInConstants(parser *Parser) error {
	if !parser.Config.Dissect {
		return nil
	}
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			if modified := removeExtraSpaceInPattern(match.Pattern); modified {
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return nil
}

type FunctionInfo struct {
	MinArgs, MaxArgs int
	Stripped         bool
}

var KnownFunctions = map[string]FunctionInfo{
	"CALC":       {MinArgs: 3, MaxArgs: 3},
	"CNVTDOMAIN": {MinArgs: 1, MaxArgs: 1},
	"DIRCHK":     {MinArgs: 1},
	"DUR":        {MinArgs: 3},
	"EVNTTIME":   {MinArgs: 2},
	"RMQ":        {MinArgs: 1, MaxArgs: 1},
	"STRCAT":     {MinArgs: 1},
	"URL":        {MinArgs: 2, MaxArgs: 2},
	"UTC":        {MinArgs: 3, MaxArgs: 3},

	"HDR":     {MinArgs: 1, MaxArgs: 1, Stripped: true},
	"SYSVAL":  {MinArgs: 1, MaxArgs: 2, Stripped: true},
	"PARMVAL": {MinArgs: 1, MaxArgs: 1, Stripped: true},
}

func detectUnknownFunctionCalls(parser *Parser) (err error) {
	unknown := make(map[string]string)
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok {
			if _, seen := unknown[call.Function]; !seen {
				if info, found := KnownFunctions[call.Function]; found {
					if info.Stripped {
						err = errors.Errorf("at %s: CALL to %s should've been stripped", call.Source(), call.Function)
						return WalkCancel, nil
					}
				} else {
					unknown[call.Function] = fmt.Sprintf("'%s' first seen at %s",
						call.Function,
						call.Source())
				}
			}
		}
		return WalkContinue, nil
	})
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		var values []string
		for _, v := range unknown {
			values = append(values, v)
		}
		return errors.Errorf("Found %d unknown functions:\n%s",
			len(values), strings.Join(values, "\n"))
	}
	return nil
}

func detectUnnamedMatches(parser *Parser) (err error) {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			if match.ID == "" {
				err = errors.Errorf("at %s: detected unnamed match (pattern=<<%s>>)",
					match.Source(), match.Pattern.Tokenizer())
				return WalkCancel, nil
			}
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

// This detects if we've generated a broken dissect pattern with two consecutive
// field captures, which currently causes the dissect processor to hang.
// TODO: Fix dissect hanging
func detectBrokenDissectPatterns(parser *Parser) error {
	var errs multierror.Errors
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			prevIsField := false
			var prev string
			for _, item := range match.Pattern {
				fld, isField := item.(Field)
				if isField {
					if prevIsField {
						errs = append(errs,
							errors.Errorf("at %s: consecutive field captures generated (fields %s and %s)",
								match.Source(), prev, fld.Name))
					}
					prev = fld.Name
				}
				prevIsField = isField
			}
		}
		return WalkContinue, nil
	})
	return errs.Err()
}

func injectCapturesInAltsPattern(pattern Pattern) (Pattern, error) {
	var lastField Value
	var remove []int
	for pos, item := range pattern {
		switch v := item.(type) {
		case Alternatives:
			if lastField != nil {
				pattern[pos] = v.InjectLeft(lastField)
				remove = append(remove, pos-1)
			}
		case Field:
			lastField = item
		default:
			lastField = nil
		}
	}
	if len(remove) > 0 {
		return pattern.Remove(remove), nil
	}
	return nil, nil
}

func injectCapturesInAlts(parser *Parser) (err error) {
	if !parser.Config.Dissect {
		return nil
	}
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			var newPattern Pattern
			newPattern, err = injectCapturesInAltsPattern(match.Pattern)
			if err != nil {
				err = errors.Wrapf(err, "at %s: cannot inject trailing capture into alternative: ", match.Source())
				return WalkCancel, nil
			}
			if newPattern != nil {
				match.Pattern = newPattern
				return WalkReplace, match
			}
		}
		return WalkContinue, nil
	})
	return err
}

func stripLeadingSpace(parser *Parser) error {
	if !parser.Config.Fixes.StripLeadingSpace {
		return nil
	}
	count := 0
	for idx, msg := range parser.Messages {
		if n := len(msg.content); n < 1 {
			continue
		}
		if ct, ok := msg.content[0].(Constant); ok {
			if trimmed := strings.TrimLeft(ct.Value(), " "); trimmed != ct.Value() {
				count++
				if len(trimmed) > 0 {
					parser.Messages[idx].content[0] = Constant(trimmed)
				} else {
					parser.Messages[idx].content = msg.content[1:]
				}

			}
		}
	}
	log.Printf("INFO - Trimmed leading space from %d out of %d messages", count, len(parser.Messages))
	return nil
}
