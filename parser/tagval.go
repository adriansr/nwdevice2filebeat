//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/model"
)

// TagValMapSettings holds information about the special characters used to
// parse key-value messages.
type TagValMapSettings struct {

	// PairSeparator is the string between key-value pairs.
	PairSeparator string

	// KeyValueSeparator is the characters that mark the end of a key and
	// start of a value.
	KeyValueSeparator string

	// OpenQuote is the character used at the start of a quoted string.
	OpenQuote string

	// CloseQuote is the character used at the end of a quoted string.
	CloseQuote string

	// KeyValueEscape is the character that escapes the characters in
	// KeyValueSeparator so that they can appear in a value without terminating
	// it.
	// Or not, not sure at all how this works.
	//KeyValueEscape string
}

// TODO: Completely unsure about this defaults
var defaults = TagValMapSettings{
	KeyValueSeparator: "=",
	//PairSeparator:     " ",
	//OpenQuote:         "",
	//CloseQuote:        "",
	//KeyValueEscape:    "\\",
}

func (p *Parser) processTagValMap(input []*model.TagValMap) (output *TagValMapSettings, err error) {
	switch len(input) {
	case 0:
		return
	case 1:
	default:
		return nil, fmt.Errorf("at %s: more than one TAGVALMAP defined",
			input[1].Pos())
	}
	def := *input[0]
	old, err := loadOldSettings(def)
	if err != nil {
		return nil, err
	}
	new, err := loadNewSettings(def)
	if err != nil {
		return nil, err
	}
	settings, ok := merge(old, new, defaults)
	if !ok {
		return nil, fmt.Errorf("at %s: Can't parse TAGVALMAP: conflicting delimiter values", def.Pos())
	}
	return &settings, err
}

var delimiterRegex = regexp.MustCompile("(.[^ ]*) ")

// loadOldSettings populates TAGVALMAP configuration from the undocumented
// "delimiter" field. The observed format is:
// <pair_separator> [ <space>? <quote_char> ]
// - pair_separator: single character (can be a space)
// - quote_char: single character, never a space.
func loadOldSettings(tvm model.TagValMap) (settings TagValMapSettings, err error) {
	var msg string
	delim := tvm.Delimiter
	n := len(delim)
	switch n {
	case 0: // Not set
		return
	case 1: // Just one separator
		return TagValMapSettings{PairSeparator: delim}, nil
	case 2: // pair separator + quote char
	case 3: // pair separator + space + quote char
		if delim[1] != ' ' {
			msg = "expected a space separator"
		}
	default:
		msg = "too many characters"
	}
	if len(msg) > 0 {
		return settings, fmt.Errorf("at %s: TAGVALMAP delimiter field not understood: %s, got \"%s\"",
			tvm.Pos(), msg, delim)
	}
	return TagValMapSettings{
		PairSeparator: string(delim[0]),
		OpenQuote:     string(delim[n-1]),
		CloseQuote:    string(delim[n-1]),
	}, nil
}

// loadNewSettings populates TAGVALMAP configuration from the documented fields.
func loadNewSettings(tvm model.TagValMap) (settings TagValMapSettings, err error) {
	settings = TagValMapSettings{
		PairSeparator:     tvm.PairDelimiter,
		KeyValueSeparator: tvm.ValueDelimiter,
		//KeyValueEscape:    tvm.EscapeValueDelimt,
	}
	switch len(tvm.Encapsulator) {
	case 0:
	case 1:
		settings.OpenQuote = tvm.Encapsulator
		settings.CloseQuote = tvm.Encapsulator
	case 2:
		settings.OpenQuote = string(tvm.Encapsulator[0])
		settings.CloseQuote = string(tvm.Encapsulator[1])
	default:
		return settings, fmt.Errorf("at %s: TAGVALMAP encapsulator field not understood: expected one or two characters, got '%s'",
			tvm.Pos(), tvm.Encapsulator)
	}
	return settings, nil
}

func mergeValue(old string, new string, def string) (string, bool) {
	oldSet := old != ""
	newSet := new != ""
	if !oldSet && !newSet {
		return def, true
	}
	if oldSet && newSet {
		if old == new {
			return old, true
		}
		return "", false
	}
	if oldSet {
		return old, true
	}
	return new, true
}

func merge(old, new, def TagValMapSettings) (out TagValMapSettings, ok bool) {
	oldV := reflect.ValueOf(old)
	newV := reflect.ValueOf(new)
	defV := reflect.ValueOf(def)
	outV := reflect.ValueOf(&out)
	for i := 0; i < oldV.NumField(); i++ {
		str, ok := mergeValue(
			oldV.Field(i).String(),
			newV.Field(i).String(),
			defV.Field(i).String())
		if !ok {
			return out, false
		}
		outV.Elem().Field(i).SetString(str)
	}
	return out, true
}

type TagValues struct {
	Map    map[string]string
	Config TagValMapSettings
}

func newTagValues(pattern Pattern, set TagValMapSettings) (tvm TagValues, err error) {
	tvm = TagValues{
		Map:    make(map[string]string, len(pattern)/2),
		Config: set,
	}
	for idx := 0; idx+1 < len(pattern); idx += 2 {
		ct, ok1 := pattern[idx].(Constant)
		fld, ok2 := pattern[idx+1].(Field)
		if !ok1 || !ok2 {
			return tvm, fmt.Errorf("pattern is not a sequence of literals and captures (position %d)", idx)
		}
		// Expressions frequently have extra spaces, sometimes a semicolon:
		key := strings.Trim(ct.Value(), " ;")
		// Remove separators
		key = strings.TrimLeft(key, set.PairSeparator)
		key = strings.TrimRight(key, set.KeyValueSeparator)
		if strings.Contains(key, set.KeyValueSeparator) {
			return tvm, fmt.Errorf("tagval message key '%s' contains the key-value separator '%s'",
				key, set.KeyValueSeparator)
		}
		if strings.Contains(key, set.PairSeparator) {
			return tvm, fmt.Errorf("tagval message key '%s' contains the pair separator '%s'",
				key, set.PairSeparator)
		}

		// Remove any spacing after separators
		key = strings.Trim(key, " ")
		if prev, found := tvm.Map[key]; found && prev != fld.Name {
			log.Printf("WARN: TAGVALMAP has duplicated key '%s' with different field capture. prev:%s new:%s",
				key, prev, fld.Name)
		}
		tvm.Map[key] = fld.Name
	}
	return tvm, nil
}

func (tv TagValues) IsSet() bool {
	return len(tv.Map) > 0
}
