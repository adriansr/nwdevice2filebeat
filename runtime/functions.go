//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"strconv"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

type FunctionImpl func([]string) (string, error)

var knownFunctions = map[string]FunctionImpl{
	"CALC":       calc,
	"CNVTDOMAIN": notimpl,
	"DIRCHK":     notimpl,
	"DUR":        notimpl,
	"EVNTTIME":   notimpl,
	"RMQ":        notimpl,
	"STRCAT":     strcat,
	"URL":        notimpl,
	"UTC":        notimpl,

	// TODO: Prune this ones
	"HDR":     noop,
	"SYSVAL":  noop,
	"PARMVAL": noop,
}

type Argument interface {
	Get(*Context) (string, error)
}

type Function struct {
	Target  string
	Args    []Argument
	Handler FunctionImpl
}

func (f Function) Run(ctx *Context) (err error) {
	args := make([]string, len(f.Args))
	for idx, arg := range f.Args {
		args[idx], err = arg.Get(ctx)
	}
	value, err := f.Handler(args)
	if err != nil {
		return err
	}
	ctx.Fields[f.Target] = value
	return nil
}

type constant string

type field string

func (c constant) Get(*Context) (string, error) {
	return string(c), nil
}

func (f field) Get(ctx *Context) (string, error) {
	return ctx.Fields.Get(string(f))
}

func newFunction(name string, target string, args []parser.Value) (Node, error) {
	handler, found := knownFunctions[name]
	if !found {
		return nil, errors.Errorf("unsupported function '%s'", name)
	}
	f := Function{
		Target:  target,
		Args:    make([]Argument, len(args)),
		Handler: handler,
	}
	for idx, value := range args {
		switch v := value.(type) {
		case parser.Constant:
			f.Args[idx] = constant(v.Value())
		case parser.Field:
			f.Args[idx] = field(v.Name())
		default:
			return nil, errors.Errorf("unknown value in function call argument: %T", v)
		}
	}
	return f, nil
}

var ErrNotImplemented = errors.New("function not implemented")

func calc(args []string) (string, error) {
	// This is already checked to be 3 arguments.
	a, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		a = 0
	}
	b, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		b = 0
	}
	var r int64
	switch args[1] {
	case "+":
		r = a + b
	case "-":
		r = a - b
	case "*":
		r = a * b
	}
	return strconv.FormatInt(r, 10), nil
}

func strcat(args []string) (string, error) {
	var n int
	for _, arg := range args {
		n += len(arg)
	}
	result := make([]byte, 0, n)
	for _, arg := range args {
		result = append(result, arg...)
	}
	return string(result), nil
}

func notimpl([]string) (string, error) {
	return "", ErrNotImplemented
}

func noop([]string) (string, error) {
	return "", nil
}
