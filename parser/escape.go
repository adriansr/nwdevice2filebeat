//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import (
	"regexp"
	"strings"
)

var (
	fieldNameRegex  = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z_$.0-9]+$`)
	constantEscapes = strings.NewReplacer("\\\\", "\\", "\\'", "'")
)

func unescapeConstant(b string) string {
	if strings.Index(b, "\\") != -1 {
		return constantEscapes.Replace(b)
	}
	return b
}

func disambiguateFieldOrConstant(s string) Value {
	trimmed := strings.Trim(s, " ")
	n := len(trimmed)
	if n == 0 {
		// Return original, spaces and all
		return Constant(s)
	}
	if n > 1 && trimmed[0] == '\'' && trimmed[n-1] == '\'' {
		return Constant(unescapeConstant(trimmed[1 : n-1]))
	}
	if fieldNameRegex.MatchString(trimmed) {
		return Field{Name: trimmed}
	}
	return Constant(trimmed)
}
