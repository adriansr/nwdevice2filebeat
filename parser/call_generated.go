//line call.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import "github.com/pkg/errors"

//line call_generated.go:13
var _parse_call_eof_actions []byte = []byte{
	0, 0, 0, 0, 0, 0, 4,
}

const parse_call_start int = 1
const parse_call_first_final int = 6
const parse_call_error int = 0

const parse_call_en_main int = 1

//line call.go.rl:14

var ErrBadCall = errors.New("malformed function call")

// ParseCall is the first step on parsing a function call.
// Input: "STRCAT('header_', msgIdPart2)"
// Output: Call(Function:"STRCAT", Args: [ Constant("header_"), Field("msgIdPart2")])
func ParseCall(data string) (pCall *Call, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)
	start := -1
	err = ErrBadCall

	var call Call

//line call_generated.go:41
	{
		cs = parse_call_start
	}

//line call_generated.go:46
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
			switch data[(p)] {
			case 40:
				goto tr3
			case 95:
				goto tr4
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr4
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr4
				}
			default:
				goto tr4
			}
			goto tr1
		case 3:
			switch data[(p)] {
			case 41:
				goto tr6
			case 44:
				goto tr1
			}
			goto tr5
		case 4:
			switch data[(p)] {
			case 41:
				goto tr8
			case 44:
				goto tr9
			}
			goto tr7
		case 6:
			switch data[(p)] {
			case 32:
				goto tr10
			case 41:
				goto tr8
			case 44:
				goto tr9
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr10
			}
			goto tr7
		case 5:
			if data[(p)] == 44 {
				goto tr1
			}
			goto tr5
		}

	tr1:
		cs = 0
		goto _again
	tr0:
		cs = 1
		goto _again
	tr4:
		cs = 2
		goto _again
	tr2:
		cs = 2
		goto f0
	tr3:
		cs = 3
		goto f1
	tr7:
		cs = 4
		goto _again
	tr5:
		cs = 4
		goto f0
	tr9:
		cs = 5
		goto f2
	tr10:
		cs = 6
		goto _again
	tr6:
		cs = 6
		goto f0
	tr8:
		cs = 6
		goto f2

	f0:
//line call.go.rl:33

		start = p

		goto _again
	f1:
//line call.go.rl:36

		call.Function = data[start:p]

		goto _again
	f2:
//line call.go.rl:39

		call.Args = append(call.Args, disambiguateFieldOrConstant(data[start:p]))

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
			case 4:
//line call.go.rl:42

				err = nil

//line call_generated.go:180
			}
		}

	_out:
		{
		}
	}

//line call.go.rl:56

	if err != nil {
		return nil, err
	}
	return &call, nil
}
