//line pattern.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

//line pattern.go:13
var _parse_pattern_eof_actions []byte = []byte{
	0, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 7, 8,
}

const parse_pattern_start int = 13
const parse_pattern_first_final int = 13
const parse_pattern_error int = 0

const parse_pattern_en_main int = 13

//line pattern.go.rl:14

var ErrBadPattern = errors.New("malformed pattern")

// ParsePattern parses a device log parser pattern.
func ParsePattern(data string) (pattern Pattern, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)
	mark := -1

	isPayload := false

//line pattern.go:39
	{
		cs = parse_pattern_start
	}

//line pattern.go:44
	{
		if (p) == (pe) {
			goto _test_eof
		}
		if cs == 0 {
			goto _out
		}
	_resume:
		switch cs {
		case 13:
			if data[(p)] == 60 {
				goto tr20
			}
			goto tr19
		case 14:
			if data[(p)] == 60 {
				goto tr22
			}
			goto tr21
		case 1:
			switch data[(p)] {
			case 33:
				goto tr0
			case 60:
				goto tr3
			case 95:
				goto tr2
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr2
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr2
				}
			default:
				goto tr2
			}
			goto tr1
		case 0:
			goto _out
		case 2:
			if data[(p)] == 112 {
				goto tr4
			}
			goto tr1
		case 3:
			if data[(p)] == 97 {
				goto tr5
			}
			goto tr1
		case 4:
			if data[(p)] == 121 {
				goto tr6
			}
			goto tr1
		case 5:
			if data[(p)] == 108 {
				goto tr7
			}
			goto tr1
		case 6:
			if data[(p)] == 111 {
				goto tr8
			}
			goto tr1
		case 7:
			if data[(p)] == 97 {
				goto tr9
			}
			goto tr1
		case 8:
			if data[(p)] == 100 {
				goto tr10
			}
			goto tr1
		case 9:
			switch data[(p)] {
			case 58:
				goto tr12
			case 62:
				goto tr13
			}
			goto tr11
		case 10:
			if data[(p)] == 95 {
				goto tr14
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr14
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr14
				}
			default:
				goto tr14
			}
			goto tr1
		case 11:
			switch data[(p)] {
			case 62:
				goto tr16
			case 95:
				goto tr15
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr15
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr15
				}
			default:
				goto tr15
			}
			goto tr11
		case 12:
			switch data[(p)] {
			case 62:
				goto tr18
			case 95:
				goto tr17
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr17
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr17
				}
			default:
				goto tr17
			}
			goto tr11
		}

	tr1:
		cs = 0
		goto _again
	tr11:
		cs = 0
		goto f0
	tr20:
		cs = 1
		goto f1
	tr22:
		cs = 1
		goto f8
	tr0:
		cs = 2
		goto _again
	tr4:
		cs = 3
		goto _again
	tr5:
		cs = 4
		goto _again
	tr6:
		cs = 5
		goto _again
	tr7:
		cs = 6
		goto _again
	tr8:
		cs = 7
		goto _again
	tr9:
		cs = 8
		goto _again
	tr10:
		cs = 9
		goto _again
	tr12:
		cs = 10
		goto f2
	tr15:
		cs = 11
		goto _again
	tr14:
		cs = 11
		goto f1
	tr17:
		cs = 12
		goto _again
	tr2:
		cs = 12
		goto f1
	tr13:
		cs = 13
		goto f3
	tr16:
		cs = 13
		goto f4
	tr18:
		cs = 13
		goto f5
	tr3:
		cs = 14
		goto _again
	tr19:
		cs = 14
		goto f1
	tr21:
		cs = 14
		goto f8

	f1:
//line pattern.go.rl:26

		mark = p

		goto _again
	f5:
//line pattern.go.rl:40

		//fmt.Fprintf(os.Stderr, "XXX: capture_field at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
		if !isPayload {
			pattern = append(pattern, Field(data[mark:p]))
		} else {
			pattern = append(pattern, Payload(Field(data[mark:p])))
			isPayload = false
		}

		goto _again
	f0:
//line pattern.go.rl:52

		mark = eof - p
		if mark > 20 {
			mark = 20
		}
		if mark > 0 {
			err = errors.Errorf("malformed pattern at position %d near '%s'", p, data[p:p+mark])
		} else {
			err = errors.Errorf("malformed pattern at position %d (EOF)", p)
		}

		goto _again
	f2:
//line pattern.go.rl:63

		isPayload = true

		goto _again
	f8:
//line pattern.go.rl:34

		//fmt.Fprintf(os.Stderr, "XXX: capture_constant at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
		if mark < p {
			pattern = append(pattern, Constant(data[mark:p]))
		}

//line pattern.go.rl:26

		mark = p

		goto _again
	f4:
//line pattern.go.rl:40

		//fmt.Fprintf(os.Stderr, "XXX: capture_field at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
		if !isPayload {
			pattern = append(pattern, Field(data[mark:p]))
		} else {
			pattern = append(pattern, Payload(Field(data[mark:p])))
			isPayload = false
		}

//line pattern.go.rl:66

		if isPayload {
			pattern = append(pattern, Payload(Field("")))
			isPayload = false
		}

		goto _again
	f3:
//line pattern.go.rl:63

		isPayload = true

//line pattern.go.rl:66

		if isPayload {
			pattern = append(pattern, Payload(Field("")))
			isPayload = false
		}

		goto _again

	_again:
		if cs == 0 {
			goto _out
		}
		if (p)++; (p) != (pe) {
			goto _resume
		}
	_test_eof:
		{
		}
		if (p) == eof {
			switch _parse_pattern_eof_actions[cs] {
			case 7:
//line pattern.go.rl:49

				err = nil

			case 1:
//line pattern.go.rl:52

				mark = eof - p
				if mark > 20 {
					mark = 20
				}
				if mark > 0 {
					err = errors.Errorf("malformed pattern at position %d near '%s'", p, data[p:p+mark])
				} else {
					err = errors.Errorf("malformed pattern at position %d (EOF)", p)
				}

			case 8:
//line pattern.go.rl:34

				//fmt.Fprintf(os.Stderr, "XXX: capture_constant at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
				if mark < p {
					pattern = append(pattern, Constant(data[mark:p]))
				}

//line pattern.go.rl:49

				err = nil

//line pattern.go:338
			}
		}

	_out:
		{
		}
	}

//line pattern.go.rl:85

	if err != nil {
		return nil, err
	}
	// TODO: Please fix this hack.
	//       The state machine above outputs single-char Constants one after the
	//       other. This joins consecutive Constants into one.
	nn := len(pattern)
	isConstant := func(v Value) bool {
		_, ok := v.(Constant)
		return ok
	}
	var out Pattern
	for i := 0; i < nn; i++ {
		if isConstant(pattern[i]) {
			next := i + 1
			for ; next < nn && isConstant(pattern[next]); next++ {
				pattern[i] = Constant(string(pattern[i].(Constant)) + string(pattern[next].(Constant)))
			}
			out = append(out, pattern[i])
			i = next - 1
		} else {
			out = append(out, pattern[i])
		}
	}
	return out, nil
}

/*
   escaped_lt = "<<";
   pattern_chars = escaped_lt | (any -- "<");
   field_chars = [A-Za-z_0-9];
   constant = pattern_chars** >mark %capture_constant;
   field_name = field_chars+ >mark %capture_field;
   payload_custom = ":" field_chars+ >mark %capture_field;
   payload_decl = "!payload" %enter_payload payload_custom? %leave_payload;
   field = "<" (payload_decl | field_name) ">";
   pattern = ( field | constant ) ;

   main := pattern* %commit $!onerror;

*/
