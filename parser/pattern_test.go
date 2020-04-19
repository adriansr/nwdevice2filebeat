//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestPattern(t *testing.T) {
	for _, testCase := range []struct {
		input    string
		expected Pattern
		err      error
	}{
		{
			input:    `a<a>`,
			expected: []Value{Constant("a"), Field("a")},
		},
		{
			input: `%ZENPRISEMDM-4: <hdate> <htime>, <hfld1> [<hprocess>] <messageid> <hdate>`,
			expected: []Value{
				Constant("%ZENPRISEMDM-4: "), Field("hdate"),
				Constant(" "), Field("htime"),
				Constant(", "), Field("hfld1"),
				Constant(" ["), Field("hprocess"),
				Constant("] "), Field("messageid"),
				Constant(" "), Field("hdate")},
		},
		{
			input:    `Hello world`,
			expected: []Value{Constant("Hello world")},
		},
		{
			input:    `<Hello><world>`,
			expected: []Value{Field("Hello"), Field("world")},
		},
		{
			input: ``,
			// TODO: Is this what we want?
		},
		{
			input:    ` <field> `,
			expected: []Value{Constant(" "), Field("field"), Constant(" ")},
		},
		{
			input: `<field`,
			err:   errors.New("malformed pattern at position 6 (EOF)"),
		},
		{
			input:    `what about > this thing in here? Is not looking good`,
			expected: Pattern{Constant("what about > this thing in here? Is not looking good")},
		},
		{
			input: `<`,
			err:   errors.New("malformed pattern at position 1 (EOF)"),
		},
		{
			input:    `>`,
			expected: Pattern{Constant(">")},
		},
		{
			input:    `<!payload>`,
			expected: []Value{Payload(Field(""))},
		},
		{
			input:    `<!payload:custom> And this is just <<neat>`,
			expected: []Value{Payload(Field("custom")), Constant(` And this is just <<neat>`)},
		},
		{
			input:    `<dot.fields> are cool too`,
			expected: []Value{Field("dot.fields"), Constant(` are cool too`)},
		},
	} {
		result, err := ParsePattern(testCase.input)
		if testCase.err == nil {
			assert.NoError(t, err, testCase.input)
		} else {
			assert.EqualError(t, err, testCase.err.Error(), testCase.input)
		}
		assert.Equal(t, testCase.expected, result, testCase.input)
	}
}
