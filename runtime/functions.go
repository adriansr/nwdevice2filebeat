//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"bytes"
	"net"
	"strconv"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

type FunctionImpl func([]string, *Context) (string, error)

var knownFunctions = map[string]FunctionImpl{
	"CALC":       calc,
	"CNVTDOMAIN": notimpl,
	"DIRCHK":     networkDirection,
	"RMQ":        removeQuotes,
	"STRCAT":     strcat,
	"URL":        notimpl,

	// These functions must already be pruned or translated, and observing
	// them in the context of a function call is an error.
	//"DUR":      forbidden,
	//"EVNTTIME": forbidden,
	//"HDR":      forbidden,
	//"SYSVAL":   forbidden,
	//"PARMVAL":  forbidden,
	//"UTC":      forbidden,

}

type Argument interface {
	Get(*Context) (string, error)
}

type Function struct {
	Name    string
	Target  string
	Args    []Argument
	Handler FunctionImpl
}

func (f Function) Run(ctx *Context) (err error) {
	args := make([]string, len(f.Args))
	for idx, arg := range f.Args {
		if args[idx], err = arg.Get(ctx); err != nil {
			return errors.Wrapf(err, "fetching argument %v for %s=%s call", arg, f.Target, f.Name)
		}
	}
	value, err := f.Handler(args, ctx)
	if err != nil {
		return errors.Wrapf(err, "running %s='%s' on args:%v", f.Target, f.Name, args)
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
		Name:    name,
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

func calc(args []string, _ *Context) (string, error) {
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

func strcat(args []string, _ *Context) (string, error) {
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

func notimpl([]string, *Context) (string, error) {
	return "", ErrNotImplemented
}

var quoteChars = []byte("\"'`")

var (
	errOneArgument     = errors.New("function requires exactly one argument")
	errDirChkArguments = errors.New("only single-argument form is supported")
)

func removeQuotes(args []string, _ *Context) (string, error) {
	// RMQ always has one argument.
	if len(args) != 1 {
		return "", errOneArgument
	}
	str := strings.TrimSpace(args[0])
	n := len(str)
	if n > 1 {
		q := str[0]
		if bytes.IndexByte(quoteChars, q) >= 0 && str[n-1] == q {
			return str[1 : n-1], nil
		}
	}
	return str, nil
}

const (
	dirChkInside  = "1"
	dirChkOutside = "0"
)

func networkDirection(args []string, ctx *Context) (string, error) {
	// The different forms of this function are:
	// DIRCHK(saddr) => '1' if addr is Inside '0' if Outside.
	// DIRCHK($IN/$OUT,saddr,daddr)
	// DIRCHK($IN/$OUT,saddr,daddr,sport,dport)
	//
	// Don't know what the multiargument ones mean exactly and what is a proper
	// result. These are only used in Netscreen log parser.
	if len(args) != 1 {
		return dirChkOutside, errDirChkArguments
	}
	ip := net.ParseIP(args[0])
	if ip == nil {
		return dirChkOutside, errors.Errorf("failed to parse '%s' as IP for network direction check.", args[0])
	}
	// TODO: Implement this better than O(n)
	for _, net := range ctx.Config.Runtime.LocalNetworks {
		if net.Contains(ip) {
			return dirChkOutside, nil
		}
	}
	return dirChkInside, nil
}
