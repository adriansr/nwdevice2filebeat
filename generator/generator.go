//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import (
	"log"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/model"
	"github.com/pkg/errors"
)

type Generator struct {
	valueMaps map[string]valueMap
	headers   []header
}

func New(dev model.Device) (gen Generator, err error) {
	if gen.valueMaps, err = processValueMaps(dev.ValueMaps); err != nil {
		return gen, err
	}
	if gen.headers, err = processHeaders(dev.Headers); err != nil {
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
		return h, errors.Errorf("no id2 in HEADER at %s", xml.Pos())
	}
	if xml.Content == "" {
		return h, errors.Errorf("no content in HEADER at %s", xml.Pos())
	}
	h = header {
		id2: xml.ID2,
	}
	if xml.MessageID != "" {
		if h.messageID, err = ParseCall(xml.MessageID); err != nil {
			return h, errors.Wrapf(err,"error parsing messageid from HEADER at %s", xml.Pos())
		}
		//log.Printf("XXX at %s: messageid=<<%s>>", xml.Pos(), h.messageID)
	}
	if h.content, err = ParsePattern(xml.Content); err != nil {
		return h, errors.Wrapf(err,"error parsing content from HEADER at %s", xml.Pos())
	}
	// TODO: functions etc.
	return h, err
}
