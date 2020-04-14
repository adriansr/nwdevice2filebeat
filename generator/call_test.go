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
			input: "STRCAT('CISCOIPORTESA','_','GENERIC')",
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
			input: "STRCAT('header_' , id2)",
			expected: Call {
				Function: "STRCAT",
				Args: []Value{
					Constant("header_"),
					Field("id2"),
				},
			},
		},
		{
			input: "STRCAT('header_' , id2)",
			expected: Call {
				Function: "STRCAT",
				Args: []Value{
					Constant("header_"),
					Field("id2"),
				},
			},
		},
		{
			input: `PARMVAL($MSG)`,
			expected: Call {
				Function: "PARMVAL",
				Args: []Value {
					Field("$MSG"),
				},
			},
		},
		{
			input: `PARMVAL($MSG)`,
			expected: Call {
				Function: "PARMVAL",
				Args: []Value {
					Field("$MSG"),
				},
			},
		},
		{
			input: `MyCall($HDR,'%G/%F/%W %H:%U:%O',hdate1,htime)`,
			expected: Call {
				Function: "MyCall",
				Args: []Value {
					Field("$HDR"),
					Constant(`%G/%F/%W %H:%U:%O`),
					Field("hdate1"),
					Field("htime"),
				},
			},
		},
		{
			input: `ESCAPED('here\'s a quote', 'and a \\ slash') `,
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
			input: "  MY_FUN(   field ) ",
			expected: Call {
				Function: "MY_FUN",
				Args: []Value{
					Field("field"),
				},
			},
		},
		{
			input: "PERFECTLY_VALID()",
			expected:Call{
				Function: "PERFECTLY_VALID",
			},
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
			input: `THIS('is not terminated`,
			err: true,
		},
		{
			input: `NEITHER(`,
			err: true,
		},
		{
			input: "UNQUOTED('this is fine', at some point someone got tired of quotes, \t, my.field)",
			expected: Call{
				Function: "UNQUOTED",
				Args:     []Value{
					Constant("this is fine"),
					Constant("at some point someone got tired of quotes"),
					Constant("\t"),
					Field("my.field"),
				},
			},
		},
	} {
		result, err := ParseCall(testCase.input)
		if !testCase.err {
			assert.NoError(t, err)
			assert.Equal(t, &testCase.expected, result, testCase.input)
		} else {
			assert.Equal(t, ErrBadCall, err, testCase.input)
		}
	}
}


func TestCall2(t *testing.T) {
	for _, testCase := range []struct {
		input    string
		expected Call
		err      bool
	}{
		{
			input: "@target:*APPEND('hola',:,feo)",
			expected: Call{
				Function: "APPEND",
				Target:   "target",
				Args:     []Value{Constant("hola"), Constant(":"), Field("feo")},
			},
		},
		{
			input: "@target:SOMETHING()",
			expected: Call{
				Function: "$set$",
				Target:   "target",
				Args:     []Value{Constant("SOMETHING()")},
			},
		},
	}{
		result, err := parseCall(testCase.input, true)
		if !testCase.err {
			assert.NoError(t, err)
			assert.Equal(t, &testCase.expected, result, testCase.input)
		} else {
			assert.Equal(t, ErrBadCall, err, testCase.input)
		}
	}
}
