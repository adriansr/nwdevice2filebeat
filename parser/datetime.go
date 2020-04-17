//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import "github.com/pkg/errors"

type DateTimeItem interface {
	Spec() byte
	Value() string
}

// Caracter after % in an EVNTTIME pattern.
type DateTimeSpec byte

const DateTimeConstant byte = 0

func (s DateTimeSpec) Spec() byte {
	return byte(s)
}

func (s DateTimeSpec) Value() string {
	return ""
}

func (Constant) Spec() byte {
	return DateTimeConstant
}

type DateTime struct {
	SourceContext
	Target string
	Fields []string
	// Content is either DateTimeSpec or Constant
	Format []DateTimeItem
}

func (DateTime) Children() []Operation {
	return nil
}

var dateTimeFormatSpecifiers = map[byte]struct{} {
	'C': {},
	'R': {},
	'B': {},
	'M': {},
	'G': {},
	'D': {},
	'F': {},
	'H': {},
	'I': {},
	'N': {},
	'T': {},
	'U': {},
	'J': {},
	'P': {},
	'S': {},
	'O': {},
	'Y': {},
	'W': {},
	'Z': {},
	'A': {},
	'Q': {},
	'K': {},
	'L': {},
	'E': {},
	'X': {},
}

func isAllSpaces(s []byte) bool {
	for _, chr := range	s {
		if chr != ' ' {
			return false
		}
	}
	return true
}

func parseDateTimeFormat(fmt string) (out []DateTimeItem, err error) {
	var ct []byte
	special := false
	for _, chr := range []byte(fmt) {
		if special {
			special = false
			if chr == '%' {
				ct = append(ct, '%')
				continue
			}
			if len(ct) > 0 {
				if !isAllSpaces(ct) {
					out = append(out, Constant(ct))
				}
				ct = ct[:0]
			}
			if _, found := dateTimeFormatSpecifiers[chr]; found {
				out = append(out, DateTimeSpec(chr))
				continue
			}
			return nil, errors.Errorf("unknown format specifier: %c", chr)
		}
		if special = chr == '%'; !special {
			ct = append(ct, chr)
		}
	}
	if special {
		return nil, errors.New("format ends in %")
	}
	if len(ct) > 0 && !isAllSpaces(ct) {
		out = append(out, Constant(ct))
	}
	return out, nil
}
