//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logs

import (
	"testing"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/stretchr/testify/assert"
)

func f(name string) parser.Field {
	return parser.Field{Name: name}
}
func c(value string) parser.Constant {
	return parser.Constant(value)
}

func Test_strcatSplit(t *testing.T) {
	for _, test := range []struct {
		name     string
		expr     []parser.Value
		str      string
		expected map[string]string
	}{
		{
			name: "f-c-f",
			expr: []parser.Value{f("fld1"), c("_"), f("fld2")},
			str:  "MESSAGE_ID",
			expected: map[string]string{
				"fld1": "MESSAGE",
				"fld2": "ID",
			},
		},
		{
			name: "f",
			expr: []parser.Value{f("fld1")},
			str:  "MESSAGE_ID",
			expected: map[string]string{
				"fld1": "MESSAGE_ID",
			},
		},
		{
			name: "c f c",
			expr: []parser.Value{c("***"), f("fld1"), c("***")},
			str:  "***MESSAGE_ID***",
			expected: map[string]string{
				"fld1": "MESSAGE_ID",
			},
		},
		{
			name: "f f c",
			expr: []parser.Value{f("fld1"), f("fld2"), c("_ID")},
			str:  "MESSAGE_ID",
			expected: map[string]string{
				"fld1": "MESS",
				"fld2": "AGE",
			},
		},
		{
			name: "f f",
			expr: []parser.Value{f("fld1"), f("fld2")},
			str:  "MESSAGE_ID",
			expected: map[string]string{
				"fld1": "MESSA",
				"fld2": "GE_ID",
			},
		},
		{
			name: "complicated",
			expr: []parser.Value{f("fld1"), f("fld2"), c("_")},
			str:  "SPECIAL_MESSAGE_",
			expected: map[string]string{
				"fld1": "SPECIAL_",
				"fld2": "MESSAGE",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := strcat(test.expr).Split(test.str)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestMapKey_Filter(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    []string
		filter   []parser.Value
		expected []string
	}{
		{
			name: "sample",
			input: []string{
				"ABC",
				"D_E_F",
				"G_H_I_",
				"JK",
				"",
				"LMN_",
				"_PQ",
				"_",
			},
			filter: []parser.Value{f("1"), c("_"), f("2")},
			expected: []string{
				"D_E_F",
				"G_H_I_",
			},
		},
		{
			name: "sample",
			input: []string{
				"ABC",
				"D_E_F",
				"G_H_I_",
				"JK",
				"",
				"LMN_",
				"_PQ",
				"_",
			},
			filter: []parser.Value{f("1")},
			expected: []string{
				"ABC",
				"D_E_F",
				"G_H_I_",
				"JK",
				"LMN_",
				"_PQ",
				"_",
			},
		},
		{
			name: "sample",
			input: []string{
				"ABC",
				"D_E_F",
				"G_H_I_",
				"JK",
				"",
				"LMN_",
				"_PQ",
				"_",
			},
			filter: []parser.Value{f("1"), c("_")},
			expected: []string{
				"G_H_I_",
				"LMN_",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := mapKey(test.input).Filter(test.filter)
			assert.Equal(t, test.expected, []string(result))
		})
	}
}

func TestMergeOverlapped(t *testing.T) {
	for _, test := range []struct {
		title    string
		header   parser.Pattern
		message  parser.Pattern
		expected map[string]string
		ok       bool
	}{
		{
			title:   "Easy",
			header:  parser.Pattern{c(": udp connection")},
			message: parser.Pattern{c(": "), f("protocol"), c(" "), f("type")},
			expected: map[string]string{
				"protocol": "udp",
				"type":     "connection",
			},
			ok: true,
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			result, ok := mergeOverlapped(test.header, test.message)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.expected, result)
		})
	}
}
