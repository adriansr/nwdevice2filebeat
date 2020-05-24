//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package output

import (
	"io"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

// Output is an interface to write a parser into a stream in a specific output
// language.
type Output interface {
	Settings() config.PipelineSettings
	Generate(parser parser.Parser, dest io.Writer) error
}

type registry map[string]Output

// Registry allows to instantiate outputs by name.
var Registry = registry{}

func (r registry) Register(name string, output Output) error {
	if _, exists := r[name]; exists {
		return errors.Errorf("output %s already registered", name)
	}
	r[name] = output
	return nil
}

func (r registry) MustRegister(name string, output Output) {
	if err := r.Register(name, output); err != nil {
		panic(err)
	}
}

func (r registry) Get(name string) (Output, error) {
	if output, found := r[name]; found {
		return output, nil
	}
	return nil, errors.Errorf("unsupported output: %s", name)
}
