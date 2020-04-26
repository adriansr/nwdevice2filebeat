//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/parser"
)

type dateTime struct {
	target  string
	fields  []string
	formats []string
	parser  func(format, value string) (time.Time, error)
}

func newDateTime(ref parser.DateTime, loc *time.Location) (dt dateTime, err error) {
	dt = dateTime{
		target: ref.Target,
		fields: ref.Fields,
	}
	dt.formats = make([]string, len(ref.Formats))
	for idx, fmt := range ref.Formats {
		if dt.formats[idx], err = dateTimeFormatToGolangLayout(fmt); err != nil {
			return dt, err
		}
	}
	if loc == nil {
		dt.parser = time.Parse
	} else {
		dt.parser = func(format, value string) (time.Time, error) {
			return time.ParseInLocation(format, value, loc)
		}
	}
	return dt, nil
}

func loadValues(fields []string, ctx *Context) (value string, err error) {
	values := make([]string, len(fields))
	for idx, fld := range fields {
		values[idx], err = ctx.Fields.Get(fld)
		if err != nil {
			return "", errors.Errorf("source field '%s' missing", fld)
		}
	}
	return strings.Join(values, " "), nil
}

func (d dateTime) Run(ctx *Context) (err error) {
	str, err := loadValues(d.fields, ctx)
	if err != nil {
		return errors.Wrap(err, "cannot apply EVNTTIME")
	}
	if !d.tryConvert(str, ctx) {
		return errors.Errorf("EVNTTIME failed to convert date str=%s formats=%v",
			str, d.formats)
	}

	return nil
}

func (d dateTime) tryConvert(str string, ctx *Context) bool {
	for _, format := range d.formats {
		if date, err := d.parser(format, str); err == nil {
			log.Printf("EVNTTIME succeeded str=%s format=%s result=%s",
				str, format, date.String())
			ctx.Fields.Put(d.target, date.String())
			return true
		}
	}
	return false
}

var timeSpecToGolang = map[byte]string{
	'C': "1/2/06 3:4:5",
	'R': "January",
	'B': "Jan",
	'M': "01",
	'G': "1",
	'D': "02",
	'F': "2",
	'H': "15",
	'I': "03",
	'N': "15", // This is supposed to be "3" (am/pm) but actually seems to mean 15 (24h). TODO: cfg flag
	'T': "04",
	'U': "4",
	'J': "__2", // julian day, this won't be correct if padded with zeroes.
	'P': "PM",
	'Q': "PM", // This is supposed to be "P.M." which golang doesn't support.
	'S': "05",
	'O': "5",
	'Y': "06",
	'W': "2006",
	'Z': "15:04:05",
	// 'A': ... number of days from the event time (for DUR, not EVNTTIME)
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

var timeSpecToDuration = map[byte]time.Duration{
	// Only these 3 patterns seen in the wild:
	// '%A%N%T%O'
	// '%N%U%O'
	// '%N:%U:%O'
	'M': time.Hour * 24 * 30,
	'G': time.Hour * 24 * 30,
	'D': time.Hour * 24,
	'F': time.Hour,
	'H': time.Hour,
	'I': time.Hour,
	'N': time.Hour,
	'T': time.Minute,
	'U': time.Minute,
	'J': time.Hour * 24,
	'S': time.Second,
	'O': time.Second,
	'A': time.Hour * 24,
	// Z
}

type duration struct {
	target  string
	fields  []string
	formats [][]byte
}

func newDuration(ref parser.Duration) (dur duration, err error) {
	dur = duration{
		target: ref.Target,
		fields: ref.Fields,
	}
	dur.formats = make([][]byte, len(ref.Formats))
	for idx, fmt := range ref.Formats {
		for _, item := range fmt {
			if item.Spec() != parser.DateTimeConstant {
				if item.Spec() != 'Z' {
					dur.formats[idx] = append(dur.formats[idx], item.Spec())
				} else {
					// Treat 'Z' as 'HTS' to simplify parsing.
					dur.formats[idx] = append(dur.formats[idx], []byte("HTS")...)
				}
			}
		}
	}
	if len(dur.formats) == 0 {
		return dur, errors.Errorf("at %s: no formats specified for DUR", ref.Source())
	}
	return dur, nil
}

type intScanner struct {
	str string
	pos int
}

func (i *intScanner) next() (value int64, ok bool) {
	n := len(i.str)
	for ; i.pos < n && i.str[i.pos] < '0' || i.str[i.pos] > '9'; i.pos++ {
	}
	if i.pos == n {
		return 0, false
	}
	value = int64(i.str[i.pos] - '0')
	for i.pos++; i.pos < n && i.str[i.pos] >= '0' && i.str[i.pos] <= '9'; i.pos++ {
		value = value*10 + int64(i.str[i.pos]-'0')
	}
	return value, true
}

func (d duration) Run(ctx *Context) (err error) {
	var scanner intScanner
	scanner.str, err = loadValues(d.fields, ctx)
	if err != nil {
		return errors.Wrap(err, "cannot apply DUR")
	}
	var seconds int64
	for _, fmt := range d.formats {
		seconds = 0
		for _, chr := range fmt {
			multiplier, found := timeSpecToDuration[chr]
			if !found {
				err = errors.Errorf("format specified %%%c not understood in DUR", chr)
				continue
			}
			value, ok := scanner.next()
			if !ok {
				err = errors.Errorf("not enough fields for DUR")
				continue
			}
			// Don't have subsecond precision
			seconds += int64((multiplier * time.Duration(value)).Seconds())
		}
	}
	if err != nil {
		return err
	}
	ctx.Fields.Put(d.target, strconv.FormatInt(seconds, 10))
	return nil
}
