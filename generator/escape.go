//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "strings"

const (
	escapedBackslash = "\\\\"
	escapedSingleQuote = "\\'"
)

var constantEscapes = strings.NewReplacer("\\\\", "\\", "\\'", "'")

func unescapeConstant(b string) string {
	if strings.Index(b, "\\") != -1{
		return constantEscapes.Replace(b)
	}
	return b
}

