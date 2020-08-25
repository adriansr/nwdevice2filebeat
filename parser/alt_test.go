//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAlternatives(t *testing.T) {
	for _, test := range []struct {
		input    string
		expected []interface{}
		err      error
	}{
		{
			input: "one pattern {alternative 1|alternative 2} final",
			expected: []interface{}{
				"one pattern ",
				[]interface{}{
					"alternative 1",
					"alternative 2",
				},
				" final",
			},
		},
		{
			input: "one pattern {alternative 1|alternative 2|3}",
			expected: []interface{}{
				"one pattern ",
				[]interface{}{
					"alternative 1",
					"alternative 2",
					"3",
				},
			},
		},
		{
			input: "one pattern",
			expected: []interface{}{
				"one pattern",
			},
		},
		{
			input: "one {{ pattern",
			expected: []interface{}{
				"one { pattern",
			},
		},
		{
			input: "still one {{ pattern}",
			expected: []interface{}{
				"still one { pattern}",
			},
		},
		{
			input: "broken { a | b",
			expected: []interface{}{
				"broken ",
			},
			err: errSplitAltFailed,
		},
		{
			input: "broken { a |}",
			expected: []interface{}{
				"broken ",
			},
			err: errSplitAltFailed,
		},
	} {
		result, err := splitAlternatives(test.input)
		if test.err == nil {
			assert.NoError(t, err, test.input)
		} else {
			assert.EqualError(t, err, test.err.Error(), test.input)
		}
		assert.Equal(t, test.expected, result, test.input)
	}
}

func TestPatternWithAlternatives(t *testing.T) {
	for _, test := range []struct {
		input    string
		expected Pattern
		err      error
	}{
		{
			input: "one <a> {alternative <c> 1|alternative 2 <d>} <e>",
			expected: Pattern{
				Constant("one "),
				Field{Name: "a"},
				Constant(" "),
				Alternatives{
					Pattern{Constant("alternative "), Field{Name: "c"}, Constant(" 1")},
					Pattern{Constant("alternative 2 "), Field{Name: "d"}},
				},
				Constant(" "), Field{Name: "e"}},
		},
	} {
		result, err := ParsePatternWithAlternatives(test.input)
		if test.err == nil {
			assert.NoError(t, err, test.input)
		} else {
			assert.EqualError(t, err, test.err.Error(), test.input)
		}
		assert.Equal(t, test.expected, result, test.input)
	}
}
