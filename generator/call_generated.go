//line call.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

//line call_generated.go:13
var _parse_call_eof_actions []byte = []byte{
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 6,
}

const parse_call_start int = 1
const parse_call_first_final int = 10
const parse_call_error int = 0

const parse_call_en_main int = 1

//line call.go.rl:14

var ErrBadCall = errors.New("malformed function call")

// ParseCall parses a function call.
// Input: "STRCAT('header_', msgIdPart2)"
// Output: Call(Function:"STRCAT", Args: [ Constant("header_"), Field("msgIdPart2")])
func ParseCall(data string) (pCall *Call, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)
	start := -1
	err = ErrBadCall

	var call Call

//line call_generated.go:42
	{
		cs = parse_call_start
	}

//line call_generated.go:47
	{
		if (p) == (pe) {
			goto _test_eof
		}
		if cs == 0 {
			goto _out
		}
	_resume:
		switch cs {
		case 1:
			switch data[(p)] {
			case 32:
				goto tr0
			case 95:
				goto tr2
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr0
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr2
					}
				case data[(p)] >= 65:
					goto tr2
				}
			default:
				goto tr2
			}
			goto tr1
		case 0:
			goto _out
		case 2:
			switch data[(p)] {
			case 32:
				goto tr3
			case 40:
				goto tr4
			case 95:
				goto tr5
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr3
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr5
					}
				case data[(p)] >= 65:
					goto tr5
				}
			default:
				goto tr5
			}
			goto tr1
		case 3:
			switch data[(p)] {
			case 32:
				goto tr6
			case 40:
				goto tr7
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr6
			}
			goto tr1
		case 4:
			switch data[(p)] {
			case 32:
				goto tr7
			case 36:
				goto tr8
			case 39:
				goto tr9
			case 95:
				goto tr8
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr7
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr8
					}
				case data[(p)] >= 65:
					goto tr8
				}
			default:
				goto tr8
			}
			goto tr1
		case 5:
			switch data[(p)] {
			case 32:
				goto tr10
			case 36:
				goto tr11
			case 41:
				goto tr12
			case 44:
				goto tr13
			case 95:
				goto tr11
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr10
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr11
					}
				case data[(p)] >= 65:
					goto tr11
				}
			default:
				goto tr11
			}
			goto tr1
		case 6:
			switch data[(p)] {
			case 32:
				goto tr14
			case 41:
				goto tr15
			case 44:
				goto tr7
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr14
			}
			goto tr1
		case 10:
			if data[(p)] == 32 {
				goto tr15
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr15
			}
			goto tr1
		case 7:
			switch data[(p)] {
			case 39:
				goto tr17
			case 92:
				goto tr18
			}
			goto tr16
		case 8:
			switch data[(p)] {
			case 39:
				goto tr20
			case 92:
				goto tr21
			}
			goto tr19
		case 9:
			switch data[(p)] {
			case 39:
				goto tr19
			case 92:
				goto tr19
			}
			goto tr1
		}

	tr1:
		cs = 0
		goto _again
	tr0:
		cs = 1
		goto _again
	tr5:
		cs = 2
		goto _again
	tr2:
		cs = 2
		goto f0
	tr6:
		cs = 3
		goto _again
	tr3:
		cs = 3
		goto f1
	tr7:
		cs = 4
		goto _again
	tr4:
		cs = 4
		goto f1
	tr13:
		cs = 4
		goto f2
	tr11:
		cs = 5
		goto _again
	tr8:
		cs = 5
		goto f0
	tr14:
		cs = 6
		goto _again
	tr10:
		cs = 6
		goto f2
	tr17:
		cs = 6
		goto f3
	tr20:
		cs = 6
		goto f4
	tr9:
		cs = 7
		goto _again
	tr19:
		cs = 8
		goto _again
	tr16:
		cs = 8
		goto f0
	tr21:
		cs = 9
		goto _again
	tr18:
		cs = 9
		goto f0
	tr15:
		cs = 10
		goto _again
	tr12:
		cs = 10
		goto f2

	f0:
//line call.go.rl:37

		start = p

		goto _again
	f1:
//line call.go.rl:40

		call.Function = data[start:p]

		goto _again
	f4:
//line call.go.rl:43

		call.Args = append(call.Args, Constant(unescapeConstant(data[start:p])))

		goto _again
	f2:
//line call.go.rl:46

		call.Args = append(call.Args, Field(data[start:p]))

		goto _again
	f3:
//line call.go.rl:37

		start = p

//line call.go.rl:43

		call.Args = append(call.Args, Constant(unescapeConstant(data[start:p])))

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
			switch _parse_call_eof_actions[cs] {
			case 6:
//line call.go.rl:52

				err = nil

//line call_generated.go:302
			}
		}

	_out:
		{
		}
	}

//line call.go.rl:70

	if err != nil {
		return nil, err
	}
	return &call, nil
}
