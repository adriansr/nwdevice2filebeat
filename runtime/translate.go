//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"log"

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
		log.Printf("match %s has %d ops:", v.ID, len(v.OnSuccess))
		for idx, op := range v.OnSuccess {
			log.Printf("   [%d] = %s", idx, op.Hashable())
			if match.onSuccess[idx], err = proc.translate(op, p); err != nil {
				return nil, errors.Wrap(err, "error translating pattern's onsuccess")
			}
		}
		return &match, nil

	case parser.SetField:
		switch val := v.Value[0].(type) {
		case parser.Constant:
			return &SetConstant{
				Field: v.Target,
				Value: val.Value(),
			}, nil
		case parser.Field:
			if len(val.Name()) == 0 {
				return nil, errors.New("empty field name in SetField")
			}
			if val.Name()[0] == '$' {
				switch val.Name() {
				case "$MSG":
					return CopyMsg(v.Target), nil
				default:
					return nil, errors.Errorf("Don't know how to SetField from '%s'", val.Name)
				}
			}
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
		return newDateTime(v, p.Config.Timezone)

	case parser.Duration:
		return newDuration(v)

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
