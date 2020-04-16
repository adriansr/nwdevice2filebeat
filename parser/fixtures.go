//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"log"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"
)

type Fixture struct {
	Name string
	Run  func(parser *Parser) error
}

var fixtures = []Fixture {
	{"translate valuemap references", convertValueMapReferences},
}

func convertValueMapReferences(parser *Parser) error {
	var errs multierror.Errors

	Walk(parser, func(node Operation) (WalkAction, Operation) {
		switch call := node.(type) {
		case Call:
			if _, found := parser.ValueMapsByName[call.Function]; found {
				if len(call.Args) != 1 {
					errs = append(errs, errors.Errorf("at %s: call to VALUEMAP must have exactly 1 argument but has %d",
						call.Source(), len(call.Args)))
				}
				return WalkReplace, ValueMapCall{
					SourceContext: call.SourceContext,
					MapName: call.Function,
					Target:  call.Target,
					Key:     [1]Operation{call.Args[0]},
				}
			}
		}
		return WalkContinue, nil
	})
	return errs.Err()
}

func (parser *Parser) Apply(fixtures []Fixture) error {
	for _, fixture := range fixtures {
		if err := fixture.Run(parser); err != nil {
			return errors.Wrapf(err, "error applying fixture %s", fixture.Name)
		}
	}
	return nil
}

func validate(parser *Parser) (err error) {
	const OpLimit = 50000000
	count := 0
	Walk(parser, func(node Operation) (WalkAction, Operation) {
		if count++; count == OpLimit {
			err = errors.Errorf("bug or device definition too large: tree traversal exceeded limit of %d nodes", OpLimit)
			return WalkCancel, nil
		}
		switch v := node.(type) {
		case Call:
			if v.Target == "" && v.Function != "SYSVAL" {
				err = errors.Errorf("at %s: call to %s function doesn't have a target", v.Source(), v.Function)
				return WalkCancel, nil
			}
		}
		return WalkContinue, nil
	})
	log.Printf("Validated tree of %d nodes\n", count)
	return
}

