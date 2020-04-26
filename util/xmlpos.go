//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package util

import "fmt"

type XMLPos struct {
	Path string
	Line uint64
	Col  uint64
}

func (p XMLPos) String() string {
	if len(p.Path) != 0 {
		return fmt.Sprintf("%s:%d:%d", p.Path, p.Line, p.Col)
	}
	return "(unknown)"
}
