//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"fmt"
	"os"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"
)

type Context struct {
	Message []byte
	Fields  Fields
	Errors  multierror.Errors
}

type Node interface {
	Run(*Context) error
}

type Chain struct {
	Nodes []Node
}

func (c *Chain) Run(ctx *Context) error {
	for _, sub := range c.Nodes {
		if err := sub.Run(ctx); err != nil {
			ctx.Errors = append(ctx.Errors, err)
		}
	}
	return nil
}

var ErrLinearSelectFailed = errors.New("linear select failed")

type LinearSelect struct {
	Nodes []Node
}

func (c *LinearSelect) Run(ctx *Context) error {
	for _, sub := range c.Nodes {
		if err := sub.Run(ctx); err == nil {
			return nil
		}
	}
	return ErrLinearSelectFailed
}

type AllMatch struct {
	Nodes []Node
}

func (c *AllMatch) Run(ctx *Context) error {
	for _, sub := range c.Nodes {
		if err := sub.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}

type SetConstant struct {
	Field, Value string
}

func (c *SetConstant) Run(ctx *Context) error {
	ctx.Fields[c.Field] = c.Value
	return nil
}

type CopyField struct {
	Src, Dst string
}

func (c *CopyField) Run(ctx *Context) error {
	value, err := ctx.Fields.Get(c.Src)
	if err != nil {
		return errors.Errorf("fetching source field '%s' doesn't exists", c.Src)
	}
	ctx.Fields[c.Dst] = value
	return nil
}

type RemoveFields []string

func (r RemoveFields) Run(ctx *Context) error {
	for _, fld := range r {
		delete(ctx.Fields, fld)
	}
	return nil
}

type MapSelect map[string]Node

var ErrMessageIDNotFound = errors.New("messageid not found")
var ErrMessageIDNotMapped = errors.New("no mapping for messageid")

func (m MapSelect) Run(ctx *Context) error {
	mID, err := ctx.Fields.Get("messageid")
	if err != nil {
		return ErrMessageIDNotFound
	}
	if next, found := m[mID]; found {
		return next.Run(ctx)
	}
	return ErrMessageIDNotMapped
}

type valueMapEntry interface {
	Get(ctx *Context) string
}

type ct string

func (c ct) Get(*Context) string {
	return string(c)
}

type fld string

func (f fld) Get(ctx *Context) string {
	value, err := ctx.Fields.Get(string(f))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Get of unset field: %s\n", string(f))
		return ""
	}
	return value
}

func newValue(op parser.Operation) (valueMapEntry, error) {
	switch v := op.(type) {
	case parser.Constant:
		return ct(v.Value()), nil
	case parser.Field:
		return fld(v.Name()), nil
	default:
		return nil, errors.Errorf("unexpected type in valuemap: %T", v)
	}
}

type valueMap struct {
	mappings map[string]valueMapEntry
	def      *valueMapEntry
}

func newValueMap(vm *parser.ValueMap, p *parser.Parser) (out *valueMap, err error) {
	out = &valueMap{
		mappings: make(map[string]valueMapEntry, len(vm.Mappings)),
	}
	if vm.Default != nil {
		def, err := newValue(*vm.Default)
		if err != nil {
			return nil, errors.Wrap(err, "failed translating valuemap default")
		}
		out.def = &def
	}
	for k, idx := range vm.Mappings {
		// parser.ValueMap nodes are actually only Values (Constant or Field),
		// but are typed parser.Operation so that they can be traversed.
		if out.mappings[k], err = newValue(vm.Nodes[idx]); err != nil {
			return nil, errors.Wrap(err, "failed translating valuemap entry")
		}
	}
	return out, nil
}

type valueMapCall struct {
	valueMap *valueMap
	key      valueMapEntry
	target   string
}

func (call valueMapCall) Run(ctx *Context) error {
	key := call.key.Get(ctx)
	entry, found := call.valueMap.mappings[key]
	if !found {
		if call.valueMap.def == nil {
			return nil
		}
		entry = *call.valueMap.def
	}
	ctx.Fields.Put(call.target, entry.Get(ctx))
	return nil
}
