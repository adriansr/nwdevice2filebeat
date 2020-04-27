//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"strings"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/adriansr/nwdevice2filebeat/util"
)

var ErrFieldNotFound = errors.New("field not found")

// This is a duration processor that will convert a field duration in the
// HH:mm:ss format into a duration in seconds.
// This would be the job of a DUR("%Z",duration) action in the log parser
// but a lot of cases are observed where duration is left unconverted in
// this format, which leads me to think that the conversion is automatic.
var convertDuration = duration{
	target:  "duration",
	fields:  []string{"duration"},
	formats: [][]byte{[]byte("HTS")},
}

type Processor struct {
	Root      Node
	logger    util.VerbosityLogger
	valueMaps map[string]*valueMap
	cfg       *config.Config
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

func New(parser *parser.Parser, warnings *util.Warnings, logger util.Logger) (p *Processor, err error) {
	if logger == nil {
		logger = util.DontLog{}
	}
	p = &Processor{
		valueMaps: make(map[string]*valueMap, len(parser.ValueMapsByName)),
		logger: util.VerbosityLogger{
			Logger:   logger,
			MaxLevel: parser.Config.Verbosity,
		},
		cfg: &parser.Config,
	}
	for name, vm := range parser.ValueMapsByName {
		if p.valueMaps[name], err = newValueMap(vm, parser); err != nil {
			return nil, err
		}
	}
	p.Root, err = p.translate(parser.Root)
	return p, err
}

func (p *Processor) Process(msg []byte) (fields Fields, errs multierror.Errors) {
	ctx := Context{
		Message:  msg,
		Fields:   make(Fields),
		Warnings: util.NewWarnings(20),
		Logger:   p.logger,
		Config:   p.cfg,
	}
	if err := p.Root.Run(&ctx); err != nil {
		return nil, append(errs, err)
	}
	p.convert(&ctx)
	return ctx.Fields, ctx.Errors
}

func (p *Processor) convert(ctx *Context) {
	if dur, found := ctx.Fields["duration"]; found && strings.IndexByte(dur, ':') != -1 {
		// Duration is not numeric, try to convert it in HH:mm:ss format.
		convertDuration.Run(ctx)
	}
}
