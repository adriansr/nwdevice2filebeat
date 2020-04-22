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
		{"adjust payload field", setPayloadField},
	},
}

var transforms = PostprocessGroup{
	Title: "transforms",
	Actions: []Action{
		// Replaces a Call() to a MalueMap with a ValueMapCall.
		{"translate VALUEMAP references", convertValueMapReferences},
		{"translate PARMVAL calls", translateParmval},
		{"strip REGX calls", stripRegex},
		{"remove no-ops in function lists", removeNoops},

		// Remove unnecessary $MSG and $HDR as first argument to calls
		{"remove special fields from some calls", removeSpecialFields},
		{"log special field usage", checkSpecialFields},

		// Convert EVNTTIME calls
		{"convert EVNTTIME calls", convertEventTime},
		// TODO:
		// Replace SYSVAL references with fields from headers (id1, messageid, etc.)

		{"fix consecutive dissect captures", fixDissectCaptures},

		{"fix repetition at edge of alternatives", fixAlternativesEdgeSpace},

		{"fix consecutive dissect captures in alternatives", fixAlternativesEndingInCapture},

		{"fix extra leading space in constants (1)", fixExtraLeadingSpaceInConstants},

		{"split alternatives into dissect patterns", splitDissect},

		//{"fix extra leading space in constants (2)", fixExtraLeadingSpaceInConstants},
		{"fix extra space in constants", fixExtraSpaceInConstants},
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

func stripRegex(parser *Parser) error {
	parser.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if call, ok := node.(Call); ok {
			if _, found := parser.RegexsByName[call.Function]; found {
				log.Printf("INFO - at %s: Stripping call to REGX %s (unsupported feature)",
					call.Source(),
					call.Function)
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
				displayCounter := partCounter
				if pos < len(match.Pattern)-1 {
					input = fmt.Sprintf("p%d", partCounter)
					part = Field(input)
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
			repl.onFailurePos = len(repl.Nodes)
			// TODO: cleanup on failure
			return WalkReplace, repl
		}
		return WalkContinue, nil
	})
	return err
}

func fixDissectCaptures(p *Parser) (err error) {
	p.Walk(func(node Operation) (action WalkAction, operation Operation) {
		if match, ok := node.(Match); ok {
			// First check if a pattern has consecutive captures outside of an
			// alternative.
			var fixes []int
			lastWasCapture, lastCapture := false, ""
			for idx, op := range match.Pattern {
				isCapture, capture := false, ""
				switch v := op.(type) {
				case Field:
					isCapture = true
					capture = v.Name()
				case Payload:
					isCapture = true
					capture = v.FieldName()
				}
				if isCapture && lastWasCapture {
					// This happens a lot. I think the proper thing is to add
					// a space in between.
					// TODO: Revisit decision and check rsa2elk
					fixes = append(fixes, idx)
					log.Printf("INFO at %s: pattern has two consecutive captures: %s and %s (fixed)",
						match.Source(), lastCapture, capture)
				}
				lastWasCapture = isCapture
				lastCapture = capture
			}
			if len(fixes) > 0 {
				for offset, pos := range fixes {
					pattern := make([]Value, 0, len(match.Pattern)+len(fixes))
					pattern = append(pattern, match.Pattern[:pos+offset]...)
					pattern = append(pattern, Constant(" "))
					pattern = append(pattern, match.Pattern[pos+offset:]...)
					match.Pattern = pattern
				}
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

func tryFixAlternativeAtPos(alt Alternatives, pos int, parent Pattern) (Pattern, error) {
	if pos+1 == len(parent) {
		// Alternatives at the end of the pattern don't need adjustment.
		return nil, nil
	}
	var endInField []int
	for idx, pattern := range alt {
		lastOp := getLastOp(pattern)
		if lastOp == nil {
			//return nil, errors.New("empty pattern inside alternatives")
			continue
		}
		switch v := lastOp.(type) {
		case Constant:
		case Field:
			endInField = append(endInField, idx)
		case Payload:
			return nil, errors.New("payload inside alternative")
		default:
			return nil, errors.Errorf("unsupported type inside alternative: %T", v)
		}
	}
	if len(endInField) == 0 {
		// No alternatives end in a field capture: Nothing to fix.
		return nil, nil
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

		log.Printf("INFO - Fixed field collision by moving a constant")
		return parent, nil
	}
	// We have two field captures in sequence, insert whitespace in between.
	for _, altIdx := range endInField {
		alt[altIdx] = alt[altIdx].InjectRight(Constant(" "))
	}
	log.Printf("INFO - Fixed field collision by injecting a constant")
	return nil, nil
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
				prefix, newAlt := extractTrailingConstantPrefix(alt)
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
					newPattern, err = tryFixAlternativeAtPos(alt, pos, match.Pattern)
					if err != nil {
						err = errors.Wrapf(err, "at %s", match.Source())
						return WalkCancel, nil
					}
					if newPattern != nil {
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
	return
}

func fixExtraLeadingSpaceInConstants(parser *Parser) error {
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
					match.Pattern[pos+shift] = Field("")
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

	// TODO: Prune this ones
	"HDR":     {MinArgs: 1, MaxArgs: 1 /*Stripped: true*/},
	"SYSVAL":  {MinArgs: 1, MaxArgs: 2 /*Stripped: true*/},
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
								match.Source(), prev, fld.Name()))
					}
					prev = fld.Name()
				}
				prevIsField = isField
			}
		}
		return WalkContinue, nil
	})
	return errs.Err()
}
