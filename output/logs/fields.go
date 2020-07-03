//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logs

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

const (
	colName = 4
	colType = 6
)

func makeYear(rng *rand.Rand, t time.Time) string {
	return strconv.Itoa(t.Year())
}

func makeMonth(rng *rand.Rand, t time.Time) string {
	return t.Month().String()
}

func makeDay(rng *rand.Rand, t time.Time) string {
	return strconv.Itoa(t.Day())
}

func makeDate(rng *rand.Rand, t time.Time) string {
	yy, mm, dd := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", yy, mm, dd)
}

func makeTime(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d",
		t.Hour(),
		t.Minute(),
		t.Second())
}

func oneOf(list ...string) valueGenerator {
	return func(rng *rand.Rand, t time.Time) string {
		return list[rng.Intn(len(list))]
	}
}

func fromDatePattern(s string) valueGenerator {
	p := dateComponent(make([]parser.DateTimeItem, len(s)))
	for idx, chr := range s {
		p[idx] = parser.DateTimeSpec(chr)
	}
	return p.Generate
}

var overrideFields = map[string]valueGenerator{
	"messageid": func(rng *rand.Rand, t time.Time) string {
		return fmt.Sprintf("MSGID%04x", rng.Intn(0x10000))
	},
	"space": func(rng *rand.Rand, t time.Time) string {
		var spaces = "    "
		return spaces[:1+rng.Intn(len(spaces)-1)]
	},
	"msgIdPart1": func(rng *rand.Rand, t time.Time) string {
		return fmt.Sprintf("MSGA_%04x", rng.Intn(0x10000))
	},
	"msgIdPart2": func(rng *rand.Rand, t time.Time) string {
		return fmt.Sprintf("MSGB_%04x", rng.Intn(0x10000))
	},
	"msgIdPart3": func(rng *rand.Rand, t time.Time) string {
		return fmt.Sprintf("MSGC_%04x", rng.Intn(0x10000))
	},

	"month": makeMonth,
	"day":   makeDay,
	"year":  makeYear,
	"date":  makeDate,
	"datetime": func(rng *rand.Rand, t time.Time) string {
		return t.String()
		//return makeDate(rng) + "T" + makeTime(rng)
	},
	"event_id": func(rng *rand.Rand, t time.Time) string {
		return fmt.Sprintf("%08x", rng.Intn(0x100000000))
	},
	"reason": oneOf(
		"blocked",
		"allowed",
		"denied",
		"cancelled",
		"accepted",
		"denylist",
		"allowlist",
		"malware",
		"attack",
	),
	"action": oneOf(
		"block",
		"allow",
		"deny",
		"cancel",
		"accept",
	),
	"status":         makeText,
	"hour":           fromDatePattern("H"),
	"min":            fromDatePattern("T"),
	"sec":            fromDatePattern("S"),
	"url":            makeURL,
	"url_raw":        makeURL,
	"p_url":          makeURL,
	"url_fld":        makeURL,
	"web_referer":    makeURL,
	"referer":        makeURL,
	"p_web_referer":  makeURL,
	"user_agent":     makeUserAgent,
	"p_user_agent":   makeUserAgent,
	"count":          makeInt,
	"number":         makeInt,
	"method":         makeHTTPMethod,
	"domain":         makeHostName,
	"tgtdomain":      makeHostName,
	"remote_domain":  makeHostName,
	"host":           makeHostName,
	"hostname":       makeHostName,
	"shost":          makeHostName,
	"dhost":          makeHostName,
	"hostid":         makeHostName,
	"devicehostname": makeHostName,
	"fqdn":           makeHostName,
	"protocol": oneOf(
		"tcp",
		"udp",
		"icmp",
		"igmp",
		"ggp",
		"rdp",
		"ipv6",
		"ipv6-icmp",
	),
	"severity": oneOf(
		"low",
		"medium",
		"high",
		"very-high",
	),
	"interface":  makeInterface,
	"sinterface": makeInterface,
	"dinterface": makeInterface,
	"linterface": makeInterface,
	"version":    join(ct("1."), makeInt),
	"result":     oneOf("failure", "success", "unknown"),
	"process":    join(makeText, ct(".exe")),
	"timezone": oneOf(
		"CET",
		"CEST",
		"OMST",
		"ET",
		"CT",
		"PT",
		"PST",
		"GMT+02:00",
		"GMT-07:00",
	),
	"gmtdate": func(rng *rand.Rand, t time.Time) string {
		return t.UTC().Format(time.RFC3339)
	},
}

type fieldsGen map[string]valueGenerator

func newFieldsFromCSV(path string) (fieldsGen, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	csvReader := csv.NewReader(f)
	csvReader.FieldsPerRecord = -1
	knownFields := make(fieldsGen)
	for k, v := range overrideFields {
		knownFields[k] = v
	}
	for lineNum := 1; ; lineNum++ {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "failed reading line %d", lineNum)
		}
		if len(record) < 17 {
			return nil, fmt.Errorf("line %d has unexpected number of columns: %d", lineNum, len(record))
		}
		if lineNum == 1 && record[0] == "revision" {
			continue
		}
		name, typ := record[colName], record[colType]
		if _, exists := knownFields[name]; exists {
			continue
		}
		gen, ok := types[typ]
		if !ok {
			if typ == "" {
				continue
			}
			return nil, fmt.Errorf("CSV field %s has unsupported type %s at line %d", name, typ, lineNum)
		}
		knownFields[name] = gen
	}
	return knownFields, nil
}
