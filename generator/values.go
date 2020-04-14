//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type Value interface{
	fmt.Stringer
	IsValue() // TODO: Just for membership
}

type Constant string

func (c Constant) String() string {
	return "Constant('" + string(c) + "')"
}

func (c Constant) IsValue() {}

type FieldRef string

func (c FieldRef) String() string {
	return "FieldRef(*" + string(c) + ")"
}

func (c FieldRef) IsValue() {}

type Field string

func (c Field) IsValue() {}

func (c Field) String() string {
	return "Field(" + string(c) + ")"
}

type Call struct {
	Function string
	Target   string
	Args     []Value
}

func (c Call) IsValue() {}

func (c Call) String() string {
	args := make([]string, len(c.Args))
	for idx, val := range c.Args {
		args[idx] = val.String()
	}
	return fmt.Sprintf("Call(fn='%s',%s)", c.Function, strings.Join(args, ","))
}

type Pattern []Value

func (p Pattern) IsValue() {}

func (p Pattern) String() string {
	items := make([]string, len(p))
	for idx, it := range p {
		items[idx] = it.String()
	}
	return fmt.Sprintf("Pattern{%s}", strings.Join(items, ", "))
}

type Payload Field

func (c Payload) IsValue() {}

func (c Payload) String() string {
	return "Payload(" + Field(c).String() + ")"
}

var fieldNameRegex = regexp.MustCompile(`^[a-zA-Z_0-9$\.]+$`)
var functionNameRegex = regexp.MustCompile(`^[A-Za-z_0-9]+$`)

func newValue(s string) (Value, error) {
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
		return FieldRef(name), nil
	default:
		if !fieldNameRegex.MatchString(s) {
			return nil, errors.Errorf("field name:<<%s>> does not match regex:<<%s>>", s, fieldNameRegex)
		}
		return Field(s), nil
	}
}
