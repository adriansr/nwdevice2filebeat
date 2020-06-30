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
		case 'T': // 2-digit minute
			sb.WriteString(fmt.Sprintf("%02d", t.Minute()))
		case 'U': // variable minute
			sb.WriteString(strconv.Itoa(t.Minute()))
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
		case 'S': // 2-digit seconds
			sb.WriteString(fmt.Sprintf("%02d", t.Second()))
		case 'O': // variable seconds
			sb.WriteString(strconv.Itoa(t.Second()))
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

type valuemap parser.ValueMap

func (c valuemap) String() string { return "value_map" }
func (valuemap) Quality() int     { return 10 }
func (c valuemap) Generate(rng *rand.Rand, t time.Time) string {
	idx, n := 0, rng.Intn(len(c.Mappings))
	for k := range c.Mappings {
		if idx < n {
			return k
		}
	}
	return makeText(rng, t)
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

var subdomain = oneOf([]string{
	"",
	"www.",
	"mail.",
	"internal.",
	"api.",
	"www5.",
})

var tld = oneOf([]string{
	".com",
	".net",
	".org",
})

func (_ url) Generate(rng *rand.Rand, t time.Time) string {
	return fmt.Sprintf("https://%sexample%s/%s?%s=%s#%s",
		subdomain(rng, t),
		tld(rng, t),
		makeText(rng, t),
		makeText(rng, t),
		makeText(rng, t),
		makeText(rng, t),
	)
}

type fieldHints map[string][]valueHint

type state struct {
	time        time.Time
	parser      parser.Parser
	rng         *rand.Rand
	fieldsGen   fieldsGen
	expression  parser.Pattern
	knownFields fieldHints
	history     []string
}

func (st *state) Build() (string, error) {
	var sb strings.Builder
	for _, act := range st.expression {
		switch v := act.(type) {
		case parser.Constant:
			sb.WriteString(v.Value())
		case parser.Field:
			value, err := st.valueFor(v.Name)
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

func (st *state) valueFor(field string) (string, error) {
	hints, ok := st.knownFields[field]
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
			st.knownFields[field] = hints[idx:]
			break
		}
		if best.Quality() < hint.Quality() {
			best = hint
		}
	}
	if best.Quality() > 0 {

	}
	return st.defaultValueFor(field)
}

func (st *state) defaultValueFor(field string) (string, error) {
	gen, ok := st.fieldsGen[field]
	if !ok {
		if len(field) > 0 {
			// override header fields.
			if field[0] == 'h' {
				if value, err := st.defaultValueFor(field[1:]); err == nil {
					return value, nil
				}
			}
			// otherwise just populate unhinted temporary fields with text
			if lastChr := field[len(field)-1]; lastChr >= '0' && lastChr <= '9' {
				return makeText(st.rng, st.time), nil
			}
		}
		return "", errors.New("no default generator for field")
	}
	return gen(st.rng, st.time), nil
}

func (st *state) Log() {
	log.Printf("Expression: %s", st.expression.Hashable())
	log.Printf("Hints: (%d values)", len(st.knownFields))
	keys := make([]string, 0, len(st.knownFields))
	for k := range st.knownFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf(" '%s' = %+v", k, st.knownFields[k])
	}
}

func (lg *logs) newLine(p parser.Parser, t time.Time) (string, error) {
	state := state{
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

func (lg *logs) randomWalk(node parser.Operation, st *state) error {
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

func (s *state) appendPattern(p parser.Pattern) error {
	for idx, entry := range p {
		switch v := entry.(type) {
		case parser.Field:
			s.knownFields[v.Name] = append(s.knownFields[v.Name], captured{})
			s.expression = append(s.expression, entry)
		case parser.Constant:
			s.expression = append(s.expression, entry)

		case parser.Payload:
			if idx < len(p)-1 || v.Name != "" {
				return errors.Errorf("overlapping payload not supported at pos %d/%d %+v",
					idx, len(p), v)
			}
			break

		case parser.Alternatives:
			if err := s.appendPattern(v[s.rng.Intn(len(v))]); err != nil {
				return err
			}

		default:
			return errors.Errorf("no support for %T in pattern", v)
		}
	}
	return nil
}

func (s *state) appendActions(list parser.OpList) error {
	for _, act := range list {
		switch v := act.(type) {
		case parser.SetField:
			switch vv := v.Value[0].(type) {
			case parser.Field:
				s.knownFields[v.Target] = append(s.knownFields[v.Target], copyField(vv))
			case parser.Constant:
				s.knownFields[v.Target] = append(s.knownFields[v.Target], constant(vv))
			default:
				return errors.Errorf("unexpected value type %T in %s", vv, v.Hashable())
			}
		case parser.DateTime:
			s.knownFields[v.Target] = append(s.knownFields[v.Target], date{})
			// TODO: Multiple formats
			if len(v.Fields) == 1 {
				// 1 to many
				s.knownFields[v.Fields[0]] = append(s.knownFields[v.Fields[0]], dateComponent(v.Formats[0]))
			} else if len(v.Fields) == len(v.Formats[0]) {
				// 1:1 mapping
				for idx, src := range v.Fields {
					s.knownFields[src] = append(s.knownFields[src], dateComponent(v.Formats[0][idx:idx+1]))
				}
			} else {
				// TODO: n fields for m components
				return errors.Errorf("don't know how to split datetime %+v", v)
			}
		case parser.ValueMapCall:
			vm, ok := s.parser.ValueMapsByName[v.MapName]
			if !ok {
				return errors.Errorf("valuemap call for unknown valuemap %s", v.MapName)
			}
			s.knownFields[v.Target] = append(s.knownFields[v.Target], valuemap(*vm))

		case parser.URLExtract:
			s.knownFields[v.Target] = append(s.knownFields[v.Target], urlComponent(v.Component))
			s.knownFields[v.Source] = append(s.knownFields[v.Source], url{})

		default:
			return errors.Errorf("unsupported action type %T", v)
		}
	}
	return nil
}
