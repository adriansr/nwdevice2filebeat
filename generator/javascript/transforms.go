//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

var preprocessors = parser.PostprocessGroup{
	Title:   "javascript transforms",
	Actions: []parser.Action{
		{
			Name: "adjust overlapping payload capture",
			Run: func(p *parser.Parser) (err error) {
				p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
					if match, ok := node.(parser.Match); ok && match.PayloadField != "" {
						var pos int
						var elem parser.Value
						found := false
						for pos, elem = range match.Pattern {
							if field, ok := elem.(parser.Field); ok && field.Name() == match.PayloadField {
								found = true
								break
							}
						}
						if !found {
							err = errors.New("payload field not found")
							return parser.WalkCancel, nil
						}
						call := parser.Call{
							SourceContext: match.SourceContext,
							Function:      "STRCAT",
							Target:        "nwparser.payload",
							Args:          match.Pattern[pos:],
						}
						match.OnSuccess = append(match.OnSuccess, call)
						return parser.WalkReplace, match
					}
					return parser.WalkContinue, nil
				})
				return err
			},
		},
		{
			Name: "adjust field names",
			Run: func(p *parser.Parser) (err error) {
				p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
					switch v := node.(type) {
					case parser.Match:
						if v.Input != "message" {
							v.Input = "nwparser." + v.Input
							return parser.WalkReplace, v
						}
					case parser.Call:
						v.Target = "nwparser." + v.Target
						return parser.WalkReplace, v
					case parser.ValueMapCall:
						v.Target = "nwparser." + v.Target
						return parser.WalkReplace, v
					case parser.SetField:
						v.Target = "nwparser." + v.Target
						return parser.WalkReplace, v
					}
					return parser.WalkContinue, nil
				})
				return err
			},

		},
	},
}
