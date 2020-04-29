//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/adriansr/nwdevice2filebeat/util"
)

type Parser struct {
	Config config.Config

	ValueMaps []ValueMap
	Regexs    []Regex
	Headers   []header
	Messages  []message

	ValueMapsByName map[string]*ValueMap
	RegexsByName    map[string]*Regex
	Root            Operation

	warnings *util.Warnings
}

func New(dev model.Device, cfg config.Config, warnings *util.Warnings) (p Parser, err error) {
	p.warnings = warnings
	p.Config = cfg
	if len(dev.TagValMaps) > 0 {
		//return p, errors.Errorf("TAGVALMAP is not implemented (at %s)", dev.TagValMaps[0].Pos())
		warnings.Add(dev.TagValMaps[0].Pos(), "TAGVALMAP is not implemented")
	}

	if len(dev.Regexs) > 0 {
		p.warnings.Add(dev.Regexs[0].Pos(), "unsupported REGX transform: will be ignored.")
	}
	if len(dev.SumDatas) > 0 {
		p.warnings.Add(dev.SumDatas[0].Pos(), "unsupported SUMDATA transform: will be ignored.")
	}
	if len(dev.VarTypes) > 0 {
		p.warnings.Add(dev.VarTypes[0].Pos(), "unsupported VARTYPE transform: will be ignored.")
	}

	if p.ValueMaps, p.ValueMapsByName, err = p.processValueMaps(dev.ValueMaps); err != nil {
		return p, err
	}
	if p.Regexs, p.RegexsByName, err = p.processRegexs(dev.Regexs); err != nil {
		return p, err
	}
	if p.Headers, err = p.processHeaders(dev.Headers); err != nil {
		return p, err
	}
	if p.Messages, err = p.processMessages(dev.Messages); err != nil {
		return p, err
	}
	if err = p.Apply(prechecks); err != nil {
		return p, err
	}
	if err = p.Apply(preactions); err != nil {
		return p, err
	}

	root := Chain{
		SourceContext: SourceContext(dev.Description.Pos()),
	}
	hNodes := make([]Operation, 0, len(p.Headers))
	for idx, h := range p.Headers {
		match := Match{
			SourceContext: SourceContext(h.pos),
			ID:            fmt.Sprintf("HEADER#%d:%s", idx, h.id2),
			Input:         "message",
			Pattern:       h.content,
			PayloadField:  h.payloadField,
		}
		match.OnSuccess = append(match.OnSuccess, SetField{
			SourceContext: match.SourceContext,
			Target:        "header_id",
			Value:         []Operation{Constant(h.id2)},
		})
		if h.messageID != nil {
			match.OnSuccess = append(match.OnSuccess, h.messageID)
		}
		match.OnSuccess = append(match.OnSuccess, h.functions...)
		hNodes = append(hNodes, match)
	}

	msgNode, err := makeMessagesNode(p.Messages)
	if err != nil {
		return p, err
	}
	root.Nodes = append(root.Nodes, LinearSelect{
		SourceContext: SourceContext{},
		Nodes:         hNodes,
	})
	root.Nodes = append(root.Nodes, msgNode)
	p.Root = root
	if err := p.Apply(transforms); err != nil {
		return p, err
	}
	if err := p.Apply(optimizations); err != nil {
		return p, err
	}
	if err := p.Apply(validations); err != nil {
		return p, err
	}
	return p, nil
}

func makeMessagesNode(msgs []message) (Operation, error) {
	var keysInOrder []string
	byID2 := make(map[string][]Operation)
	for idx, msg := range msgs {
		match := Match{
			ID:            fmt.Sprintf("MESSAGE#%d:%s", idx, msg.id1),
			SourceContext: SourceContext(msg.pos),
			Input:         "payload",
			Pattern:       msg.content,
		}
		match.OnSuccess = make([]Operation, 0, 2+len(msg.functions))
		if msg.eventcategory != "" {
			match.OnSuccess = append(match.OnSuccess, SetField{
				SourceContext: match.SourceContext,
				Target:        "eventcategory",
				Value:         []Operation{Constant(msg.eventcategory)},
			})
		}
		// This causes every message to be unique. Outputs must remember to
		// extract this value to deduplicate properly.
		if msg.id1 != "" {
			match.OnSuccess = append(match.OnSuccess, SetField{
				SourceContext: match.SourceContext,
				Target:        "msg_id1",
				Value:         []Operation{Constant(msg.id1)},
			})
		}
		if msg.id2 == "" {
			return nil, errors.Errorf("at %s: MESSAGE has no ID2", msg.pos)
		}
		for _, fn := range msg.functions {
			match.OnSuccess = append(match.OnSuccess, fn)
		}
		if _, exists := byID2[msg.id2]; !exists {
			keysInOrder = append(keysInOrder, msg.id2)
		}
		byID2[msg.id2] = append(byID2[msg.id2], match)
	}
	msgIdSelect := MsgIdSelect{
		Map: make(map[string]int, len(byID2)),
	}
	counter := 0
	for _, k := range keysInOrder {
		list := byID2[k]
		var elem Operation
		if len(list) == 1 {
			elem = list[0]
		} else {
			elem = LinearSelect{
				Nodes: list,
			}
		}
		msgIdSelect.Nodes = append(msgIdSelect.Nodes, elem)
		msgIdSelect.Map[k] = counter
		counter++
	}
	return msgIdSelect, nil
}

func (p *Parser) processValueMaps(input []*model.ValueMap) (output []ValueMap, byName map[string]*ValueMap, err error) {
	byName = make(map[string]*ValueMap, len(input))
	output = make([]ValueMap, len(input))
	for idx, xml := range input {
		vm, err := p.newValueMap(xml)
		if err != nil {
			return output, byName, errors.Wrapf(err, "error parsing VALUEMAP at %s", xml.Pos())
		}
		if byName[vm.Name] != nil {
			return output, byName, errors.Errorf("duplicated VALUEMAP name at %s", xml.Pos())
		}
		output[idx] = vm
		byName[vm.Name] = &output[idx]
	}
	return output, byName, nil
}

func (p *Parser) processRegexs(input []*model.RegX) (output []Regex, byName map[string]*Regex, err error) {
	byName = make(map[string]*Regex, len(input))
	output = make([]Regex, len(input))
	for idx, xml := range input {
		regex := Regex{
			SourceContext: SourceContext(xml.Pos()),
			Name:          xml.Name,
		}
		if byName[regex.Name] != nil {
			return output, byName, errors.Errorf("duplicated REGX name at %s", xml.Pos())
		}
		output[idx] = regex
		byName[regex.Name] = &output[idx]
	}
	return output, byName, nil
}

func (p *Parser) processHeaders(input []*model.Header) (output []header, err error) {
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

func (p *Parser) processMessages(input []*model.Message) (output []message, err error) {
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

var valueMapNullValues = map[string]bool{
	"":      true,
	"$NONE": true,
	"$NULL": true,
}

func (p *Parser) newValueMap(xml *model.ValueMap) (vm ValueMap, err error) {
	vm.SourceContext = SourceContext(xml.Pos())
	if !valueMapNullValues[xml.Default] {
		v, err := newValue(xml.Default, false)
		if err != nil {
			return vm, errors.Wrapf(err, "cannot parse VALUEMAP default value '%s'", xml.Default)
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
			// Ignoring this as only happens once (barracudasf).
			p.warnings.Addf(xml.Pos(), "bad entry in VALUEMAP keyvaluepair: '%s'", pair)
			continue
		}
		value, err := newValue(kv[1], true)
		if err != nil {
			return vm, err
		}
		if prevIdx, found := vm.Mappings[kv[0]]; found {
			prev := vm.Nodes[prevIdx]
			if prev == value {
				p.warnings.Addf(xml.Pos(), "duplicate entry in VALUEMAP: '%s'", pair)
				continue
			}
			// It happens only once (ibmracf). Overwrite duplicates.
			p.warnings.Addf(xml.Pos(), "duplicated VALUEMAP entry '%s' with differing value old:'%s' new:'%s'",
				kv[0], prev, value)
			vm.Nodes[prevIdx] = value
		} else {
			vm.Nodes = append(vm.Nodes, value)
			vm.Mappings[kv[0]] = len(vm.Nodes) - 1
		}
	}
	return vm, nil
}

type header struct {
	pos       util.XMLPos
	id2       string
	messageID Operation
	functions []Operation
	content   Pattern

	// This field is not in the XML. We're adding this information to help
	// capture payload when the payload overlaps part of the header.
	payloadField string
}

func newHeader(xml *model.Header) (h header, err error) {
	// This appears in all the messages.
	if xml.ID2 == "" {
		return h, errors.Errorf("empty id2 attribute")
	}
	if xml.Content == "" {
		return h, errors.Errorf("empty content attribute")
	}
	h = header{
		pos: xml.Pos(),
		id2: xml.ID2,
	}
	if xml.MessageID != "" {
		if h.messageID, err = parseCall(xml.MessageID, false, SourceContext(xml.Pos())); err != nil {
			return h, errors.Wrap(err, "error parsing messageid")
		}
		switch v := h.messageID.(type) {
		case Call:
			v.Target = "messageid"
			h.messageID = v
		case SetField:
			v.Target = "messageid"
			h.messageID = v
		}
	}
	if h.content, err = ParsePatternWithAlternatives(xml.Content); err != nil {
		return h, errors.Wrap(err, "error parsing content")
	}
	if h.functions, err = parseFunctions(xml.Functions, SourceContext(xml.Pos())); err != nil {
		return h, errors.Wrap(err, "error parsing functions")
	}
	return h, err
}

type message struct {
	pos           util.XMLPos
	id1           string
	id2           string
	eventcategory string
	functions     []Operation
	content       Pattern
}

func (m message) String() string {
	return fmt.Sprintf("message={id1='%s', id2='%s', eventcategory='%s' functions='%+v', content=%s}",
		m.id1, m.id2, m.eventcategory, m.functions, m.content.String())
}

func newMessage(xml *model.Message) (m message, err error) {
	// These appear in all the messages.
	if xml.ID1 == "" {
		return m, errors.Errorf("empty ID1 attribute")
	}
	if xml.ID2 == "" {
		return m, errors.New("empty ID2 attribute")
	}
	if xml.Content == "" {
		return m, errors.New("empty content attribute")
	}

	m = message{
		pos:           xml.Pos(),
		id1:           xml.ID1,
		id2:           xml.ID2,
		eventcategory: xml.EventCategory,
	}

	if m.content, err = ParsePatternWithAlternatives(xml.Content); err != nil {
		return m, errors.Wrap(err, "error parsing content")
	}
	if m.functions, err = parseFunctions(xml.Functions, SourceContext(xml.Pos())); err != nil {
		return m, errors.Wrap(err, "error parsing functions")
	}
	return m, err
}

func parseFunctions(s string, loc SourceContext) (calls []Operation, err error) {
	// Skip leading spaces
	i := 0
	for n := len(s); i < n && s[i] == ' '; i++ {
	}
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
	for n := len(s); ; {
		strCall := s[start+1 : end]
		call, err := parseCall(strCall, true, loc)
		if err != nil {
			return nil, errors.Wrapf(err, "can't parse call at %d:%d : '%s'", start, end, strCall)
		}
		calls = append(calls, call)
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

func parseCall(s string, allowTarget bool, location SourceContext) (op Operation, err error) {
	n := len(s)
	var target string
	if allowTarget && s[0] == '@' {
		end := strings.IndexByte(s, ':')
		if end == -1 {
			return op, errors.Errorf("target not terminated at '%s'", s)
		}
		target = s[1:end]
		s = s[end+1:]
		if n = len(s); n == 0 {
			return op, errors.Errorf("bad target pattern at '%s'", s)
		}
		if s[0] != '*' {
			return SetField{
				SourceContext: location,
				Target:        target,
				Value:         []Operation{Constant(s)},
			}, nil
		}
		s = s[1:]
	} else {
		if s[0] == '*' {
			s = s[1:]
		}
	}
	p, err := ParseCall(s)
	if err != nil {
		return op, errors.Wrapf(err, "bad function call at '%s'", s)
	}
	p.SourceContext = location
	p.Target = target
	return p, nil
}
