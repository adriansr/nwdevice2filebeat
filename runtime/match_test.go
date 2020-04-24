//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"testing"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/stretchr/testify/assert"
)

type P = parser.Pattern
type C = parser.Constant
type F = parser.Field
type Y = parser.Payload
type A = parser.Alternatives

func s(str string) []byte {
	return []byte(str)
}

func Test_newPattern(t *testing.T) {
	for _, test := range []struct {
		title    string
		input    parser.Pattern
		expected [][]pattern
		wantErr  bool
	}{
		{
			title: "simple pattern",
			input: P{C("hello "), F("f1"), C("! ")},
			expected: [][]pattern{
				{
					{
						element{value: s("hello")},
						element{value: s("f1"), isCapture: true},
						element{value: s("!")},
					},
				},
			},
		},
		{
			title: "pattern with alternatives",
			input: P{C("hello "), F("f1"), C(" : "), A{P{C(" ! "), F("f2"), C(".")}, P{F("f2"), C(".")}}, C("great")},
			expected: [][]pattern{
				{
					{
						element{value: s("hello")},
						element{value: s("f1"), isCapture: true},
						element{value: s(":")},
					},
				},
				{
					{
						element{value: s("!")},
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
					{
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
				},
				{
					{
						element{value: s("great")},
					},
				},
			},
		},
		{
			title: "emtpy alternative branch",
			input: P{C("hello "), A{P{C(" world")}, P{}}, F("rest")},
			expected: [][]pattern{
				{
					{
						element{value: s("hello")},
					},
				},
				{
					{
						element{value: s("world")},
					},
					{},
				},
				{
					{
						element{value: s("rest"), isCapture: true},
					},
				},
			},
		},
		{
			title: "payload at end",
			input: P{C("hello "), F("f1"), C(" : "), A{P{C(" ! "), F("f2"), C(".")}, P{F("f2"), C(".")}}, Y("")},
			expected: [][]pattern{
				{
					{
						element{value: s("hello")},
						element{value: s("f1"), isCapture: true},
						element{value: s(":")},
					},
				},
				{
					{
						element{value: s("!")},
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
					{
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
				},
				{
					{
						element{value: s(""), isCapture: false, isPayload: true},
					},
				},
			},
		},
		{
			title: "payload overlap",
			input: P{C("hello "), F("f1"), C(" "), A{P{C(" ! "), F("f2"), C(".")}, P{F("f2"), C(".")}}, Y("f1")},
			expected: [][]pattern{
				{
					{
						element{value: s("hello")},
						element{value: s("f1"), isCapture: true, isPayload: true},
						element{value: s(" ")},
					},
				},
				{
					{
						element{value: s("!")},
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
					{
						element{value: s("f2"), isCapture: true},
						element{value: s(".")},
					},
				},
			},
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			gotOutput, err := newPattern(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("newPattern() error = %v, wantErr %v", err, test.wantErr)
			}
			assert.Equal(t, test.expected, gotOutput)
		})
	}
}

func Test_capture(t *testing.T) {
	for _, test := range []struct {
		title    string
		pattern  parser.Pattern
		message  string
		expected Context
		err      bool
	}{
		{
			title:   "simple pattern",
			pattern: P{C("hello "), F("f1"), C("! ")},
			message: "hello world!",
			expected: Context{
				Message: s(""),
				Fields: Fields{
					"f1": "world",
				},
			},
		},
		{
			title:   "capture first and last",
			pattern: P{F("leading"), C("hello "), F("f1"), C("! "), F("last")},
			message: "Well, hello neighbour ! How are you",
			expected: Context{
				Message: s(""),
				Fields: Fields{
					"leading": "Well,",
					"f1":      "neighbour",
					"last":    "How are you",
				},
			},
		},
		{
			title:   "trailing payload",
			pattern: P{F("leading"), C("hello "), F("f1"), C("! "), Y("")},
			message: "Well, hello neighbour ! How are you",
			expected: Context{
				Message: s(" How are you"),
				Fields: Fields{
					"leading": "Well,",
					"f1":      "neighbour",
				},
			},
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			pattern, err := newPattern(test.pattern)
			if err != nil {
				t.Fatal(err)
			}
			m := match{
				pattern: pattern,
			}
			ctx := Context{
				Message: []byte(test.message),
			}
			err = m.Run(&ctx)
			if !test.err {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, ctx)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
