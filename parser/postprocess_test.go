//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPostprocessTree(act func(*Parser) error, input Operation) (output Operation, err error) {
	parser := Parser{
		Root: input,
	}
	err = act(&parser)
	return parser.Root, err
}

func treeEquals(t *testing.T, expected, actual Operation) {
	assert.Equal(t, expected, actual)
	//assert.Equal(t, expected.Hashable(), actual.Hashable())
}

type actionTestCase struct {
	title    string
	input    Operation
	expected Operation
	err      error
}

func testAction(t *testing.T, act func(*Parser) error, cases []actionTestCase) {
	for _, test := range cases {
		t.Run(test.title, func(t *testing.T) {
			result, err := testPostprocessTree(act, test.input)
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

func TestFixAlternativesEndInCapture(t *testing.T) {
	testAction(t, fixAlternativesEndingInCapture, []actionTestCase{
		{
			// Both the alternative patterns end in a field capture.
			// The alternative is followed by a constant.
			// Fix by moving the constant into the alternatives.
			title: "move constant into alternative",

			input: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
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
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Constant("x"), Field("b"), Constant("c"),
					Alternatives{
						Pattern{Constant("y"), Field("c"), Constant("z")},
						Pattern{Field("d"), Constant("z")},
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
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
					Field("e"),
				},
			},
			expected: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c"), Constant(" ")},
						Pattern{Field("d"), Constant(" ")},
					},
					Field("e"),
				},
			},
		},
		{
			// Alternatives as the last element in a pattern are not an issue.
			title: "ignore final alternative",

			input: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
				},
			},
			expected: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y"), Field("c")},
						Pattern{Field("d")},
					},
				},
			},
		},
		{
			title: "mixed alternatives followed by field",
			input: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y")},
						Pattern{Field("d")},
					},
					Field("z"),
				},
			},
			expected: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y")},
						Pattern{Field("d"), Constant(" ")},
					},
					Field("z"),
				},
			},
		},
		{
			title: "mixed alternatives followed by a constant",
			input: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("y")},
						Pattern{Field("d")},
					},
					Constant("z"),
				},
			},
			expected: Match{
				Input:     "input",
				OnSuccess: OpList{SetField{Target: "a", Value: []Operation{Constant("b")}}},
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("yz")},
						Pattern{Field("d"), Constant("z")},
					},
				},
			},
		},
	})
}

func TestExtractLeadingConstantPrefix(t *testing.T) {
	for _, test := range []struct {
		input    Alternatives
		expected Alternatives
		prefix   string
	}{
		{
			input: Alternatives{
				Pattern{Constant("Hello world")},
				Pattern{Constant("Hell is "), Field("f")},
				Pattern{Constant("Helium")},
				Pattern{Constant("Hel"), Field("z")},
				Pattern{Constant("Hel")},
			},
			expected: Alternatives{
				Pattern{Constant("lo world")},
				Pattern{Constant("l is "), Field("f")},
				Pattern{Constant("ium")},
				Pattern{Field("z")},
			},
			prefix: "Hel",
		},
		{
			input: Alternatives{
				Pattern{Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad.")},
			},
			expected: Alternatives{},
			prefix:   "Repetition is bad.",
		},
		{
			input: Alternatives{
				Pattern{Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad."), Field("isit")},
				Pattern{Constant("Repetition is bad.")},
			},
			expected: Alternatives{
				Pattern{Field("isit")},
			},
			prefix: "Repetition is bad.",
		},
	} {
		prefix, result := extractLeadingConstantPrefix(test.input)
		assert.Equal(t, test.prefix, prefix)
		assert.Equal(t, test.expected, result)
	}
}

func TestExtractTrailingConstantPrefix(t *testing.T) {
	for _, test := range []struct {
		input    Alternatives
		expected Alternatives
		prefix   string
	}{
		{
			input: Alternatives{
				Pattern{Constant("Bananan")},
				Pattern{Constant("nananananan")},
				Pattern{Field("f"), Constant("Batman")},
				Pattern{Constant("an")},
			},
			expected: Alternatives{
				Pattern{Constant("Banan")},
				Pattern{Constant("nanananan")},
				Pattern{Field("f"), Constant("Batm")},
			},
			prefix: "an",
		},
		{
			input: Alternatives{
				Pattern{Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad.")},
			},
			expected: Alternatives{},
			prefix:   "Repetition is bad.",
		},
		{
			input: Alternatives{
				Pattern{Constant("Repetition is bad.")},
				Pattern{Field("isit"), Constant("Repetition is bad.")},
				Pattern{Constant("Repetition is bad.")},
			},
			expected: Alternatives{
				Pattern{Field("isit")},
			},
			prefix: "Repetition is bad.",
		},
	} {
		prefix, result := extractTrailingConstantPrefix(test.input)
		assert.Equal(t, test.prefix, prefix)
		assert.Equal(t, test.expected, result)
	}
}

func TestFixAlternativesEdgeSpace(t *testing.T) {
	testAction(t, fixAlternativesEdgeSpace, []actionTestCase{
		{
			title: "leading space",
			input: LinearSelect{
				Nodes: []Operation{
					LinearSelect{},
					LinearSelect{
						Nodes: []Operation{
							Match{
								Pattern: Pattern{
									Constant("hello"),
									Alternatives{
										Pattern{Constant(" world")},
										Pattern{Constant(" "), Field("capture")},
										Pattern{Constant(" ")},
									}},
							},
						},
					},
					MsgIdSelect{},
				},
			},
			expected: LinearSelect{
				Nodes: []Operation{
					LinearSelect{},
					LinearSelect{
						Nodes: []Operation{
							Match{
								Pattern: Pattern{
									Constant("hello "),
									Alternatives{
										Pattern{Constant("world")},
										Pattern{Field("capture")},
									}},
							},
						},
					},
					MsgIdSelect{},
				},
			},
		},
		{
			title: "first item",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant(" hello")},
						Pattern{Constant(" world!")},
					},
				},
			},
			expected: Match{
				Pattern: Pattern{
					Constant(" "),
					Alternatives{
						Pattern{Constant("hello")},
						Pattern{Constant("world!")},
					},
				},
			},
		},
		{
			title: "trailing space",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("hello ")},
						Pattern{Constant("worldo ")},
					},
				},
			},
			expected: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("hell")},
						Pattern{Constant("world")},
					},
					Constant("o "),
				},
			},
		},
		{
			title: "trailing and leading",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("banana")},
						Pattern{Constant("banda")},
					},
				},
			},
			expected: Match{
				Pattern: Pattern{
					Constant("ban"),
					Alternatives{
						Pattern{Constant("an")},
						Pattern{Constant("d")},
					},
					Constant("a"),
				},
			},
		},
		{
			title: "two alternatives",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("logs ")},
						Pattern{Constant("files ")},
					},
					Alternatives{
						Pattern{Constant("scan")},
						Pattern{Constant("skipped")},
					},
				},
			},
			expected: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("log")},
						Pattern{Constant("file")},
					},
					Constant("s s"),
					Alternatives{
						Pattern{Constant("can")},
						Pattern{Constant("kipped")},
					},
				},
			},
		},
		{
			title: "two alternatives with field",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("logs ")},
						Pattern{Constant("files ")},
					},
					Field("z"),
					Alternatives{
						Pattern{Constant("scan")},
						Pattern{Constant("skipped")},
					},
				},
			},
			expected: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Constant("log")},
						Pattern{Constant("file")},
					},
					Constant("s "),
					Field("z"),
					Constant("s"),
					Alternatives{
						Pattern{Constant("can")},
						Pattern{Constant("kipped")},
					},
				},
			},
		},
	})
}

func TestFixExtraLeadingSpaceInConstants(t *testing.T) {
	testAction(t, fixExtraLeadingSpaceInConstants, []actionTestCase{
		{
			title: "leading space",
			input: LinearSelect{
				Nodes: []Operation{
					LinearSelect{},
					LinearSelect{
						Nodes: []Operation{
							Match{
								Pattern: Pattern{
									Constant("  hello"),
									Alternatives{
										Pattern{Constant("world")},
										Pattern{Field("capture")},
									},
									Constant(" space here"),
								},
							},
						},
					},
					MsgIdSelect{},
				},
			},
			expected: LinearSelect{
				Nodes: []Operation{
					LinearSelect{},
					LinearSelect{
						Nodes: []Operation{
							Match{
								Pattern: Pattern{
									Field(""),
									Constant("hello"),
									Alternatives{
										Pattern{Constant("world")},
										Pattern{Field("capture")},
									},
									Field(""),
									Constant("space here"),
								},
							},
						},
					},
					MsgIdSelect{},
				},
			},
		},
		{
			title: "Extra space after alternative",
			input: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Field("a")},
						Pattern{Field("b")},
					},
					Constant(" :"),
				},
			},
			expected: Match{
				Pattern: Pattern{
					Alternatives{
						Pattern{Field("a")},
						Pattern{Field("b")},
					},
					Field(""),
					Constant(":"),
				},
			},
		},
	})
}

func TestRemoveNops(t *testing.T) {
	testAction(t, removeNoops, []actionTestCase{
		{
			title: "single argument",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Noop{},
				},
			},
			expected: Match{
				Input:     "test",
				OnSuccess: []Operation{},
			},
		},
		{
			title: "first",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Noop{},
					Call{},
					SetField{},
				},
			},
			expected: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					SetField{},
				},
			},
		},
		{
			title: "last",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					SetField{},
					Noop{},
				},
			},
			expected: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					SetField{},
				},
			},
		},
		{
			title: "middle",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					Noop{},
					DateTime{},
					SetField{},
				},
			},
			expected: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					DateTime{},
					SetField{},
				},
			},
		},
		{
			title: "multiple",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Noop{},
					Call{},
					Noop{},
					Noop{},
					Noop{},
					DateTime{},
					Noop{},
					SetField{},
					Noop{},
				},
			},
			expected: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					DateTime{},
					SetField{},
				},
			},
		},
		{
			title: "none",
			input: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					DateTime{},
					SetField{},
				},
			},
			expected: Match{
				Input: "test",
				OnSuccess: []Operation{
					Call{},
					DateTime{},
					SetField{},
				},
			},
		},
	})
}
