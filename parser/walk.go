//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"log"

	"github.com/pkg/errors"
)

type WalkAction int

const (
	WalkContinue WalkAction = iota
	WalkSkip
	WalkCancel
)

type WalkFn func(node Operation) WalkAction

func Walk(node Operation, visitor WalkFn) WalkAction {
	act := visitor(node)
	switch act {
	case WalkContinue:
		for _, n := range node.Children() {
			if act = Walk(n, visitor); act == WalkCancel {
				return act
			}
		}
	case WalkSkip:
		act = WalkContinue
	}
	return act
}

func validate(parser Parser) (err error) {
	const OpLimit = 5000000
	count := 0
	Walk(parser.Root, func(node Operation) WalkAction {
		if count++; count == OpLimit {
			err = errors.Errorf("bug or device definition too large: tree traversal exceeded limit of %d nodes", OpLimit)
			return WalkCancel
		}
		switch v := node.(type) {
		case Call:
			if false && v.Target == "" {
				err = errors.Errorf("at %s: call to %s function doesn't have a target", v.Source(), v.Function)
				return WalkCancel
			}
		}
		return WalkContinue
	})
	log.Printf("Validated tree of %d nodes\n", count)
	return
}
