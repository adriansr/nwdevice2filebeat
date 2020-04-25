//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"
)

var ErrFieldNotFound = errors.New("field not found")

type Processor struct {
	Root Node

	valueMaps map[string]*valueMap
}

type Fields map[string]string

func (f Fields) Get(name string) (string, error) {
	if value, found := f[name]; found {
		return value, nil
	}
	return "", ErrFieldNotFound
}

func (f Fields) Put(name, value string) {
	f[name] = value
}

func New(parser *parser.Parser) (p *Processor, err error) {
	p = &Processor{
		valueMaps: make(map[string]*valueMap, len(parser.ValueMapsByName)),
	}
	for name, vm := range parser.ValueMapsByName {
		if p.valueMaps[name], err = newValueMap(vm, parser); err != nil {
			return nil, err
		}
	}
	p.Root, err = p.translate(parser.Root, parser)
	return p, err
}

func (p *Processor) Process(msg []byte) (fields Fields, errs multierror.Errors) {
	ctx := Context{
		Message: msg,
		Fields:  make(Fields),
	}
	if err := p.Root.Run(&ctx); err != nil {
		return nil, append(errs, err)
	}
	return ctx.Fields, ctx.Errors
}
