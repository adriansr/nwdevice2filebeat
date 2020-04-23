//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

type Value interface {
	fmt.Stringer
	Operation
	// TODO: Move token to it's own interface. Value should be anything that
	//       you can use as a value to set a field.
	Token() string
}

type Constant string

func (c Constant) String() string {
	return "Constant('" + string(c) + "')"
}

func (c Constant) Hashable() string {
	return c.String()
}

func (c Constant) Children() []Operation {
	return nil
}

func (c Constant) Token() string {
	return string(c)
}

func (c Constant) Value() string {
	return string(c)
}

type Field string

func (c Field) String() string {
	return "Field(" + string(c) + ")"
}

func (c Field) Name() string {
	return string(c)
}

func (f Field) Token() string {
	if len(f) == 0 {
		return "%{}"
	}
	//return "%{" + string(f) + "->}"
	return "%{" + string(f) + "}"
}

func (c Field) Children() []Operation {
	return nil
}

func (c Field) Hashable() string {
	return c.String()
}

type Alternatives []Pattern

func (a Alternatives) String() string {
	var sb strings.Builder
	sb.WriteString("Alt{")
	if len(a) > 0 {
		sb.WriteString(a[0].String())
		for _, p := range a[1:] {
			sb.WriteByte(',')
			sb.WriteString(p.String())
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func (a Alternatives) Hashable() string {
	return a.String()
}

func (Alternatives) Children() []Operation {
	// TODO: Sure?
	return nil
}

func (Alternatives) Token() string {
	return "<<alternative>>"
}

func (alt Alternatives) InjectRight(v Value) Alternatives {
	for idx, pattern := range alt {
		alt[idx] = pattern.InjectRight(v)
	}
	return alt
}

func (alt Alternatives) SquashConstants() Alternatives {
	for idx, pattern := range alt {
		alt[idx] = pattern.SquashConstants()
	}
	return alt
}

func (alt Alternatives) InjectLeft(v Value) Alternatives {
	for idx, pattern := range alt {
		alt[idx] = pattern.InjectLeft(v)
	}
	return alt
}

func (alt Alternatives) StripLeft() Alternatives {
	for idx, pattern := range alt {
		alt[idx] = pattern.StripLeft()
	}
	return alt
}

func (alt Alternatives) StripRight() Alternatives {
	for idx, pattern := range alt {
		alt[idx] = pattern.StripRight()
	}
	return alt
}

type Pattern []Value

func (p Pattern) String() string {
	items := make([]string, len(p))
	for idx, it := range p {
		items[idx] = it.String()
	}
	return fmt.Sprintf("Pattern{%s}", strings.Join(items, ", "))
}

func (p Pattern) Hashable() string {
	return p.String()
}

func (p Pattern) Children() []Operation {
	return nil
}

func (p Pattern) HasAlternatives() bool {
	for _, elem := range p {
		if _, found := elem.(Alternatives); found {
			return true
		}
	}
	return false
}

var ErrNoPayload = errors.New("no payload field in expression")

func (c Pattern) PayloadField() (field string, err error) {
	err = ErrNoPayload
	for _, elem := range c {
		if payload, ok := elem.(Payload); ok {
			if err == nil {
				return field, errors.New("multiple payload fields in pattern")
			}
			field = payload.FieldName()
			err = nil
		}
	}
	return
}

func (p Pattern) StripLeft() Pattern {
	if len(p) == 0 {
		return nil
	}
	return p[1:]
}

func (p Pattern) StripRight() Pattern {
	n := len(p)
	if n == 0 {
		return nil
	}
	return p[:n-1]
}

func (p Pattern) InjectRight(v Value) Pattern {
	return append(p, v)
}

func (p Pattern) InjectLeft(v Value) Pattern {
	return append(append(Pattern(nil), v), p...)
}

func (p Pattern) Remove(indices []int) Pattern {
	sort.Ints(indices)
	last := -1
	removed, n := 0, len(p)
	for shift, pos := range indices {
		if pos != last && pos >= 0 && pos < n {
			copy(p[pos-shift:], p[pos-shift+1:])
			last = pos
			removed++
		}
	}
	return p[:len(p)-removed]
}

// SquashConstants joins together consecutive constants in a pattern.
func (p Pattern) SquashConstants() Pattern {
	var output Pattern
	lastCt := -2
	for _, entry := range p {
		ct, isConstant := entry.(Constant)
		if isConstant {
			if n := len(output); lastCt == n-1 {
				output[lastCt] = Constant(output[lastCt].(Constant).Value() + ct.Value())
				continue
			} else {
				lastCt = n
			}
		}
		output = append(output, entry)
	}
	return output
}

type Tokenizer interface {
	Token() string
}

func (c Pattern) Tokenizer() string {
	parts := make([]string, len(c))
	for idx, val := range c {
		parts[idx] = val.Token()
	}
	s := strings.Join(parts, "")
	if len(s) == 0 {
		return "%{}"
	}
	return s
}

type Payload Field

func (c Payload) String() string {
	return "Payload(" + Field(c).String() + ")"
}

func (c Payload) Hashable() string {
	return c.String()
}

func (c Payload) FieldName() string {
	return string(c)
}

func (c Payload) Children() []Operation {
	return nil
}

func (p Payload) Token() string {
	if len(p) == 0 {
		return "%{payload}"
	}
	return "%{" + string(p) + "}"
}

func newValue(s string, unquotedMeansField bool) (Value, error) {
	n := len(s)
	if n == 0 {
		return nil, errors.New("empty value not allowed")
	}
	switch s[0] {
	case '\'':
		if s[n-1] != '\'' {
			return nil, errors.Errorf("badly quoted value: <<%s>>", s)
		}
		return Constant(s[1 : n-1]), nil
	case '*':
		name := s[1:]
		if !fieldNameRegex.MatchString(name) {
			return nil, errors.Errorf("field name in reference:<<%s>> does not match regex:<<%s>>", name, fieldNameRegex)
		}
		// TODO: This was FieldRef, but the distinction doesn't seem necessary.
		return Field(name), nil
	default:
		if unquotedMeansField {
			if !fieldNameRegex.MatchString(s) {
				return nil, errors.Errorf("field name:<<%s>> does not match regex:<<%s>>", s, fieldNameRegex)
			}
			return Field(s), nil
		} else {
			return Constant(s), nil
		}
	}
}
