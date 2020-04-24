//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"github.com/pkg/errors"
)

type Context struct {
	Message []byte
	Fields  Fields
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
			//return err
		}
	}
	// TODO
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
		return err
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

func (m MapSelect) Run(ctx *Context) error {
	mID, err := ctx.Fields.Get("messageid")
	if err != nil {
		return err
	}
	if next, found := m[mID]; found {
		return next.Run(ctx)
	}
	return ErrMessageIDNotFound
}
