//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import "github.com/pkg/errors"

func ParsePatternWithAlternatives(data string) (pattern Pattern, err error) {
	alts, err := splitAlternatives(data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to split alternatives in <<%s>>", data)
	}
	return dissectRecursive(alts)
}

func dissectRecursive(expr []interface{}) (pattern Pattern, err error) {
	for _, e := range expr {
		switch v := e.(type) {
		case string:
			inner, err := ParsePattern(v)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse inner pattern <<%s>>", v)
			}
			pattern = append(pattern, inner...)
		case []interface{}:
			var alt Alternatives
			for _, subexpr := range v {
				inner, err := dissectRecursive([]interface{}{subexpr})
				if err != nil {
					return nil, err
				}
				alt = append(alt, inner)
			}
			pattern = append(pattern, alt)
		}
	}
	return
}
