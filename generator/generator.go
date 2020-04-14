//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import (
	"fmt"
	"log"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/pkg/errors"
)

type Generator struct {
	valueMaps map[string]valueMap
	headers   []header
	messages  []message
}

func New(dev model.Device) (gen Generator, err error) {
	if gen.valueMaps, err = processValueMaps(dev.ValueMaps); err != nil {
		return gen, err
	}
	if gen.headers, err = processHeaders(dev.Headers); err != nil {
		return gen, err
	}
	if gen.messages, err = processMessages(dev.Messages); err != nil {
		return gen, err
	}
	return gen, nil
}

func processValueMaps(input []*model.ValueMap) (output map[string]valueMap, err error) {
	output = make(map[string]valueMap, len(input))
	for _, xml := range input {
		vm, err := newValueMap(xml)
		if err != nil {
			return output, errors.Wrapf(err, "error parsing VALUEMAP at %s", xml.Pos())
		}
		if _, found := output[vm.name]; found {
			return output, errors.Errorf("duplicated VALUEMAP name at %s", xml.Pos())
		}
		output[vm.name] = vm
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

// TODO: Sometimes keys are numeric (and hex!) should it support numeric keys
//       in different base? As in 33 for 0x21
// TODO: Values are either quoted (single) or refs to fields (*dport)
type valueMap struct {
	name string
	def  string
	mappings map[string]Value
}

func newValueMap(xml *model.ValueMap) (vm valueMap, err error) {
	if xml.Default != "$NONE" {
		vm.def = xml.Default
	}
	vm.name = xml.Name
	kvpairs := strings.Split(xml.KeyValuePairs, "|")
	vm.mappings = make(map[string]Value, len(kvpairs))
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
		value, err := newValue(kv[1])
		if err != nil {
			return vm, err
		}
		if prev, found := vm.mappings[kv[0]]; found {
			if prev == value {
				log.Printf("WARN: parsing VALUEMAP at %s: duplicated keyvaluepair entry: '%s'", xml.Pos(), pair)
				continue
			}
			// TODO:
			// What to do here. It happens only once (ibmracf)
			return vm, errors.Errorf("found duplicated key '%s' with differing value old:%s new:%s", kv[0], prev, value)
		}
		vm.mappings[kv[0]] = value
	}
	return vm, nil
}

type header struct {
	id2 string
	messageID *Call
	content Pattern
}

func newHeader(xml *model.Header) (h header, err error) {
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
	//log.Printf("XXX at %s: got pattern=<<%s>>", xml.Pos(), h.content)
	// TODO: functions etc.
	return h, err
}

type message struct {
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
	log.Printf("XXX at %s: got %+v", xml.Pos(), m)
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
