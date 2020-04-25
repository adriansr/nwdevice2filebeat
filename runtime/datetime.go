//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

type dateTime struct {
	target  string
	fields  []string
	formats []string
}

func (d dateTime) Run(ctx *Context) (err error) {
	values := make([]string, len(d.fields))
	for idx, fld := range d.fields {
		values[idx], err = ctx.Fields.Get(fld)
		if err != nil {
			return errors.Errorf("field '%s' missing for date conversion", fld)
		}
	}
	str := strings.Join(values, " ")

	for _, format := range d.formats {
		if date, err := time.Parse(format, str); err == nil {
			log.Printf("EVNTTIME succeeded str=%s format=%s result=%s",
				str, format, date.String())
			ctx.Fields.Put(d.target, date.String())
			return nil
		}
	}
	return errors.Errorf("EVNTTIME failed to convert date str=%s formats=%v",
		str, d.formats)
}

var timeSpecToGolang = map[byte]string{
	'R': "January",
	'B': "Jan",
	'M': "01",
	'G': "1",
	'D': "02",
	'F': "2",
	'H': "15",
	'I': "03",
	'N': "15", // This is supposed to be "3" but actually seems to mean 15.
	'T': "04",
	'U': "4",
	// 'J': ... julian day
	'P': "PM",
	// 'Q': "p.m."
	'S': "05",
	'O': "5",
	'Y': "06",
	'W': "2006",
	'Z': "15:04:05",
	// 'A': ... number of days from the event time
	// 'X': ... UNIX timestamp
}

func dateTimeFormatToGolangLayout(input []parser.DateTimeItem) (layout string, err error) {
	var gen []byte
	lastWasConstant := true
	for _, item := range input {
		if item.Spec() == parser.DateTimeConstant {
			lastWasConstant = true
			gen = append(gen, item.Value()...)
		} else {
			ref, ok := timeSpecToGolang[item.Spec()]
			if !ok {
				return "", errors.Errorf("EVNTTIME spec %%%c not supported", item.Spec())
			}
			if !lastWasConstant {
				gen = append(gen, ' ')
			}
			gen = append(gen, ref...)
			lastWasConstant = false
		}
	}
	return string(gen), nil
}
