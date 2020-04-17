//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type Value interface{
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
	return "%{" + string(f) + "}"
}

func (c Field) Children() []Operation {
	return nil
}

type Pattern []Value

func (p Pattern) String() string {
	items := make([]string, len(p))
	for idx, it := range p {
		items[idx] = it.String()
	}
	return fmt.Sprintf("Pattern{%s}", strings.Join(items, ", "))
}

func (c Pattern) Children() []Operation {
	return nil
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

type Tokenizer interface {
	Token() string
}

func (c Pattern) Tokenizer() string {
	parts := make([]string, len(c))
	for idx, val := range c {
		parts[idx] = val.Token()
	}
	return strings.Join(parts, "")
}

type Payload Field

func (c Payload) String() string {
	return "Payload(" + Field(c).String() + ")"
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
		return Constant(s[1:n-1]), nil
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
