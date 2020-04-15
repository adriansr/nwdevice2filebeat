//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/pkg/errors"
)

type Parser struct {
	ValueMaps []ValueMap
	Headers   []header
	Messages  []message

	Root Operation
}

func New(dev model.Device) (p Parser, err error) {
	if p.ValueMaps, err = processValueMaps(dev.ValueMaps); err != nil {
		return p, err
	}
	p.Headers, err = processHeaders(dev.Headers)
	if err != nil {
		return p, err
	}
	if p.Messages, err = processMessages(dev.Messages); err != nil {
		return p, err
	}

	root := Chain {
		SourceContext: SourceContext(dev.Description.Pos()),
	}

	hNodes := make([]Operation, 0, len(p.Headers))
	for _, h := range p.Headers {
		match := Match{
			SourceContext: SourceContext(h.pos),
			Input:         "message",
			Pattern:       h.content,
		}
		match.OnSuccess = make([]Operation, 0, 1+len(h.functions))
		if h.messageID != nil {
			match.OnSuccess = append(match.OnSuccess, h.messageID)
		}
		for _, fn := range h.functions {
			match.OnSuccess = append(match.OnSuccess, fn)
		}
		hNodes = append(hNodes, match)
	}

	mNodes := make([]Operation, 0, len(p.Messages))
	for _, m := range p.Messages {
		match := Match{
			SourceContext: SourceContext(m.pos),
			Input:         "message", // TODO
			Pattern:       m.content,
		}
		match.OnSuccess = make([]Operation, 0, 1+len(m.functions))
		if m.eventcategory != "" {
			match.OnSuccess = append(match.OnSuccess, Constant(m.eventcategory))
		}
		for _, fn := range m.functions {
			match.OnSuccess = append(match.OnSuccess, fn)
		}
		mNodes = append(mNodes, match)
	}
	root.Nodes = append(root.Nodes, LinearSelect{
		SourceContext: SourceContext{},
		Nodes:         hNodes,
	})
	root.Nodes = append(root.Nodes, LinearSelect{
		SourceContext: SourceContext{},
		Nodes:         mNodes,
	})
	p.Root = root
	return p, validate(p)
}

func processValueMaps(input []*model.ValueMap) (output []ValueMap, err error) {
	seen := make(map[string]bool, len(input))
	for _, xml := range input {
		vm, err := newValueMap(xml)
		if err != nil {
			return output, errors.Wrapf(err, "error parsing VALUEMAP at %s", xml.Pos())
		}
		if seen[vm.Name] {
			return output, errors.Errorf("duplicated VALUEMAP name at %s", xml.Pos())
		}
		seen[vm.Name] = true
		output = append(output, vm)
	}
	return output, nil
}

func processHeaders(input []*model.Header) (output []header, err error) {
	output = make([]header, len(input))
	for idx, xml := range input {
		vm, err := newHeader(xml)
		if err != nil {
			return output, errors.Wrapf(err, "error parsing HEADER at %s", xml.Pos())
		}
		output[idx] = vm
	}
	return output, nil
}

func processMessages(input []*model.Message) (output []message, err error) {
	output = make([]message, len(input))
	for idx, xml := range input {
		vm, err := newMessage(xml)
		if err != nil {
			return output, errors.Wrapf(err, "error parsing MESSAGE at %s", xml.Pos())
		}
		output[idx] = vm
	}
	return output, nil
}

var valueMapNullValues = map[string]bool {
	"": true,
	"$NONE": true,
	"$NULL": true,
}

func newValueMap(xml *model.ValueMap) (vm ValueMap, err error) {
	vm.SourceContext = SourceContext(xml.Pos())
	if !valueMapNullValues[xml.Default] {
		v, err := newValue(xml.Default, false)
		if err != nil {
			return vm, errors.Wrapf(err,"cannot parse VALUEMAP default value '%s'", xml.Default)
		}
		vm.Default = &v
	}
	vm.Name = xml.Name
	kvpairs := strings.Split(xml.KeyValuePairs, "|")
	vm.Nodes = make([]Operation, 0, len(kvpairs))
	vm.Mappings = make(map[string]int, len(kvpairs))
	for _, pair := range kvpairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			// TODO: Warnings
			//
			// Ignoring this as only happens once (barracudasf).
			log.Printf("WARN: parsing HEADER at %s: bad entry in keyvaluepair: '%s'", xml.Pos(), pair)
			continue
			//return vm, errors.New("failed parsing keyvaluepairs")
		}
		value, err := newValue(kv[1], true)
		if err != nil {
			return vm, err
		}
		if prevIdx, found := vm.Mappings[kv[0]]; found {
			prev := vm.Nodes[prevIdx]
			if prev == value {
				log.Printf("WARN: parsing VALUEMAP at %s: duplicated keyvaluepair entry: '%s'", xml.Pos(), pair)
				continue
			}
			// TODO:
			// What to do here. It happens only once (ibmracf)
			log.Printf("WARN: parsing VALUEMAP at %s: found duplicated key '%s' with differing value old:%s new:%s", xml.Pos(), kv[0], prev, value)
			vm.Nodes[prevIdx] = value
		} else {
			vm.Nodes = append(vm.Nodes, value)
			vm.Mappings[kv[0]] = len(vm.Nodes) - 1
		}
	}
	return vm, nil
}

type header struct {
	pos model.XMLPos
	id2 string
	messageID *Call
	functions []Call
	content Pattern
}

func newHeader(xml *model.Header) (h header, err error) {
	h.pos = xml.Pos()
	// This appears in all the messages.
	if xml.ID2 == "" {
		return h, errors.Errorf("empty id2 attribute")
	}
	if xml.Content == "" {
		return h, errors.Errorf("empty content attribute")
	}
	h = header {
		id2: xml.ID2,
	}
	if xml.MessageID != "" {
		if h.messageID, err = parseCall(xml.MessageID, false); err != nil {
			return h, errors.Wrap(err,"error parsing messageid")
		}
		//log.Printf("XXX at %s: messageid=<<%s>>", xml.Pos(), h.messageID)
	}
	if h.content, err = ParsePattern(xml.Content); err != nil {
		return h, errors.Wrap(err,"error parsing content")
	}
	if h.functions, err = parseFunctions(xml.Functions); err != nil {
		return h, errors.Wrap(err,"error parsing functions")
	}
	return h, err
}

type message struct {
	pos model.XMLPos
	id1 string
	id2 string
	eventcategory string
	functions []Call
	content Pattern
}

func (m message) String() string {
	return fmt.Sprintf("message={id1='%s', id2='%s', eventcategory='%s' functions='%+v', content=%s}",
		m.id1, m.id2, m.eventcategory, m.functions, m.content.String())
}

func newMessage(xml *model.Message) (m message, err error) {
	// This appears in all the messages.
	if xml.ID1 == "" {
		return m, errors.Errorf("empty ID1 attribute")
	}
	if xml.ID2 == "" {
		return m, errors.New("empty ID2 attribute")
	}
	if xml.Content == "" {
		return m, errors.New("empty content attribute")
	}
	//if xml.Functions == "" {
	//	return m, errors.Errorf("no functions in MESSAGE at %s", xml.Pos())
	//}
	m = message {
		id1: xml.ID1,
		id2: xml.ID2,
		eventcategory: xml.EventCategory,
	}

	if m.content, err = ParsePattern(xml.Content); err != nil {
		return m, errors.Wrap(err,"error parsing content")
	}
	if m.functions, err = parseFunctions(xml.Functions); err != nil {
		return m, errors.Wrap(err, "error parsing functions")
	}
	//log.Printf("XXX at %s: got %+v", xml.Pos(), m)
	return m, err
}

func parseFunctions(s string) (calls []Call, err error) {
	// Skip leading spaces
	i := 0
	for n := len(s);i<n && s[i] == ' '; i++ {}
	s = s[i:]
	if s == "" {
		return nil, nil
	}
	if n := len(s); n < 2 || s[0] != '<' {
		if n > 20 {
			n = 20
		}
		return nil, errors.Errorf("pattern start error at '%s'", s[:n])
	}
	start := 0
	end := strings.IndexByte(s, '>')
	if end == -1 {
		return nil, errors.New("no closing brace")
	}
	for n := len(s);; {
		strCall := s[start+1:end]
		call, err := parseCall(strCall, true)
		if err != nil {
			return nil, errors.Wrapf(err,"can't parse call at %d:%d : '%s'", start, end, strCall)
		}
		calls = append(calls, *call)
		for start = end + 1; start < n && s[start] == ' '; start++ {
		}
		if start >= n {
			break
		}
		if s[start] != '<' {
			return nil, errors.Errorf("no opening brace at '%s'", s[start:])
		}
		end = strings.IndexByte(s[start:], '>')
		if end == -1 {
			return nil, errors.Errorf("no closing brace at '%s'", s[start:])
		}
		end += start
	}
	return calls, nil
}

func parseCall(s string, allowTarget bool) (call *Call, err error) {
	call = &Call {}
	n := len(s)
	if allowTarget && s[0] == '@' {
		end := strings.IndexByte(s, ':')
		if end == -1 {
			return call, errors.Errorf("target not terminated at '%s'", s)
		}
		call.Target = s[1:end]
		s = s[end+1:]
		if n = len(s); n == 0 {
			return call, errors.Errorf("bad target pattern at '%s'", s)
		}
		if s[0] != '*' {
			call.Function = "$set$"
			call.Args = []Value{ Constant(s) }
			return call, nil
		}
		s = s[1:]
	} else {
		if s[0] == '*' {
			s = s[1:]
		}
	}
	p, err := ParseCall(s)
	if err != nil {
		return call, errors.Wrapf(err,"bad function call at '%s'", s)
	}
	call.Function = p.Function
	call.Args = p.Args
	return call, nil
}
