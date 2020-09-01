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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adriansr/nwdevice2filebeat/runtime"
	"github.com/pkg/errors"

	"github.com/adriansr/nwdevice2filebeat/config"
	"github.com/adriansr/nwdevice2filebeat/layout"
	"github.com/adriansr/nwdevice2filebeat/output"
	"github.com/adriansr/nwdevice2filebeat/parser"
)

const fieldsFile = "ecs-mappings.csv"

var (
	startTime = time.Unix(1451662973, 0).UTC() //  January 1, 2016 15:42:53
	endTime   = time.Unix(1575158399, 0).UTC() // November 30, 2019 23:59:59
	maxOffset = time.Hour * 24 * 31
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Error: panic when generating logs: %+v", r)
			err = errors.New("execution panic")
		}
	}()
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
	offset := time.Duration(float64(maxOffset) * 2.0 * (lg.rng.Float64() - 0.5))
	delta := endTime.Sub(startTime) / time.Duration(p.Config.NumLines)
	date := startTime.Add(offset)
	run, err := runtime.New(&p, nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to allocate runtime")
	}

	var errCount uint
	const maxErrors = 1000
	var numLines uint
	for numLines < p.Config.NumLines {
		log.Printf("=== Line #%d (%s) ===", numLines, date.Format(time.RFC3339))
		text, err := lg.newLine(p, date)
		if err != nil {
			log.Printf("Generate line error: %v", err)
			errCount++
			if errCount/(numLines+1) > maxErrors {
				return errors.New("too many errors")
			}
			continue
		}
		log.Printf("Candidate: %s", text)
		_, runErrs := run.Process([]byte(text))
		if len(runErrs) > 0 {
			log.Printf("Test line errors: %v", runErrs)
			errCount++
			if errCount > maxErrors {
				return errors.New("too many errors")
			}
			continue
		}

		errCount = 0
		numLines++
		log.Printf("Output: %s", text)
		lg.tmpFile.WriteString(text)
		lg.tmpFile.WriteString("\n")
		date = date.Add(delta)
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

type noopHint struct{}

func (_ noopHint) String() string { return "noop" }
func (_ noopHint) Quality() int   { return 0 }
func (_ noopHint) Generate(rng *rand.Rand, t time.Time) string {
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
func (_ constant) Quality() int   { return 1 }
func (c constant) Generate(rng *rand.Rand, t time.Time) string {
	return parser.Constant(c).Value()
}

type date struct{}

func (date) String() string { return "date" }
func (date) Quality() int   { return 1 }
func (date) Generate(rng *rand.Rand, t time.Time) string {
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
			panic(errors.Errorf("unsupported format %%%v for date hint", comp.Spec()))
		}
	}
	return sb.String()
}

type mapKey []string

func newMapKey(dict map[string]int) mapKey {
	result := make(mapKey, 0, len(dict))
	for key := range dict {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func (c mapKey) String() string { return "value_map" }
func (mapKey) Quality() int     { return 10 }
func (c mapKey) Generate(rng *rand.Rand, t time.Time) string {
	return c[rng.Intn(len(c))]
}

func (c mapKey) Filter(expr strcat) (filtered mapKey) {
	for _, entry := range c {
		if dict := expr.Split(entry); dict != nil {
			filtered = append(filtered, entry)
		}
	}
	return filtered
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

type strcat []parser.Value

func (c strcat) String() string { return fmt.Sprintf("strcat(%+v)", []parser.Value(c)) }
func (_ strcat) Quality() int   { return 20 }
func (_ strcat) Generate(rng *rand.Rand, t time.Time) string {
	panic("strcat can't be generated directly")
}

func (c strcat) Split(s string) (kv map[string]string) {
	// This is similar to a dissect, but need to support consecutive "captures".
	// not consecutive constants as those are merged by the parser.
	// strcat(fl1, fl2)
	// strcat("_", fl1, fl2, "_", fl3)
	// strcat(fl1, "_", fl2, fl3)
	// strcat("_", fld1, "_")
	if len(c) == 0 {
		return nil
	}

	// Validate and remove trailing constant. This is to avoid an edge cases
	// like: strcat{fld1,'_',fld2,'_')
	//    s: "A_B_C_"
	// wanted: fld1=A fld2=B_C
	//    not: fld1=A fld2=B
	if lastCt, ok := c[len(c)-1].(parser.Constant); ok {
		pos := strings.LastIndex(s, lastCt.Value())
		if pos == -1 || pos+len(lastCt.Value()) != len(s) {
			return nil
		}
		s = s[:pos]
		c = c[:len(c)-1]
	}
	var values []string   // N values (the string between constants)
	var fields [][]string // N x M values (one or more fields per each value above)
	for pos := 0; pos < len(c); pos++ {
		switch v := c[pos].(type) {
		case parser.Constant:
			if strings.Index(s, v.Value()) != 0 {
				return nil
			}
			s = s[len(v.Value()):]
		case parser.Field:
			if len(s) == 0 {
				return nil
			}
			thisFields := []string{v.Name}
			nextCtPos := -1
			nextCtValue := ""
			for nextPos := pos + 1; nextCtPos == -1 && nextPos < len(c); nextPos++ {
				switch next := c[nextPos].(type) {
				case parser.Field:
					thisFields = append(thisFields, next.Name)
				case parser.Constant:
					nextCtPos = nextPos
					nextCtValue = next.Value()
				}
			}
			fields = append(fields, thisFields)
			if nextCtPos != -1 {
				// Need at least one character per captured field
				if len(s) < len(thisFields) {
					return nil
				}
				end := strings.Index(s[len(thisFields):], nextCtValue)
				if end == -1 {
					return nil
				}
				end += len(thisFields)
				values = append(values, s[:end])
				s = s[end+len(nextCtValue):]
				pos = nextCtPos
			} else {
				values = append(values, s)
				s = ""
				pos = len(c)
			}
		default:
			panic(fmt.Sprintf("bad item in strcat expression: %T (%v)", v, v))
		}
	}
	//if len(s) > 0 {
	//	values = append(values, s)
	//}
	if len(s) > 0 || len(values) != len(fields) || len(fields) == 0 {
		panic(fmt.Sprintf("unexpected %+q vs %+q", values, fields))
	}
	kv = make(map[string]string)
	for idx, fs := range fields {
		value := values[idx]
		if len(fs) == 1 {
			kv[fs[0]] = value
		} else {
			div := len(value) / len(fs)
			rem := len(value) % len(fs)
			for i, f := range fs {
				end := div
				if i == 0 {
					end += rem
				}
				kv[f] = value[:end]
				value = value[end:]
			}
		}
	}
	return kv
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

var removeWhitespace = regexp.MustCompile(" +")

func (lc *lineComposer) Build() (string, error) {
	// Try to balance hinting captures with actual captures for each field.
	fldCaptures := make(map[string]int)
	for _, act := range lc.expression {
		switch v := act.(type) {
		case parser.Field:
			fldCaptures[v.Name]++
		}
	}
	var cap captured
	for fld, hints := range lc.knownFields {
		hintCaptures := 0
		for _, hint := range hints {
			if hint == cap {
				hintCaptures++
			}
		}
		actualCaptures := fldCaptures[fld]
		switch {
		case actualCaptures == 0:
			// Fields set during actions
			continue

		case actualCaptures == hintCaptures:
			continue

		case fldCaptures[fld] == 1 && hintCaptures > 1:
			// This happens due to overlap, when a field is hinted twice once
			// in the header and once in the overlapped message.
			// When a message is overlapped we don't yet have enough information
			// to fix this, as we don't know which capture is more interesting
			// or if the field is going to be repeated.

			// Replace all hints with noops to ensure all hints are considered.
			var repl valueHint = captured{}
			for idx, hint := range hints {
				if hint == cap {
					hints[idx] = repl
					repl = noopHint{}
				}
			}
			continue
		}
		return "", fmt.Errorf("bad hinting for field '%s'. hintCaptures=%d, actual=%d, hints=%v", fld, hintCaptures, actualCaptures, hints)
	}

	// Build the final log message.
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
	return removeWhitespace.ReplaceAllString(sb.String(), " "), nil
}

func (lc *lineComposer) valueFor(field string) (value string, err error) {
	hints, ok := lc.knownFields[field]
	if !ok || len(hints) == 0 {
		return "", errors.Errorf("field %s not captured", field)
	}
	var capt captured
	// Discard information prior to capture
	for len(hints) > 0 && hints[0] != capt {
		hints = hints[1:]
	}
	if len(hints) == 0 {
		return "", errors.Errorf("field %s not captured", field)
	}
	best := hints[0]
	defer func() {
		log.Printf("valueFor('%s') hints:%+v best=%+v result='%s'", field, hints, best, value)
	}()

	idx := 1
	for ; idx < len(hints); idx++ {
		hint := hints[idx]
		if hint == capt {
			break
		}
		if best.Quality() < hint.Quality() {
			best = hint
		}
	}
	// Discard used hints
	lc.knownFields[field] = hints[idx:]
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
		//return "", errors.New("no default generator for field")
		gen = makeText
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
	log.Printf("Path: %+v", lc.history)
}

func (lg *logs) newLine(p parser.Parser, t time.Time) (string, error) {
	state := lineComposer{
		time:        t,
		parser:      p,
		rng:         lg.rng,
		fieldsGen:   lg.fieldsGen,
		knownFields: make(fieldHints),
	}
	if err := state.randomWalk(p.Root); err != nil {
		return "", errors.Wrapf(err, "error during random walk (historic:%+v)", state.history)
	}
	state.Log()
	return state.Build()
}

func (lc *lineComposer) randomWalk(node parser.Operation) error {
	switch v := node.(type) {
	case parser.Chain:
		for _, child := range v.Children() {
			if err := lc.randomWalk(child); err != nil {
				return err
			}
		}

	case parser.LinearSelect:
		children := v.Children()
		return lc.randomWalk(children[lc.rng.Intn(len(children))])

	case parser.Match:
		lc.history = append(lc.history, v.ID)
		if err := lc.appendPattern(v.Pattern); err != nil {
			return err
		}
		if err := lc.appendActions(v.OnSuccess); err != nil {
			return err
		}

	case parser.MsgIdSelect:
		msgID, err, basePath := "", errBadOverlap, lc.history
		// Repeat until a message with valid overlap is found
		for iter := 1; iter < 2 && err == errBadOverlap; iter++ {
			lc.history = basePath
			msgID, err = lc.composeMessageID(v)
			if err != nil {
				return err
			}
			idx, ok := v.Map[msgID]
			if !ok {
				return errors.Errorf("No messages for messageid '%s'", msgID)
			}
			err = lc.randomWalk(v.Children()[idx])
		}
		if err != nil {
			return err
		}

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

func (lc *lineComposer) getAssignedValue(fieldName string) (string, bool) {
	hints := lc.knownFields[fieldName]
	if len(hints) == 2 {
		_, isCaptured := hints[0].(captured)
		ct, isConstant := hints[1].(constant)
		if isCaptured && isConstant {
			return ct.Generate(lc.rng, lc.time), true
		}
	}
	return "", false
}

func (lc *lineComposer) assignValue(field, value string) {
	lc.knownFields[field] = []valueHint{captured{}, constant(value)}
}

func (lc *lineComposer) composeMessageID(node parser.MsgIdSelect) (msgID string, err error) {
	hints := lc.knownFields["messageid"]
	log.Printf("MessageID hints: %+v", hints)
	defer func() {
		log.Printf("MessageID result: %+v", lc.knownFields["messageid"])
	}()
	if len(hints) != 1 {
		if value, ok := lc.getAssignedValue("messageid"); ok {
			return value, nil
		}
		return "", errors.Errorf("bad number of hints for messageid: %+v", hints)
	}
	switch v := hints[0].(type) {
	case constant:
		msgID = parser.Constant(v).Value()
	case captured:
		// Let's just select a messageID at random
		msgID = newMapKey(node.Map).Generate(lc.rng, lc.time)
	case strcat:
		// Compose a messageid from an expression like:
		// strcat([Field(msgIdPart1) Constant('_') Field(msgIdPart2) Constant('_') Field(msgIdPart3)])
		matching := newMapKey(node.Map).Filter(v)
		if len(matching) == 0 {
			return "", errors.Errorf("no messageids match strcat pattern %+v", v)
		}
		msgID = matching.Generate(lc.rng, lc.time)
		kv := v.Split(msgID)
		if kv == nil {
			return "", errors.Errorf("strcat pattern %v doesn't split '%s'", v, msgID)
		}
		// set the appropriate msgIdPartN fields so that the STRCAT operation
		// generates the expected messageid
		log.Printf("Generated messageid '%s'", msgID)
		log.Printf("^ for pattern '%+v'", v)
		for field, value := range kv {
			log.Printf("^ with '%s'='%s'", field, value)
			lc.assignValue(field, value)
		}
	default:
		return "", errors.Errorf("don't know how to generate a messageid from hints=%+v", hints)
	}
	lc.assignValue("messageid", msgID)
	return msgID, nil
}

var errBadOverlap = errors.New("bad overlap")

func (lc *lineComposer) appendPattern(p parser.Pattern) (err error) {
	if lc.payload != nil {
		if p, err = lc.resolveAlternatives(p); err != nil {
			return err
		}
		if p = lc.applyOverlap(p); p == nil {
			return errBadOverlap
		}
	}

	for idx, entry := range p {
		switch v := entry.(type) {
		case parser.Field:
			// mark field as captured
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
				if len(loc) == 1 {
					lc.payload = &loc[0]
				} else if len(loc) == 0 && v.Name == "$START" {
					lc.payload = new(int) // payload pos is zero
				} else {
					return errors.Errorf("payload field must appear once. historic=%+v loc=%+v pattern=%s",
						lc.history, loc, p.String())
				}
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

func (lc *lineComposer) resolveAlternatives(p parser.Pattern) (result parser.Pattern, err error) {
	for _, entry := range p {
		switch v := entry.(type) {
		case parser.Field, parser.Constant:
			result = append(result, entry)

		case parser.Alternatives:
			part, err := lc.resolveAlternatives(v[lc.rng.Intn(len(v))])
			if err != nil {
				return nil, err
			}
			result = append(result, part...)
		default:
			return nil, errors.Errorf("no support for %T in message pattern", v)
		}
	}
	return result, nil
}

func (lc *lineComposer) applyOverlap(p parser.Pattern) parser.Pattern {
	if lc.payload == nil {
		return p
	}
	loc := *lc.payload
	overlap := len(lc.expression) - loc
	if overlap <= 0 {
		panic("bad overlap")
	}

	// Build the overlapped pattern from the header-side, replacing any fixed
	// (messageid/msgIdPart1/etc.) fields with their expected values.

	var fromHeader parser.Pattern
	for _, item := range lc.expression[loc:] {
		if fld, ok := item.(parser.Field); ok {
			if value, ok := lc.getAssignedValue(fld.Name); ok {
				fromHeader = append(fromHeader, parser.Constant(value))
				continue
			}
		}
		fromHeader = append(fromHeader, item)
	}
	fromHeader = fromHeader.SquashConstants()
	log.Printf("overlap prev: %+v", lc.expression[loc:])
	log.Printf("overlap adj.: %+v", fromHeader)
	log.Printf("overlap next: %+v", p)

	p, fields, ok := lc.mergeOverlapped(fromHeader, p)
	if !ok {
		log.Printf("Overlap failed")
		return nil
	}
	log.Printf("Overlap success:")
	for k, v := range fields {
		log.Printf(" %s <- '%s'", k, v)
		lc.assignValue(k, v)
	}
	lc.expression = lc.expression[:loc]
	log.Printf("Overlap header: %+v", lc.expression)
	log.Printf("Overlap message: %+v", p)
	lc.payload = nil
	return p
}

type helper struct {
	p      parser.Pattern
	chrPos int
}

func getField(item parser.Value) parser.Field {
	fld, ok := item.(parser.Field)
	if !ok {
		panic("expected a field")
	}
	return fld
}

func getConstant(item parser.Value) parser.Constant {
	ct, ok := item.(parser.Constant)
	if !ok {
		panic("expected a constant")
	}
	return ct
}

func (h *helper) Len() int {
	return len(h.p)
}

func (h *helper) Current() (string, bool) {
	switch v := h.p[0].(type) {
	case parser.Constant:
		return v.Value()[h.chrPos:], true
	case parser.Field:
		return v.Name, false
	default:
		panic(v)
	}
}

func (h *helper) AdvanceChars(n int) {
	ct := getConstant(h.p[0])
	h.chrPos += n
	if h.chrPos > len(ct.Value()) {
		panic("overflow")
	}
	if h.chrPos == len(ct.Value()) {
		h.chrPos = 0
		h.p = h.p[1:]
	}
}

func (h *helper) AdvanceField() {
	_ = getField(h.p[0])
	h.p = h.p[1:]
}

func (h *helper) Anchor() string {
	_ = getField(h.p[0])
	if len(h.p) < 2 {
		return ""
	}
	ct := getConstant(h.p[1])
	return ct.Value()[:1]
}

func (h *helper) Capture(anchor string) (string, bool) {
	ct := getConstant(h.p[0])
	value := ct.Value()[h.chrPos:]
	if anchor == "" {
		return value, true
	}
	pos := strings.Index(value, anchor)
	if pos == -1 {
		return "", false
	}
	result := value[:pos]
	h.AdvanceChars(pos)
	return result, true
}

func (lc *lineComposer) mergeOverlapped(header, message parser.Pattern) (p parser.Pattern, vars map[string]string, merged bool) {
	vars = make(map[string]string)
	h := helper{
		p: header,
	}
	m := helper{
		p: message,
	}
	for h.Len() > 0 && m.Len() > 0 {
		hVal, hIsCt := h.Current()
		mVal, mIsCt := m.Current()
		log.Printf("Loop '%s'/%v '%s'/%v [%d/%d]", hVal, hIsCt, mVal, mIsCt, h.Len(), m.Len())
		switch {
		case hIsCt && mIsCt:
			ovr := constantOverlap(hVal, mVal)
			if ovr == 0 {
				return nil, nil, false
			}
			h.AdvanceChars(ovr)
			m.AdvanceChars(ovr)

		case hIsCt && !mIsCt:
			anchor := m.Anchor()
			//if anchor == "" {
			//	// TODO?
			//	return vars, true
			//}
			if h.Len() == 1 {
				vars[mVal] = hVal
				return message, vars, true
			}
			value, ok := h.Capture(anchor)
			if !ok {
				return nil, nil, false
			}
			vars[mVal] = value
			m.AdvanceField()

		case !hIsCt && mIsCt:
			anchor := h.Anchor()
			//if anchor == "" {
			//	// TODO?
			//	return vars, true
			//}
			_, ok := m.Capture(anchor)
			if !ok {
				return nil, nil, false
			}
			h.AdvanceField()

		case !hIsCt && !mIsCt:
			replace := strings.HasPrefix(mVal, "fld")
			log.Printf("Overlap: replace field %s with %s? %v", mVal, hVal, replace)
			if replace {
				fld := getField(m.p[0])
				fld.Name = hVal
				m.p[0] = fld
				// Drop the current hints for the replaced field, otherwise
				// it'll have double the hints as the overlapped pattern is
				// processed again
			}
			log.Printf("Overlap: replaced: %v", message)
			m.AdvanceField()
			h.AdvanceField()
			// TODO
		}
	}
	return message, vars, true
}

func constantOverlap(a, b string) int {
	if a > b {
		a, b = b, a
	}
	n := len(a)
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func (lc *lineComposer) addHint(field string, hint valueHint) {
	lc.knownFields[field] = append(lc.knownFields[field], hint)
}

func (lc *lineComposer) appendActions(list parser.OpList) error {
	for _, act := range list {
		switch v := act.(type) {
		case parser.SetField:
			switch vv := v.Value[0].(type) {
			case parser.Field:
				lc.addHint(v.Target, copyField(vv))
			case parser.Constant:
				lc.addHint(v.Target, constant(vv))
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
				lc.addHint(fld.Name, newMapKey(vm.Mappings))
			}

		case parser.URLExtract:
			lc.addHint(v.Target, urlComponent(v.Component))
			lc.addHint(v.Source, url{})

		case parser.Call:
			// Only care about calls that set messageid
			if v.Target != "messageid" {
				continue
			}
			if v.Function != "STRCAT" {
				return errors.Errorf("unsupported function to set messageid: %s", v.Function)
			}
			lc.addHint(v.Target, strcat(v.Args))

		default:
			return errors.Errorf("unsupported action type %T: %s", v, v.Hashable())
		}
	}
	return nil
}

func (lc *lineComposer) enrichFromDateTime(dt parser.DateTime) (err error) {
	lc.addHint(dt.Target, date{})
	fields := dt.Fields
	var hints map[string]valueHint
	for _, fmt := range dt.Formats {
		if hints, err = hintsFromDateTimePattern(fmt, fields, nil); err == nil {
			for fld, hint := range hints {
				lc.addHint(fld, hint)
			}
			break
		}
	}
	return err
}

func hintsFromDateTimePattern(fmt []parser.DateTimeItem, fields []string, hints map[string]valueHint) (map[string]valueHint, error) {
	if hints == nil {
		hints = make(map[string]valueHint, len(fields))
	}
	switch {
	case len(fields) == 1: // 1 to many
		hints[fields[0]] = dateComponent(fmt)

	case len(fields) == len(fmt): // 1:1 mapping
		for idx, src := range fields {
			hints[src] = dateComponent(fmt[idx : idx+1])
		}

	case len(fields) < len(fmt): // Split fields, at least 1 fmt per fld
		// Try to split the fmt in two parts, one for date, one for time.
		if dtSplit := splitPatternDateAndTime(fmt); dtSplit != -1 {
			if len(fields) == 2 {
				hints[fields[0]] = dateComponent(fmt[:dtSplit])
				hints[fields[1]] = dateComponent(fmt[dtSplit:])
				break
			}
			multifields := map[string]struct{}{
				"date":  {},
				"hdate": {},
				"time":  {},
				"htime": {},
			}
			if _, found := multifields[fields[0]]; found {
				hints[fields[0]] = dateComponent(fmt[:dtSplit])
				return hintsFromDateTimePattern(fmt[dtSplit:], fields[1:], hints)
			}
			if _, found := multifields[fields[len(fields)-1]]; found {
				hints[fields[len(fields)-1]] = dateComponent(fmt[dtSplit:])
				return hintsFromDateTimePattern(fmt[:dtSplit], fields[:len(fields)-1], hints)
			}
		}

		div := len(fmt) / len(fields)
		rem := len(fmt) % len(fields)
		pos := 0
		for _, fld := range fields {
			next := pos + div
			if pos == 0 {
				next += rem
			}
			hints[fld] = dateComponent(fmt[pos:next])
			pos = next
		}
	default:
		// Don't know how to split this. More fields than formats
		return nil, errors.Errorf("don't know how to split datetime %+v", fmt)
	}
	return hints, nil
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
func splitPatternDateAndTime(pattern []parser.DateTimeItem) int {
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
