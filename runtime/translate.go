//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

func (proc *Processor) translate(op parser.Operation, p *parser.Parser) (result Node, err error) {
	switch v := op.(type) {
	case parser.Chain:
		chain := Chain{
			Nodes: make([]Node, len(v.Nodes)),
		}
		for idx, op := range v.Children() {
			if chain.Nodes[idx], err = proc.translate(op, p); err != nil {
				return nil, err
			}
		}
		return &chain, nil

	case parser.LinearSelect:
		sel := LinearSelect{
			Nodes: make([]Node, len(v.Nodes)),
		}
		for idx, op := range v.Children() {
			if sel.Nodes[idx], err = proc.translate(op, p); err != nil {
				return nil, err
			}
		}
		return &sel, nil

	case parser.Match:
		pattern, err := newPattern(v.Pattern)
		if err != nil {
			return nil, errors.Wrap(err, "error converting pattern")
		}
		match := match{
			pattern:   pattern,
			onSuccess: make([]Node, len(v.OnSuccess)),
		}
		for idx, op := range v.OnSuccess {
			if match.onSuccess[idx], err = proc.translate(op, p); err != nil {
				return nil, errors.Wrap(err, "error translating pattern's onsuccess")
			}
		}
		return &match, nil

	/* Runtime doesn't receive AllMatch because it's not breaking patterns
	   into dissect patterns.
	case parser.AllMatch:
		chain := AllMatch{
			Nodes: make([]Node, len(v.Nodes)),
		}
		for idx, op := range v.Processors() {
			if chain.Nodes[idx], err = translate(op, p); err != nil {
				return nil, err
			}
		}
		//onSuccess := v.OnSuccess()
		return &chain, nil*/

	case parser.SetField:
		switch val := v.Value[0].(type) {
		case parser.Constant:
			return &SetConstant{
				Field: v.Target,
				Value: val.Value(),
			}, nil
		case parser.Field:
			return &CopyField{
				Dst: v.Target,
				Src: val.Name(),
			}, nil

		default:
			return nil, errors.Errorf("unexpected type in SetField value: %T", val)
		}

	case parser.Call:
		return newFunction(v.Function, v.Target, v.Args)

	case parser.MsgIdSelect:
		node := MapSelect{}
		for k, idx := range v.Map {
			if node[k], err = proc.translate(v.Nodes[idx], p); err != nil {
				return nil, err
			}
		}
		return node, nil

	case parser.DateTime:
		// TODO
		return DateTime{}, nil

	case parser.ValueMapCall:
		vm, ok := proc.valueMaps[v.MapName]
		if !ok {
			return nil, errors.Errorf("access to unknown valuemap: %s", v.MapName)
		}
		if len(v.Key) != 1 {
			return nil, errors.Errorf("bad key at valuemap call for: %s", v.MapName)
		}
		key, err := newValue(v.Key[0])
		if err != nil {
			return nil, errors.Wrapf(err, "bad key at valuemap call for: %s", v.MapName)
		}
		return valueMapCall{
			valueMap: vm,
			key:      key,
			target:   v.Target,
		}, nil
	default:
		return nil, errors.Errorf("unknown type to translate: %T", v)
	}
}
