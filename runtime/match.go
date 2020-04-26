//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package runtime

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/adriansr/nwdevice2filebeat/parser"
	"github.com/pkg/errors"
)

type match struct {
	pattern   [][]pattern
	onSuccess []Node
}

func (m match) String() string {
	var sb strings.Builder
	for _, chunk := range m.pattern {
		if len(chunk) == 1 {
			sb.WriteString(chunk[0].String())
		} else {
			sb.WriteString("{{{")
			sb.WriteString(chunk[0].String())
			for _, alt := range chunk[1:] {
				sb.WriteString("|||")
				sb.WriteString(alt.String())
			}
			sb.WriteString("}}}")
		}
	}
	return sb.String()
}

type element struct {
	value     []byte
	isCapture bool
	isPayload bool
}

type pattern []element

func (p pattern) String() string {
	var sb strings.Builder
	for _, item := range p {
		if item.isCapture {
			sb.WriteByte('<')
			if item.isPayload {
				sb.WriteByte('!')
			}
			sb.Write(item.value)
			sb.WriteByte('>')
		} else {
			if item.isCapture {
				sb.WriteString("<(!")
				sb.Write(item.value)
				sb.WriteString(")>")
			} else {
				sb.Write(item.value)
			}
		}
	}
	return sb.String()
}

func newPattern(input parser.Pattern) (output [][]pattern, err error) {
	output, err = newPatternInner(input, 0)
	if err != nil {
		return output, err
	}
	// look for the payload field
	numPayload := 0
	var payloadField *element
	for _, chunk := range output {
		for _, alt := range chunk {
			for idx, elem := range alt {
				if elem.isPayload {
					numPayload++
					payloadField = &alt[idx]
				}
			}
		}
	}
	if numPayload == 0 || payloadField == nil {
		return output, nil
	}
	if numPayload > 1 {
		return output, errors.New("more than one payload field found")
	}
	lastChunk := output[len(output)-1]
	if len(lastChunk) != 1 {
		// Need at least a single non-alternative chunk to contain the payload definition.
		return output, errors.New("last chunk is an alternative")
	}
	if len(lastChunk[0]) == 0 {
		// Need at least the payload definition in there
		return output, errors.New("last chunk is empty")
	}
	if !lastChunk[0][len(lastChunk[0])-1].isPayload {
		return output, errors.New("payload definition is not the last element in the pattern")
	}
	// Strip payload definition
	lastChunk[0] = lastChunk[0][:len(lastChunk[0])-1]
	if len(lastChunk[0]) == 0 {
		output = output[:len(output)-1]
	}
	if len(payloadField.value) == 0 {
		return output, nil
	}

	set := 0
	for _, chunk := range output {
		setIn := 0
		for _, alt := range chunk {
			for idx, elem := range alt {
				if elem.isCapture && bytes.Equal(elem.value, payloadField.value) {
					alt[idx].isPayload = true
					setIn++
				}
			}
		}
		if setIn > 0 {
			if set > 0 {
				return output, errors.New("payload field appears in more than one chunk")
			}
			if setIn != len(chunk) {
				// This means some paths through the pattern will not capture payload.
				return output, errors.New("not all alternatives contain the payload field")
			}
			set += setIn
		}
	}
	return output, nil
}

func newPatternInner(input parser.Pattern, depth int) (output [][]pattern, err error) {
	var current pattern
	gotPayload := false
	for _, elem := range input {
		var entry element
		if gotPayload {
			return nil, errors.New("payload is not the last component")
		}
		switch v := elem.(type) {
		case parser.Constant:
			entry.value = adjustConstant([]byte(v.Value()))
			current = append(current, entry)
		case parser.Field:
			entry.value = []byte(v.Name())
			entry.isCapture = true
			current = append(current, entry)
		case parser.Payload:
			if depth != 0 {
				return nil, errors.New("found a payload definition inside alternative")
			}
			entry.value = []byte(v.FieldName())
			//entry.isCapture = true
			entry.isPayload = true
			gotPayload = true
			current = append(current, entry)
		case parser.Alternatives:
			if depth != 0 {
				return nil, errors.New("found nested alternatives")
			}
			if len(current) > 0 {
				output = append(output, []pattern{current})
				current = nil
			}
			choices := make([]pattern, len(v))
			for idx, subpattern := range v {
				inner, err := newPatternInner(subpattern, depth+1)
				if err != nil {
					return nil, errors.Wrap(err, "when parsing alternative subpattern")
				}
				// numChunks will be 1 (only one sequence of captures with no nested alternatives
				numChunks := len(inner)
				if numChunks != 1 {
					return nil, errors.Errorf("output of subpattern is %d instead of 1", numChunks)
				}
				numAlternatives := len(inner[0])
				if numAlternatives != 1 {
					return nil, errors.Errorf("inner length of subpattern is %d instead of 1", numAlternatives)
				}
				choices[idx] = inner[0][0]
			}
			output = append(output, choices)

		default:
			return nil, errors.Errorf("Unknown type %T in pattern", v)
		}
	}
	if len(current) > 0 || len(output) == 0 {
		if current == nil {
			current = make(pattern, 0)
		}
		output = append(output, []pattern{current})
	}
	return output, nil
}

// adjustConstant strips all leading, trailing spaces and replaces all
// intermediate sequence of spaces with a single space.
func adjustConstant(str []byte) (modified []byte) {
	result := make([]byte, 0, len(str))
	space := true
	for _, chr := range str {
		if chr != ' ' {
			if space {
				if len(result) > 0 {
					result = append(result, ' ')
				}
				space = false
			}
			result = append(result, chr)
		} else {
			space = true
		}
	}
	if len(result) == 0 && space {
		result = append(result, ' ')
	}
	return result
}

type capture struct {
	field      []byte
	start, end int
}

type captures struct {
	fields  []capture
	payload int
}

var ErrNoMatch = errors.New("pattern didn't match")
var ErrMultiplePayload = errors.New("multiple payloads captured")

func (m *match) Run(ctx *Context) error {
	/*THIS*/ fmt.Fprintf(os.Stderr, "-> run \"%s\"\n", m.String())
	/*THIS*/ fmt.Fprintf(os.Stderr, " > msg ='%s'\n", ctx.Message)
	fullCapture := captures{
		payload: -1,
	}
	pos := 0
	for _, chunk := range m.pattern {
		var nextPos int
		for _, alt := range chunk {
			var partial captures
			/*THIS*/ fmt.Fprintf(os.Stderr, " ~> try: <<%s>>\n", ctx.Message[pos:])
			if nextPos, partial = matchPattern(ctx.Message, pos, alt); nextPos != -1 {
				/*THIS*/ fmt.Fprintf(os.Stderr, " <~ matched <<%s>> payload: %d nexPos=%d\n", alt.String(), partial.payload, nextPos)
				if partial.payload >= 0 {
					if fullCapture.payload != -1 {
						/*THIS*/ fmt.Fprintf(os.Stderr, " ! multiple payload!\n")
						return ErrMultiplePayload
					}
					fullCapture.payload = partial.payload
				}
				fullCapture.fields = append(fullCapture.fields, partial.fields...)
				break
			} else {
				/*THIS*/ fmt.Fprintf(os.Stderr, " <~ didn't match <<%s>> payload: %d\n", alt.String(), partial.payload)
			}
		}
		if nextPos == -1 {
			/*THIS*/ fmt.Fprintf(os.Stderr, "<- not matched\n")
			return ErrNoMatch
		}
		pos = nextPos
	}
	if len(fullCapture.fields) > 0 {
		for _, capture := range fullCapture.fields {
			key, value := string(capture.field), string(ctx.Message[capture.start:capture.end])
			/*THIS*/ fmt.Fprintf(os.Stderr, " + captured '%s'='%s'\n", key, value)
			ctx.Fields.Put(key, value)
		}
	} else {
		/*THIS*/ fmt.Fprintf(os.Stderr, " ? captured zero fields\n")
	}
	// Not sure about references to $MSG in function calls. Should it wait to
	// update ctx.Message after functions applied?
	for _, act := range m.onSuccess {
		if err := act.Run(ctx); err != nil {
			ctx.Errors = append(ctx.Errors, err)
		}
	}
	if fullCapture.payload >= 0 {
		ctx.Message = ctx.Message[fullCapture.payload:]
		/*THIS*/ fmt.Fprintf(os.Stderr, " + payload ='%s'\n", ctx.Message)
	} else {
		ctx.Message = ctx.Message[pos:]
	}
	return nil
}

func matchPattern(msg []byte, pos int, pattern pattern) (nextPos int, captured captures) {
	captured.payload = -1
	msgPos, msgLen := pos, len(msg)
	itemIdx, numItems := 0, len(pattern)
	if !pattern[itemIdx].isCapture {
		if msgPos = skipConstant(msg, msgPos, pattern[itemIdx].value); msgPos == -1 {
			return msgPos, captures{}
		}
		//fmt.Fprintf(os.Stderr, "skipConstant: msg=%s pos=%d(%s)\n", msg, msgPos, msg[:msgPos])
		itemIdx++
	}
	for itemIdx < numItems {
		// Fetch position of next constant
		if !pattern[itemIdx].isCapture {
			panic("this is by definition a capture, but isn't")
		}
		if pattern[itemIdx].isPayload {
			captured.payload = msgPos
		}
		nextCt := itemIdx + 1
		if nextCt >= numItems {
			if len(pattern[itemIdx].value) > 0 {
				for ; msgPos < msgLen && msg[msgPos] == ' '; msgPos++ {
				}
				captured.fields = append(captured.fields, capture{
					field: pattern[itemIdx].value,
					start: msgPos,
					end:   msgLen,
				})
			}
			return msgLen, captured
		}
		if pattern[nextCt].isCapture {
			panic("here")
		}
		if len(pattern[nextCt].value) == 0 {
			panic("there")
		}
		start, end := findConstant(msg, msgPos, pattern[nextCt].value)
		if start == -1 {
			return -1, captures{}
		}
		//fmt.Fprintf(os.Stderr, "findConstant(<<%s>>): <<%s>> in <<%s>>\n", pattern[nextCt].value, msg[start:end], msg)
		if len(pattern[itemIdx].value) > 0 {
			for ; msgPos < start && msg[msgPos] == ' '; msgPos++ {
			}
			for ; start > msgPos && msg[start-1] == ' '; start-- {
			}
			captured.fields = append(captured.fields, capture{
				field: pattern[itemIdx].value,
				start: msgPos,
				end:   start,
			})
		}
		msgPos = end
		itemIdx = nextCt + 1
	}
	return msgPos, captured
}

func skipConstant(msg []byte, pos int, pattern []byte) int {
	n := len(msg)
	if pos >= n {
		return -1
	}
	if pattern[0] == ' ' {
		if msg[pos] != ' ' {
			return -1
		}
		pos++
		pattern = pattern[1:]
	}
	for ; pos < n && msg[pos] == ' '; pos++ {
	}
	for _, chr := range pattern {
		if pos >= n || msg[pos] != chr {
			return -1
		}
		pos++
		if chr == ' ' {
			for ; pos < n && msg[pos] == ' '; pos++ {
			}
		}
	}
	return pos
}

func findConstant(msg []byte, pos int, pattern []byte) (start, end int) {
	M := len(msg)
	P := len(pattern)
	if P == 0 {
		panic("zero P")
	}
	for {
		for ; pos < M && msg[pos] != pattern[0]; pos++ {
		}
		if pos+P > M {
			return -1, -1
		}
		start = pos
		pos++
		var k int
		for k = 1; k < P; k++ {
			if msg[pos] == pattern[k] {
				pos++
				if pattern[k] == ' ' {
					for ; pos < M && msg[pos] == ' '; pos++ {
					}
					if pos == M {
						return -1, -1
					}
				}
			} else {
				break
			}
		}
		for ; pos < M && msg[pos] == ' '; pos++ {
		}
		if k == P {
			return start, pos
		}
	}
}
