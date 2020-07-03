//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package logs

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/layout"
	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
)

const fieldsFile = "ecs-mappings.csv"

var (
	startTime = time.Unix(1508168573, 0).UTC()
	endTime   = time.Unix(1593548776, 0).UTC()
)

type logs struct {
	tmpFile   *os.File
	rng       *rand.Rand
	fieldsGen fieldsGen
}

func init() {
	instance := new(logs)
	output.Registry.MustRegister("logs", instance)
}

func (lg *logs) Settings() config.PipelineSettings {
	return config.PipelineSettings{
		// Prefer non-split patterns
		Dissect: false,

		// Better with payload information.
		StripPayload: false,
	}
}

func (lg *logs) Populate(lyt *layout.Generator) (err error) {
	return errors.New("unimplemented")
}

func (lg *logs) OutputFile() string {
	return lg.tmpFile.Name()
}

func (lg *logs) Generate(p parser.Parser) (err error) {
	lg.fieldsGen, err = newFieldsFromCSV(fieldsFile)
	if err != nil {
		return errors.Wrapf(err, "loading %s", fieldsFile)
	}
	lg.tmpFile, err = ioutil.TempFile("", "generated-*.log")
	if err != nil {
		return err
	}
	defer lg.tmpFile.Close()
	num, err := lg.measureComplexity(p.Root)
	if err != nil {
		return errors.Wrap(err, "measuring complexity")
	}
	log.Printf("Total number of possible pattern combinations is %d", num)
	log.Printf("Generating %d random lines using seed=%x", p.Config.NumLines, p.Config.Seed)
	lg.rng = rand.New(rand.NewSource(int64(p.Config.Seed)))
	t := startTime
	delta := endTime.Sub(startTime) / time.Duration(p.Config.NumLines)
	minDelta := time.Second
	for line := uint(1); line <= p.Config.NumLines; line++ {
		text, err := lg.newLine(p, t)
		if err != nil {
			return errors.Wrapf(err, "failed to generate line #%d", line)
		}
		lg.tmpFile.WriteString(text)
		lg.tmpFile.WriteString("\n")
		t = t.Add(2*time.Duration(lg.rng.Intn(int(delta-minDelta))) + minDelta)
	}
	return nil
}

func (lg *logs) measureComplexity(node parser.Operation) (combinations uint64, err error) {
	switch v := node.(type) {
	case parser.Chain:
		combinations = 1
		for _, child := range v.Children() {
			inner, err := lg.measureComplexity(child)
			if err != nil {
				return 0, err
			}
			combinations *= inner
		}

	case parser.LinearSelect, parser.MsgIdSelect:
		combinations = 0
		for _, child := range v.Children() {
			inner, err := lg.measureComplexity(child)
			if err != nil {
				return 0, err
			}
			combinations += inner
		}

	case parser.Match:
		combinations = patternComplexity(v.Pattern)

	default:
		return 0, errors.Errorf("unsupported node type %T", v)
	}
	return combinations, nil
}

func patternComplexity(pattern parser.Pattern) (c uint64) {
	c = 1
	for _, item := range pattern {
		if alts, isAlt := item.(parser.Alternatives); isAlt {
			k := uint64(1)
			for _, alt := range alts {
				k += patternComplexity(alt)
			}
			c *= k
		}
	}
	return c
}

type valueHint interface {
	fmt.Stringer
	Quality() int
	Generate(rng *rand.Rand, t time.Time) string
}

// Empty hint, indicates that the value is captured/recaptured in a pattern
type captured struct{}

func (_ captured) String() string { return "captured" }
func (_ captured) Quality() int   { return 0 }
func (_ captured) Generate(rng *rand.Rand, t time.Time) string {
	return makeText(rng, t)
}

type copyField parser.Field

func (c copyField) String() string { return "copy_from(" + c.Name + ")" }
func (_ copyField) Quality() int   { return 0 }
func (_ copyField) Generate(rng *rand.Rand, t time.Time) string {
	return makeText(rng, t)
}

type constant parser.Constant

func (c constant) String() string { return "const('" + parser.Constant(c).Value() + "')" }
func (_ constant) Quality() int   { return 0 }
func (_ constant) Generate(rng *rand.Rand, t time.Time) string {
	return makeText(rng, t)
}

type date struct{}

func (c date) String() string { return "date" }
func (_ date) Quality() int   { return 0 }
func (_ date) Generate(rng *rand.Rand, t time.Time) string {
	return makeTimeT(rng, t)
}

type dateComponent []parser.DateTimeItem

func (c dateComponent) String() string {
	var sb strings.Builder
	sb.WriteString("date_comp('")
	for _, comp := range c {
		if comp.Spec() == parser.DateTimeConstant {
			sb.WriteString(comp.Value())
		} else {
			sb.WriteByte('%')
			sb.WriteByte(comp.Spec())
		}
	}
	sb.WriteString("')")
	return sb.String()
}

func (dateComponent) Quality() int { return 10 }

func (c dateComponent) Generate(rng *rand.Rand, t time.Time) string {
	var sb strings.Builder
	lastIsConst := true
	for _, comp := range c {
		if comp.Spec() == parser.DateTimeConstant {
			sb.WriteString(comp.Value())
			lastIsConst = true
			continue
		}
		if !lastIsConst {
			sb.WriteByte(' ')
		}
		lastIsConst = false
		switch comp.Spec() {
		case 'R': // Long month name
			sb.WriteString(t.Month().String())
		case 'B': // Short month name
			sb.WriteString(t.Month().String()[:3])
		case 'M': // 2-digit month
			sb.WriteString(fmt.Sprintf("%02d", t.Month()))
		case 'G': // variable month
			sb.WriteString(strconv.Itoa(int(t.Month())))
		case 'D': // 2-digit day
			sb.WriteString(fmt.Sprintf("%02d", t.Day()))
		case 'F': // variable day
			sb.WriteString(strconv.Itoa(int(t.Day())))
		case 'H': // 2-digit 24h
			sb.WriteString(fmt.Sprintf("%02d", t.Hour()))
		case 'I': // 2-digit 12h
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			sb.WriteString(fmt.Sprintf("%02d", h))
		case 'N': // variable 12h
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			sb.WriteString(strconv.Itoa(h))
		case 'T', 'U': // 2-digit minute
			sb.WriteString(fmt.Sprintf("%02d", t.Minute()))
		case 'P': // AM/PM
			if t.Hour() < 12 {
				sb.WriteString("AM")
			} else {
				sb.WriteString("PM")
			}
		case 'Q': // A.M./P.M.
			if t.Hour() < 12 {
				sb.WriteString("A.M.")
			} else {
				sb.WriteString("P.M.")
			}
		case 'S', 'O': // 2-digit seconds
			sb.WriteString(fmt.Sprintf("%02d", t.Second()))
		case 'Y': // 2-digit year
			sb.WriteString(fmt.Sprintf("%02d", t.Year()%100))
		case 'W': // 4-digit year
			sb.WriteString(fmt.Sprintf("%04d", t.Year()))
		case 'Z':
			sb.WriteString(fmt.Sprintf("%02d:%02d:%02d",
				t.Hour(),
				t.Minute(),
				t.Second(),
			))
		case 'X':
			sb.WriteString(fmt.Sprintf("%d", t.Unix()))
		default:
			panic(errors.Errorf("unsupported format %%%s for date hint", comp.Spec()))
		}
	}
	return sb.String()
}

type valueMapKey []string

func newValueMapKey(vm *parser.ValueMap) valueMapKey {
	result := make(valueMapKey, 0, len(vm.Mappings))
	for key := range vm.Mappings {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func (c valueMapKey) String() string { return "value_map" }
func (valueMapKey) Quality() int     { return 10 }
func (c valueMapKey) Generate(rng *rand.Rand, t time.Time) string {
	return c[rng.Intn(len(c))]
}

type urlComponent parser.URLComponent

func (c urlComponent) String() string { return "url_component" }
func (urlComponent) Quality() int     { return 0 }
func (_ urlComponent) Generate(rng *rand.Rand, t time.Time) string {
	return makeText(rng, t)
}

type url struct{}

func (c url) String() string { return "url" }
func (url) Quality() int     { return 10 }

func (_ url) Generate(rng *rand.Rand, t time.Time) string {
	return makeURL(rng, t)
}

type fieldHints map[string][]valueHint

type lineComposer struct {
	payload     *int
	time        time.Time
	parser      parser.Parser
	rng         *rand.Rand
	fieldsGen   fieldsGen
	expression  parser.Pattern
	knownFields fieldHints
	history     []string
}

func (lc *lineComposer) Build() (string, error) {
	var sb strings.Builder
	for _, act := range lc.expression {
		switch v := act.(type) {
		case parser.Constant:
			sb.WriteString(v.Value())
		case parser.Field:
			value, err := lc.valueFor(v.Name)
			if err != nil {
				return "", errors.Wrapf(err, "getting value for field '%s'", v.Name)
			}
			sb.WriteString(value)
		default:
			return "", errors.Errorf("no support for type %T when building log", v)
		}
	}
	return sb.String(), nil
}

func (lc *lineComposer) valueFor(field string) (string, error) {
	hints, ok := lc.knownFields[field]
	if !ok || len(hints) == 0 {
		return "", errors.Errorf("field %s not captured", field)
	}
	var c captured
	if hints[0] != c {
		return "", errors.Errorf("field %s is not captured", field)
	}
	best := hints[0]
	idx := 1
	for ; idx < len(hints); idx++ {
		hint := hints[idx]
		if hint == c {
			// Discard used hints
			lc.knownFields[field] = hints[idx:]
			break
		}
		if best.Quality() < hint.Quality() {
			best = hint
		}
	}
	if best.Quality() > 0 {
		return best.Generate(lc.rng, lc.time), nil
	}
	return lc.defaultValueFor(field)
}

func (lc *lineComposer) defaultValueFor(field string) (string, error) {
	gen, ok := lc.fieldsGen[field]
	if !ok {
		if len(field) > 0 {
			// override header fields.
			if field[0] == 'h' {
				if value, err := lc.defaultValueFor(field[1:]); err == nil {
					return value, nil
				}
			}
			// otherwise just populate unhinted temporary fields with text
			if lastChr := field[len(field)-1]; lastChr >= '0' && lastChr <= '9' {
				return makeText(lc.rng, lc.time), nil
			}
		}
		return "", errors.New("no default generator for field")
	}
	return gen(lc.rng, lc.time), nil
}

func (lc *lineComposer) Log() {
	log.Printf("Expression: %s", lc.expression.Hashable())
	log.Printf("Hints: (%d values)", len(lc.knownFields))
	keys := make([]string, 0, len(lc.knownFields))
	for k := range lc.knownFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf(" '%s' = %+v", k, lc.knownFields[k])
	}
}

func (lg *logs) newLine(p parser.Parser, t time.Time) (string, error) {
	state := lineComposer{
		time:        t,
		parser:      p,
		rng:         lg.rng,
		fieldsGen:   lg.fieldsGen,
		knownFields: make(fieldHints),
	}
	if err := lg.randomWalk(p.Root, &state); err != nil {
		return "", errors.Wrapf(err, "error during random walk (historic:%+v)", state.history)
	}
	state.Log()
	return state.Build()
}

func (lg *logs) randomWalk(node parser.Operation, st *lineComposer) error {
	switch v := node.(type) {
	case parser.Chain:
		for _, child := range v.Children() {
			if err := lg.randomWalk(child, st); err != nil {
				return err
			}
		}

	case parser.LinearSelect:
		children := v.Children()
		return lg.randomWalk(children[lg.rng.Intn(len(children))], st)

	case parser.Match:
		st.history = append(st.history, v.ID)
		if err := st.appendPattern(v.Pattern); err != nil {
			return err
		}
		if err := st.appendActions(v.OnSuccess); err != nil {
			return err
		}

	case parser.MsgIdSelect:
		_, found := st.knownFields["messageid"]
		if !found {
			return errors.New("no hints for messageid")
		}
		// TODO:
		return lg.randomWalk(v.Children()[lg.rng.Intn(len(v.Children()))], st)
	default:
		return errors.Errorf("unsupported node type %T", v)
	}
	return nil
}

func findField(p parser.Pattern, name string) (pos []int) {
	for idx, entry := range p {
		if fld, ok := entry.(parser.Field); ok && fld.Name == name {
			pos = append(pos, idx)
		}
	}
	return pos
}

func (lc *lineComposer) appendPattern(p parser.Pattern) error {
	if lc.payload != nil {
		loc := *lc.payload
		if overlap := len(lc.expression) - loc; overlap <= 0 || len(p) < overlap {
			errors.Errorf("payload overlap is troublesome. historic=%+v overlap=%d in=%s new=%s",
				lc.history, overlap, p.String(), lc.expression.String())
		}
		// TODO
		lc.expression = lc.expression[:loc]
		lc.payload = nil
	}

	for idx, entry := range p {
		switch v := entry.(type) {
		case parser.Field:
			lc.knownFields[v.Name] = append(lc.knownFields[v.Name], captured{})
			lc.expression = append(lc.expression, entry)
		case parser.Constant:
			lc.expression = append(lc.expression, entry)

		case parser.Payload:
			if idx != len(p)-1 {
				return errors.Errorf("payload field is not the final entry in the pattern. historic=%+v pattern=%s",
					lc.history, p.String())
			}
			if v.Name != "" {
				loc := findField(lc.expression, v.Name)
				if len(loc) != 1 {
					return errors.Errorf("payload field must appear once. historic=%+v loc=%+v pattern=%s",
						lc.history, loc, p.String())
				}
				lc.payload = &loc[0]
			}
			break

		case parser.Alternatives:
			if err := lc.appendPattern(v[lc.rng.Intn(len(v))]); err != nil {
				return err
			}

		default:
			return errors.Errorf("no support for %T in pattern", v)
		}
	}
	return nil
}

func (lc *lineComposer) appendActions(list parser.OpList) error {
	for _, act := range list {
		switch v := act.(type) {
		case parser.SetField:
			switch vv := v.Value[0].(type) {
			case parser.Field:
				lc.knownFields[v.Target] = append(lc.knownFields[v.Target], copyField(vv))
			case parser.Constant:
				lc.knownFields[v.Target] = append(lc.knownFields[v.Target], constant(vv))
			default:
				return errors.Errorf("unexpected value type %T in %s", vv, v.Hashable())
			}
		case parser.DateTime:
			if err := lc.enrichFromDateTime(v); err != nil {
				return err
			}
		case parser.ValueMapCall:
			vm, ok := lc.parser.ValueMapsByName[v.MapName]
			if !ok {
				return errors.Errorf("valuemap call for unknown valuemap %s", v.MapName)
			}
			if len(v.Key) != 1 {
				continue
			}
			fld, ok := v.Key[0].(parser.Field)
			if ok {
				lc.knownFields[fld.Name] = append(lc.knownFields[fld.Name], newValueMapKey(vm))
			}

		case parser.URLExtract:
			lc.knownFields[v.Target] = append(lc.knownFields[v.Target], urlComponent(v.Component))
			lc.knownFields[v.Source] = append(lc.knownFields[v.Source], url{})

		default:
			return errors.Errorf("unsupported action type %T", v)
		}
	}
	return nil
}

func (lc *lineComposer) enrichFromDateTime(dt parser.DateTime) error {
	lc.knownFields[dt.Target] = append(lc.knownFields[dt.Target], date{})
	// TODO: Multiple formats
	fmt := dt.Formats[0]
	fields := dt.Fields
	switch {
	case len(fields) == 1: // 1 to many
		lc.knownFields[fields[0]] = append(lc.knownFields[fields[0]], dateComponent(fmt))

	case len(fields) == len(fmt): // 1:1 mapping
		for idx, src := range fields {
			lc.knownFields[src] = append(lc.knownFields[src], dateComponent(fmt[idx:idx+1]))
		}

	case len(fields) == 2 && len(fmt) > 2: // Split 3+ fields in 2.
		// Try to split the fmt in two parts, one for date, one for time.
		pos := splitDateAndTime(fmt)
		if pos != -1 {
			lc.knownFields[fields[0]] = append(lc.knownFields[fields[0]], dateComponent(fmt[:pos]))
			lc.knownFields[fields[1]] = append(lc.knownFields[fields[1]], dateComponent(fmt[pos:]))
			break
		}
		fallthrough
	case len(fields) < len(fmt): // Split fields, at least 1 fmt per fld
		div := len(fmt) / len(fields)
		rem := len(fmt) % len(fields)
		pos := 0
		for _, fld := range fields {
			next := pos + div
			if pos == 0 {
				next += rem
			}
			lc.knownFields[fld] = append(lc.knownFields[fld], dateComponent(fmt[pos:next]))
			pos = next
		}
	default:
		// Don't know how to split this. More fields than formats
		return errors.Errorf("don't know how to split datetime %+v", dt)
	}
	return nil
}

// Which chars correspond to date components, in opposition to time components
var dateChars = map[byte]struct{}{
	'R': {},
	'B': {},
	'M': {},
	'G': {},
	'D': {},
	'F': {},
	'Y': {},
	'W': {},
}

// Find the offset which splits a datetime pattern into a date and time
// component (or time and date). Returns -1 if there is no such offset.
// (for example only date components or mixed date and time).
func splitDateAndTime(pattern []parser.DateTimeItem) int {
	split, isDate := -1, false
	for idx, elem := range pattern {
		if elem.Spec() == parser.DateTimeConstant {
			continue
		}
		_, is := dateChars[elem.Spec()]
		if is != isDate {
			isDate = is
			if idx > 0 {
				if split != -1 {
					return -1
				}
				split = idx
			}
		}
	}
	return split
}
