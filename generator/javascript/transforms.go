//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package javascript

import (
	"log"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

var preprocessors = parser.PostprocessGroup{
	Title:   "javascript transforms",
	Actions: []parser.Action{
		{
			Name: "adjust overlapping payload capture",
			Run: adjustOverlappingPayload,
		},
		{
			Name: "adjust field names",
			Run:  adjustFieldNames,
		},
		{
			Name: "set @timestamp",
			Run:  setTimestamp,
		},
	},
}

func adjustFieldNames(p *parser.Parser) (err error) {
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
}

func adjustOverlappingPayload(p *parser.Parser) (err error) {
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
}

func setTimestamp(p *parser.Parser) (err error) {
	timeFields := map[string]int{}
	p.Walk(func(node parser.Operation) (action parser.WalkAction, operation parser.Operation) {
		if datetime, ok := node.(parser.DateTime); ok {
			target := datetime.Target
			if datetime.Target == "" {
				err = errors.Errorf("at %s: no target for EVNTTIME", datetime.Source())
			}
			timeFields[target] += 1
		}
		return parser.WalkContinue, nil
	})
	if err != nil {
		return err
	}
	var selectedField string
	for _, field := range []string{"event_time", "eventtime", "recorded_time", "starttime"} {
		if timeFields[field] > 0 {
			selectedField = field
			break
		}
	}
	if selectedField == "" && len(timeFields) == 1 {
		for k := range timeFields {
			selectedField = k
		}
	}
	if selectedField != "" {
		rootChain := p.Root.(parser.Chain)
		rootChain.Nodes = append(rootChain.Nodes, parser.SetField{
			Target: "@timestamp",
			Value:  [1]parser.Operation{parser.Field(selectedField)},
		})
		p.Root = rootChain
	} else {
		log.Printf("WARN: can't set @timestamp. Fields set by EVNTTIME: %+v", timeFields)
	}
	return nil
}
