//line call.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

//line call.go:11
var _parse_call_eof_actions []byte = []byte{
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 6,
}

const parse_call_start int = 1
const parse_call_first_final int = 11
const parse_call_error int = 0

const parse_call_en_main int = 1

//line call.rl:12

var ErrBadCall = errors.New("malformed function call")

// ParseCall parses a function call.
// Input: "STRCAT('header_', msgIdPart2)"
// Output: Call(Function:"STRCAT", Args: [ Constant("header_"), Field("msgIdPart2")])
func ParseCall(data string) (call Call, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)
	start := -1
	err = ErrBadCall

//line call.go:38
	{
		cs = parse_call_start
	}

//line call.go:43
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
			case 42:
				goto tr2
			case 95:
				goto tr3
			}
			switch {
			case data[(p)] > 13:
				if 65 <= data[(p)] && data[(p)] <= 90 {
					goto tr3
				}
			case data[(p)] >= 9:
				goto tr0
			}
			goto tr1
		case 0:
			goto _out
		case 2:
			if data[(p)] == 95 {
				goto tr3
			}
			if 65 <= data[(p)] && data[(p)] <= 90 {
				goto tr3
			}
			goto tr1
		case 3:
			switch data[(p)] {
			case 32:
				goto tr4
			case 40:
				goto tr5
			case 95:
				goto tr6
			}
			switch {
			case data[(p)] > 13:
				if 65 <= data[(p)] && data[(p)] <= 90 {
					goto tr6
				}
			case data[(p)] >= 9:
				goto tr4
			}
			goto tr1
		case 4:
			switch data[(p)] {
			case 32:
				goto tr7
			case 40:
				goto tr8
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr7
			}
			goto tr1
		case 5:
			switch data[(p)] {
			case 32:
				goto tr8
			case 39:
				goto tr9
			case 95:
				goto tr10
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr8
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr10
					}
				case data[(p)] >= 65:
					goto tr10
				}
			default:
				goto tr10
			}
			goto tr1
		case 6:
			switch data[(p)] {
			case 39:
				goto tr12
			case 92:
				goto tr13
			}
			goto tr11
		case 7:
			switch data[(p)] {
			case 39:
				goto tr15
			case 92:
				goto tr16
			}
			goto tr14
		case 8:
			switch data[(p)] {
			case 32:
				goto tr17
			case 41:
				goto tr18
			case 44:
				goto tr8
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr17
			}
			goto tr1
		case 11:
			if data[(p)] == 32 {
				goto tr18
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr18
			}
			goto tr1
		case 9:
			switch data[(p)] {
			case 39:
				goto tr14
			case 92:
				goto tr14
			}
			goto tr1
		case 10:
			switch data[(p)] {
			case 32:
				goto tr19
			case 41:
				goto tr20
			case 44:
				goto tr21
			case 95:
				goto tr22
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr19
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr22
					}
				case data[(p)] >= 65:
					goto tr22
				}
			default:
				goto tr22
			}
			goto tr1
		}

	tr1:
		cs = 0
		goto _again
	tr0:
		cs = 1
		goto _again
	tr2:
		cs = 2
		goto _again
	tr6:
		cs = 3
		goto _again
	tr3:
		cs = 3
		goto f0
	tr7:
		cs = 4
		goto _again
	tr4:
		cs = 4
		goto f1
	tr8:
		cs = 5
		goto _again
	tr5:
		cs = 5
		goto f1
	tr21:
		cs = 5
		goto f4
	tr9:
		cs = 6
		goto _again
	tr14:
		cs = 7
		goto _again
	tr11:
		cs = 7
		goto f0
	tr17:
		cs = 8
		goto _again
	tr12:
		cs = 8
		goto f2
	tr15:
		cs = 8
		goto f3
	tr19:
		cs = 8
		goto f4
	tr16:
		cs = 9
		goto _again
	tr13:
		cs = 9
		goto f0
	tr22:
		cs = 10
		goto _again
	tr10:
		cs = 10
		goto f0
	tr18:
		cs = 11
		goto _again
	tr20:
		cs = 11
		goto f4

	f0:
//line call.rl:33

		start = p

		goto _again
	f1:
//line call.rl:36

		call.Function = data[start:p]

		goto _again
	f3:
//line call.rl:39

		call.Args = append(call.Args, Constant(unescapeConstant(data[start:p])))

		goto _again
	f4:
//line call.rl:42

		call.Args = append(call.Args, Field(data[start:p]))

		goto _again
	f2:
//line call.rl:33

		start = p

//line call.rl:39

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
//line call.rl:45

				err = nil

//line call.go:287
			}
		}

	_out:
		{
		}
	}

//line call.rl:64

	return call, err
}
