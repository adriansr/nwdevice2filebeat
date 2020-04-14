//line pattern.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

//line pattern.go:13
var _parse_pattern_eof_actions []byte = []byte{
	1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 7, 1,
}

const parse_pattern_start int = 0
const parse_pattern_first_final int = 0
const parse_pattern_error int = -1

const parse_pattern_en_main int = 0

//line pattern.go.rl:14

var ErrBadPattern = errors.New("malformed pattern")

// ParsePattern parses a device log parser pattern.
func ParsePattern(data string) (pattern Pattern, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)
	mark := -1
	last := 0
	//err = ErrBadPattern

	isPayload := false

//line pattern.go:41
	{
		cs = parse_pattern_start
	}

//line pattern.go:46
	{
		if (p) == (pe) {
			goto _test_eof
		}
	_resume:
		switch cs {
		case 0:
			if data[(p)] == 60 {
				goto tr1
			}
			goto tr0
		case 1:
			switch data[(p)] {
			case 33:
				goto tr2
			case 60:
				goto tr1
			case 95:
				goto tr3
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr3
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr3
				}
			default:
				goto tr3
			}
			goto tr0
		case 2:
			switch data[(p)] {
			case 60:
				goto tr1
			case 112:
				goto tr4
			}
			goto tr0
		case 3:
			switch data[(p)] {
			case 60:
				goto tr1
			case 97:
				goto tr5
			}
			goto tr0
		case 4:
			switch data[(p)] {
			case 60:
				goto tr1
			case 121:
				goto tr6
			}
			goto tr0
		case 5:
			switch data[(p)] {
			case 60:
				goto tr1
			case 108:
				goto tr7
			}
			goto tr0
		case 6:
			switch data[(p)] {
			case 60:
				goto tr1
			case 111:
				goto tr8
			}
			goto tr0
		case 7:
			switch data[(p)] {
			case 60:
				goto tr1
			case 97:
				goto tr9
			}
			goto tr0
		case 8:
			switch data[(p)] {
			case 60:
				goto tr1
			case 100:
				goto tr10
			}
			goto tr0
		case 9:
			switch data[(p)] {
			case 58:
				goto tr11
			case 60:
				goto tr1
			case 62:
				goto tr12
			}
			goto tr0
		case 10:
			switch data[(p)] {
			case 60:
				goto tr1
			case 95:
				goto tr13
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr13
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr13
				}
			default:
				goto tr13
			}
			goto tr0
		case 11:
			switch data[(p)] {
			case 60:
				goto tr1
			case 62:
				goto tr15
			case 95:
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
			goto tr0
		case 12:
			if data[(p)] == 60 {
				goto tr17
			}
			goto tr16
		case 13:
			switch data[(p)] {
			case 60:
				goto tr1
			case 62:
				goto tr19
			case 95:
				goto tr18
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr18
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr18
				}
			default:
				goto tr18
			}
			goto tr0
		}

	tr0:
		cs = 0
		goto _again
	tr16:
		cs = 0
		goto f7
	tr1:
		cs = 1
		goto f1
	tr17:
		cs = 1
		goto f8
	tr2:
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
	tr11:
		cs = 10
		goto f3
	tr14:
		cs = 11
		goto _again
	tr13:
		cs = 11
		goto f2
	tr12:
		cs = 12
		goto f4
	tr15:
		cs = 12
		goto f5
	tr19:
		cs = 12
		goto f9
	tr18:
		cs = 13
		goto _again
	tr3:
		cs = 13
		goto f2

	f2:
//line pattern.go.rl:28

		mark = p

		goto _again
	f1:
//line pattern.go.rl:36

		if last < p {
			pattern = append(pattern, Constant(data[last:p]))
		}

		goto _again
	f7:
//line pattern.go.rl:41

		last = p

		goto _again
	f9:
//line pattern.go.rl:44

		if !isPayload {
			pattern = append(pattern, Field(data[mark:p]))
		} else {
			pattern = append(pattern, Payload(Field(data[mark:p])))
			isPayload = false
		}

		goto _again
	f3:
//line pattern.go.rl:70

		isPayload = true

		goto _again
	f8:
//line pattern.go.rl:41

		last = p

//line pattern.go.rl:36

		if last < p {
			pattern = append(pattern, Constant(data[last:p]))
		}

		goto _again
	f5:
//line pattern.go.rl:44

		if !isPayload {
			pattern = append(pattern, Field(data[mark:p]))
		} else {
			pattern = append(pattern, Payload(Field(data[mark:p])))
			isPayload = false
		}

//line pattern.go.rl:73

		if isPayload {
			pattern = append(pattern, Payload(Field("")))
			isPayload = false
		}

		goto _again
	f4:
//line pattern.go.rl:70

		isPayload = true

//line pattern.go.rl:73

		if isPayload {
			pattern = append(pattern, Payload(Field("")))
			isPayload = false
		}

		goto _again

	_again:
		if (p)++; (p) != (pe) {
			goto _resume
		}
	_test_eof:
		{
		}
		if (p) == eof {
			switch _parse_pattern_eof_actions[cs] {
			case 1:
//line pattern.go.rl:52

				if err == nil && last < p {
					pattern = append(pattern, Constant(data[last:p]))
					last = mark
				}
				//err = nil

			case 7:
//line pattern.go.rl:41

				last = p

//line pattern.go.rl:52

				if err == nil && last < p {
					pattern = append(pattern, Constant(data[last:p]))
					last = mark
				}
				//err = nil

//line pattern.go:348
			}
		}

	}

//line pattern.go.rl:93

	if err != nil {
		return nil, err
	}
	return pattern, nil
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

   main := pattern* %commit;

*/
