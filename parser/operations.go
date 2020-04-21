//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"sort"
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
	Hashable() string
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

func (c Chain) Hashable() string {
	var sb strings.Builder
	sb.WriteString("Chain{[")
	if len(c.Nodes) > 0 {
		sb.WriteString(c.Nodes[0].Hashable())
		for _, n := range c.Nodes[1:] {
			sb.WriteByte(',')
			sb.WriteString(n.Hashable())
		}
	}
	sb.WriteString("]}")
	return sb.String()
}

func (c Chain) String() string {
	return c.Hashable()
}

type LinearSelect struct {
	SourceContext
	Nodes []Operation
}

func (c LinearSelect) Children() []Operation {
	return c.Nodes
}

func (c LinearSelect) Hashable() string {
	return "LinearSelect{" + OpList(c.Nodes).Hashable() + "}"
}


type MsgIdSelect struct {
	SourceContext
	Nodes   []Operation
	//Default int
	Map     map[string]int
}

func (c MsgIdSelect) Children() []Operation {
	return c.Nodes
}

func (c MsgIdSelect) Hashable() string {
	var sb strings.Builder
	sb.WriteString("MsgIdSelect{")
	keys := make([]string, len(c.Map))
	for k := range c.Map {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteByte('"')
		sb.WriteString(k)
		sb.WriteString("\":")
		sb.WriteString(c.Nodes[c.Map[k]].Hashable())
		sb.WriteByte(',')
	}
	//sb.WriteString(fmt.Sprintf("def=%d}", c.Default))
	return sb.String()
}

type AllMatch struct {
	SourceContext
	Nodes []Operation
	// Nodes must be a list of all operations contained so we can optimize it.
	// so it's necessary to know were some start and others end.
	onSuccessPos int
	onFailurePos int
}

func (am AllMatch) Processors() []Operation {
	return am.Nodes[:am.onSuccessPos]
}

func (am AllMatch) OnSuccess() []Operation {
	return am.Nodes[am.onSuccessPos:am.onFailurePos]
}

func (am AllMatch) OnFailure() []Operation {
	return am.Nodes[am.onFailurePos:]
}

func (am AllMatch) Children() []Operation {
	return am.Nodes
}

func (am AllMatch) Hashable() string {
	return "All{n:" + OpList(am.Processors()).Hashable() +
		",succ=" + OpList(am.OnSuccess()).Hashable() +
		",fail=" + OpList(am.OnFailure()).Hashable() +
		"}"
}

type SwitchSelect struct {
	LinearSelect
	Key       Field
	KeyLength int
	Mapping   map[string]*Operation
}

type Match struct {
	SourceContext
	Input        string
	Pattern      Pattern
	PayloadField string
	OnSuccess    []Operation
}

func (m Match) Children() []Operation {
	return m.OnSuccess
}

func (m Match) Hashable() string {
	var sb strings.Builder
	sb.WriteString("Match{")
	sb.WriteString("Input:" + m.Input)
	sb.WriteString(",Pattern:" + m.Pattern.Hashable())
	sb.WriteString("PayloadField:" + m.PayloadField)
	sb.WriteString("OnSuccess:")
	sb.WriteString(OpList(m.OnSuccess).Hashable())
	sb.WriteByte('}')
	return sb.String()
}

type SetField struct {
	SourceContext
	Target string
	Value  []Operation
}

func (c SetField) Children() []Operation {
	return c.Value
}

type OpList []Operation

func (c OpList) Hashable() string {
	var sb strings.Builder
	comma := false
	sb.WriteByte('[')
	for _, op := range c {
		if comma {
			sb.WriteByte(',')
		}
		sb.WriteString(op.Hashable())
		comma = true
	}
	sb.WriteByte(']')
	return sb.String()
}

func (c SetField) Hashable() string {
	var sb strings.Builder
	sb.WriteString("SetField{")
	sb.WriteString("Target:" + c.Target)
	sb.WriteString(OpList(c.Value).Hashable())
	sb.WriteByte('}')
	return sb.String()
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
		target = "target=" + c.Target + ","
	}
	args := make([]string, len(c.Args))
	for idx, val := range c.Args {
		args[idx] = val.String()
	}
	return fmt.Sprintf("Call(%sfn='%s',%s)", target, c.Function, strings.Join(args, ","))
}

func (c Call) Hashable() string {
	var sb strings.Builder
	sb.WriteString("Call{target:" + c.Target)
	sb.WriteString(",function:" + c.Function)
	sb.WriteString(",args:")
	for _, a := range c.Args {
		sb.WriteString(a.String())
	}
	sb.WriteByte('}')
	return sb.String()
}

// TODO: Sometimes keys are numeric (and hex!) should it support numeric keys
//       in different base? As in 33 for 0x21
// TODO: Values are either quoted (single) or refs to fields (*dport)
type ValueMap struct {
	SourceContext
	Nodes    []Operation
	Name     string
	Default  *Value
	Mappings map[string]int
}

func (v ValueMap) Children() []Operation {
	return v.Nodes
}

func (v ValueMap) Hashable() string {
	var sb strings.Builder
	sb.WriteString("ValueMap{Name:")
	sb.WriteString(v.Name)
	if v.Default != nil {
		sb.WriteString(",Def:")
		sb.WriteString((*v.Default).String())
	}
	sb.WriteString(",Map:{")
	content := make([][2]string, len(v.Mappings))
	for k, idx := range v.Mappings {
		content[idx] = [2]string{k, v.Nodes[idx].Hashable()}
	}
	for _, c := range content {
		sb.WriteByte('"')
		sb.WriteString(c[0])
		sb.WriteString("\":")
		sb.WriteString(c[1])
		sb.WriteString("\",")
	}
	sb.WriteString("}}")
	return sb.String()
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

func (v ValueMapCall) Hashable() string {
	var sb strings.Builder
	sb.WriteString("ValueMapCall{Target:")
	sb.WriteString(v.Target)
	sb.WriteString(",Name:")
	sb.WriteString(v.MapName)
	sb.WriteString(",Key:")
	sb.WriteString(OpList(v.Key).Hashable())
	sb.WriteByte('}')
	return sb.String()
}
