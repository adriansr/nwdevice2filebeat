//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"testing"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/stretchr/testify/assert"
)

func Test_urlExtract_Run(t *testing.T) {
	for _, test := range []struct {
		title    string
		url      string
		expected map[string]string
	}{
		{
			title: "complete URL",
			url:   "https://www.example.net/docs/networking/guide.html?darkmode=1",
			expected: map[string]string{
				"$DOMAIN": "example.net",
				"$EXT":    ".html",
				"$FQDN":   "www.example.net",
				"$PAGE":   "guide.html",
				"$PATH":   "/docs/networking/guide.html",
				"$PORT":   "443",
				"$QUERY":  "darkmode=1",
				"$ROOT":   "https://www.example.net/",
			},
		},
		{
			title: "single domain name",
			url:   "cloud.elastic.co",
			expected: map[string]string{
				"$DOMAIN": "elastic.co",
				"$EXT":    "",
				"$FQDN":   "cloud.elastic.co",
				"$PAGE":   "",
				"$PATH":   "",
				"$PORT":   "",
				"$QUERY":  "",
				"$ROOT":   "http://cloud.elastic.co/",
			},
		},
		{
			title: "empty",
			url:   "",
			expected: map[string]string{
				"$DOMAIN": "",
				"$EXT":    "",
				"$FQDN":   "",
				"$PAGE":   "",
				"$PATH":   "",
				"$PORT":   "",
				"$QUERY":  "",
				"$ROOT":   "",
			},
		},
	} {
		t.Run(test.title, func(t *testing.T) {
			results := make(map[string]string)
			for k := range test.expected {
				code, found := parser.VarNameToURLComponent[k]
				if !assert.True(t, found) {
					t.FailNow()
				}
				extractor := urlExtract{Component: code}
				result, _ := extractor.Extract(test.url)
				//assert.NoError(t, err)
				results[k] = result
			}
			assert.Equal(t, test.expected, results)
		})
	}
}
