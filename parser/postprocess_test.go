//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPostprocessTree(act func(*Parser) error, input Operation) (output Operation, err error) {
	parser := Parser {
		Root: input,
	}
	err = act(&parser)
	return parser.Root, err
}

func treeEquals(t *testing.T, expected , actual Operation) {
	assert.Equal(t, expected, actual)
	//assert.Equal(t, expected.Hashable(), actual.Hashable())
}

func TestFixAlternativesEndInCapture(t *testing.T) {
	for _, test := range []struct {
		title string
		input Operation
		expected Operation
		err error
	} {
		{
			// Both the alternative patterns end in a field capture.
			// The alternative is followed by a constant.
			// Fix by moving the constant into the alternatives.
			title: "move constant into alternative",

			input: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Constant("x"), Field("b"), Constant("c"),
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
					Constant("z"),
					Field("e"),
				},
			},
			expected: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Constant("x"), Field("b"), Constant("c"),
					Alternatives{
						Pattern{Constant("y"), Field("c"), Constant("z"),},
						Pattern{Field("d"), Constant("z"),},
					},
					Field("e"),
				},
			},
		},
		{
			// Both the alternative patterns end in a field capture.
			// The alternative is followed by another field capture.
			// Fix by injecting a space constant into the alternatives.
			title: "inject space into alternatives",

			input: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
					Field("e"),
				},
			},
			expected: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c"), Constant(" "),},
						Pattern{Field("d"), Constant(" "),},
					},
					Field("e"),
				},
			},
		},
		{
			// Alternatives as the last element in a pattern are not an issue.
			title: "ignore final alternative",

			input: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
				},
			},
			expected: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c"),},
						Pattern{Field("d"),},
					},
				},
			},
		},
		{
			title: "mixed alternatives followed by field",
			input: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"),},
						Pattern{Field("d")},
					},
					Field("z"),
				},
			},
			expected: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"),},
						Pattern{Field("d"), Constant(" ")},
					},
					Field("z"),
				},
			},
		},
		{
			title: "mixed alternatives followed by a constant",
			input: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("y"),},
						Pattern{Field("d")},
					},
					Constant("z"),
				},
			},
			expected: Match{
				Input:         "input",
				OnSuccess:	   OpList{ SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern:       Pattern{
					Alternatives{
						Pattern{Constant("yz")},
						Pattern{Field("d"), Constant("z")},
					},
				},
			},
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			result, err := testPostprocessTree(fixAlternativesEndingInCapture, test.input)
			if test.err != nil {
				if assert.Error(t, err) {
					assert.EqualError(t, test.err, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
			treeEquals(t, test.expected, result)
		})
	}
}
