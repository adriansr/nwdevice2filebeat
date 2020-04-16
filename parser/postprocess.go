//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"log"

	"github.com/joeshaw/multierror"
	"github.com/pkg/errors"
)

type Action struct {
	Name string
	Run  func(parser *Parser) error
}

type PostprocessGroup struct {
	Title string
	Actions []Action
}

// These actions check that the parser extracted from the XML is well-formed.
// They run before the parser tree is built.
var prechecks = PostprocessGroup{
	Title: "pre-checks",
	Actions: []Action{
		// It's an error for a message to contain a payload field.
		{"check for payload in messages", checkPayloadInMessages},

		// Consider an error for a HEADER to contain a payload in a position other
		// than last.
		{"check payload position in headers", checkPayloadPositionInHeaders},

		// This checks that if a header uses a payload field (the payload
		// overlaps part of the header), then this field must appear once.
		{ "check overlapped payload fields", checkPayloadOverlap},
	},
}

var preactions = PostprocessGroup{
	Title:   "pre-actions",
	Actions: []Action{
		{ "adjust payload field", setPayloadField},
	},
}

var transforms = PostprocessGroup {
	Title: "transforms",
	Actions: []Action{
		// Replaces a Call() to a MalueMap with a ValueMapCall.
		{"translate valuemap references", convertValueMapReferences},
	},
}

func checkPayloadInMessages(parser *Parser) error {
	for _, msg := range parser.Messages {
		if _, err := msg.content.PayloadField(); err == nil {
			return errors.Errorf("at %s: MESSAGE with payload field", msg.pos)
		}
	}
	return nil
}

func checkPayloadPositionInHeaders(parser *Parser) error {
	for _, hdr := range parser.Headers {
		// Skip headers without a payload.
		if _, err := hdr.content.PayloadField(); err != nil {
			continue
		}
		last := hdr.content[len(hdr.content)-1]
		if _, ok := last.(Payload); !ok {
			return errors.Errorf("at %s: HEADER payload is not the last field", hdr.pos)
		}
	}
	return nil
}

func setPayloadField(parser *Parser) error {
	for _, hdr := range parser.Headers {
		payload, err := hdr.content.PayloadField()
		if err != nil {
			return errors.Wrapf(err, "at %s", hdr.pos)
		}
		if payload != "" {
			hdr.payloadField = payload
		}
		n := len(hdr.content)
		last := hdr.content[n-1]
		if _, ok := last.(Payload); !ok {
			return errors.New("expected payload as last field")
		}
		hdr.content[n-1] = Field("payload")
	}
	return nil
}

func checkPayloadOverlap(parser *Parser) (err error) {
	var payload string
	for _, hdr := range parser.Headers {
		if payload, err = hdr.content.PayloadField(); err != nil {
			return errors.Wrapf(err,"at %s", hdr.pos)
		}
		if payload == "" || payload == "$START" {
			continue
		}
		count := 0
		for _, elem := range hdr.content {
			if fld, ok := elem.(Field); ok && fld.Name() == payload {
				count ++
			}
		}
		if count != 1 {
			return errors.Errorf("at %s: payload field '%s' appears %d times. Expected 1.", hdr.pos, payload, count)
		}
	}
	return nil
}

func convertValueMapReferences(parser *Parser) error {
	var errs multierror.Errors
	parser.Walk(func(node Operation) (WalkAction, Operation) {
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

func (parser *Parser) Apply(group PostprocessGroup) error {
	log.Printf("Running %d %s\n", len(group.Actions), group.Title)
	for _, act := range group.Actions {
		if err := act.Run(parser); err != nil {
			return errors.Wrapf(err, "error applying %s/%s", group.Title, act.Name)
		}
	}
	return nil
}

func validate(parser *Parser) (err error) {
	const OpLimit = 50000000
	count := 0
	parser.Walk(func(node Operation) (WalkAction, Operation) {
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

