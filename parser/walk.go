//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

type WalkAction int

const (
	WalkContinue WalkAction = iota
	WalkSkip
	WalkCancel
	WalkReplace
)

type WalkFn func(node Operation) (WalkAction, Operation)

func (parser *Parser) Walk(visitor WalkFn) WalkAction {
	return walk(&parser.Root, visitor)
}

func walk(ref *Operation, visitor WalkFn) WalkAction {
	act, repl := visitor(*ref)
	switch act {
	case WalkReplace:
		*ref = repl
		fallthrough
	case WalkContinue:
		nodes := (*ref).Children()
		for idx := range nodes {
			if act = walk(&nodes[idx], visitor); act == WalkCancel {
				return act
			}
		}
	case WalkSkip:
		act = WalkContinue

	}
	return act
}
