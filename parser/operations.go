//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/model"
)

type Tree struct {
	Root Operation
}

type SourceContext model.XMLPos

type WithSourceContext interface {
	Source() SourceContext
}

type Operation interface {
	Children() []Operation
}

func (b SourceContext) Source() model.XMLPos {
	return model.XMLPos(b)
}

type Chain struct {
	SourceContext
	Nodes []Operation
}

func (c Chain) Children() []Operation {
	return c.Nodes
}

type LinearSelect struct {
	SourceContext
	Nodes []Operation
}

func (c LinearSelect) Children() []Operation {
	return c.Nodes
}

type SwitchSelect struct {
	LinearSelect
	Key Field
	KeyLength int
	Mapping map[string]*Operation
}

type Match struct {
	SourceContext
	Input   string
	Pattern Pattern
	PayloadField string
	OnSuccess []Operation
}

func (m Match) Children() []Operation {
	return m.OnSuccess
}

type SetField struct {
	SourceContext
	Target string
	Value []Operation
}

func (c SetField) Children() []Operation {
	return c.Value
}

type Call struct {
	SourceContext
	Function string
	Target   string
	Args     []Value
}

func (c Call) Children() []Operation {
	// Or args? :/
	return nil
}

func (c Call) String() string {
	var target string
	if c.Target != "" {
		target = "target="+ c.Target + ","
	}
	args := make([]string, len(c.Args))
	for idx, val := range c.Args {
		args[idx] = val.String()
	}
	return fmt.Sprintf("Call(%sfn='%s',%s)", target, c.Function, strings.Join(args, ","))
}


// TODO: Sometimes keys are numeric (and hex!) should it support numeric keys
//       in different base? As in 33 for 0x21
// TODO: Values are either quoted (single) or refs to fields (*dport)
type ValueMap struct {
	SourceContext
	Nodes 	 []Operation
	Name     string
	Default  *Value
	Mappings map[string]int
}

func (v ValueMap) Children() []Operation {
	return v.Nodes
}

type ValueMapCall struct {
	SourceContext
	Target  string
	MapName string
	Key     []Operation
}


func (v ValueMapCall) Children() []Operation {
	return v.Key[:]
}
