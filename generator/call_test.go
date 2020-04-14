//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCall(t *testing.T) {
	for _, testCase := range []struct {
		input string
		expected Call
		err bool
	} {
		{
			input: "*STRCAT('CISCOIPORTESA','_','GENERIC')",
			expected: Call {
				Function: "STRCAT",
				Args: []Value{
					Constant("CISCOIPORTESA"),
					Constant("_"),
					Constant("GENERIC"),
				},
			},
		},
		{
			input: "*STRCAT('header_' , id2)",
			expected: Call {
				Function: "STRCAT",
				Args: []Value{
					Constant("header_"),
					Field("id2"),
				},
			},
		},
		{
			input: `*ESCAPED('here\'s a quote', 'and a \\ slash') `,
			expected: Call {
				Function: "ESCAPED",
				Args: []Value{
					Constant("here's a quote"),
					Constant(`and a \ slash`),
				},
			},
		},
		{
			input: "MY_FUN(field)",
			expected: Call {
				Function: "MY_FUN",
				Args: []Value{
					Field("field"),
				},
			},
		},
		{
			input: "  *MY_FUN  (   \tfield ) ",
			expected: Call {
				Function: "MY_FUN",
				Args: []Value{
					Field("field"),
				},
			},
		},
		{
			input: "INVALID()",
			err: true,
		},
		{
			input: "INVALID ",
			err: true,
		},
		{
			input: "INVALID (what is this)",
			err: true,
		},
		{
			input: "ALSO INVALID",
			err: true,
		},
		{
			input: `*THIS('is\'just'plain'wrong')`,
			err: true,
		},
		{
			input: `*THIS('is not terminated`,
			err: true,
		},
		{
			input: `NEITHER(`,
			err: true,
		},
	} {
		result, err := ParseCall(testCase.input)
		if !testCase.err {
			assert.NoError(t, err)
			assert.Equal(t, &testCase.expected, result, testCase.input)
		} else {
			assert.Equal(t, ErrBadCall, err)
		}
	}
}
