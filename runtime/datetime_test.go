//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"testing"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/stretchr/testify/assert"
)

type DTFormat = []parser.DateTimeItem
type S = parser.DateTimeSpec

func Test_duration_Run(t *testing.T) {
	for _, test := range []struct {
		title    string
		fields   [][]string
		formats  []DTFormat
		expected string
	}{
		{
			title: "multiple fields",
			fields: [][]string{
				{"days", "1 "},
				{"hours", "3"},
				{"minutes", "42"},
				{"seconds", " 13"},
			},
			formats: []DTFormat{
				DTFormat{S('D'), S('F'), S('U'), S('O')},
			},
			expected: "99733",
		},
		{
			title: "single field Z",
			fields: [][]string{
				{"duration", "6:42:12"},
			},
			formats: []DTFormat{
				DTFormat{S('Z')},
			},
			expected: "24132",
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			var fields []string
			ctx := Context{
				Fields: make(Fields),
			}
			for _, kv := range test.fields {
				fields = append(fields, kv[0])
				ctx.Fields[kv[0]] = kv[1]
			}
			d, err := newDuration(parser.Duration{
				Target:  "duration",
				Fields:  fields,
				Formats: test.formats,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = d.Run(&ctx)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ctx.Fields.Get("duration")
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, test.expected, got)
		})
	}
}
