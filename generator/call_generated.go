//line call.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

//line call_generated.go:13
var _parse_call_eof_actions []byte = []byte{
	0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 6, 7,
	9, 9, 9, 9, 9,
}

const parse_call_start int = 1
const parse_call_first_final int = 14
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

//line call_generated.go:43
	{
		cs = parse_call_start
	}

//line call_generated.go:48
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
			case 64:
				goto tr4
			case 95:
				goto tr3
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
						goto tr3
					}
				case data[(p)] >= 65:
					goto tr3
				}
			default:
				goto tr3
			}
			goto tr1
		case 0:
			goto _out
		case 2:
			switch data[(p)] {
			case 32:
				goto tr0
			case 42:
				goto tr2
			case 95:
				goto tr3
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
						goto tr3
					}
				case data[(p)] >= 65:
					goto tr3
				}
			default:
				goto tr3
			}
			goto tr1
		case 3:
			if data[(p)] == 95 {
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
			goto tr1
		case 4:
			switch data[(p)] {
			case 32:
				goto tr5
			case 40:
				goto tr6
			case 95:
				goto tr7
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr5
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr7
					}
				case data[(p)] >= 65:
					goto tr7
				}
			default:
				goto tr7
			}
			goto tr1
		case 5:
			switch data[(p)] {
			case 32:
				goto tr8
			case 40:
				goto tr9
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr8
			}
			goto tr1
		case 6:
			switch data[(p)] {
			case 32:
				goto tr9
			case 36:
				goto tr10
			case 39:
				goto tr11
			case 95:
				goto tr10
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr9
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
		case 7:
			switch data[(p)] {
			case 32:
				goto tr12
			case 36:
				goto tr13
			case 41:
				goto tr14
			case 44:
				goto tr15
			case 95:
				goto tr13
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr12
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr13
					}
				case data[(p)] >= 65:
					goto tr13
				}
			default:
				goto tr13
			}
			goto tr1
		case 8:
			switch data[(p)] {
			case 32:
				goto tr16
			case 41:
				goto tr17
			case 44:
				goto tr9
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr16
			}
			goto tr1
		case 14:
			if data[(p)] == 32 {
				goto tr17
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr17
			}
			goto tr1
		case 9:
			switch data[(p)] {
			case 39:
				goto tr19
			case 92:
				goto tr20
			}
			goto tr18
		case 10:
			switch data[(p)] {
			case 39:
				goto tr22
			case 92:
				goto tr23
			}
			goto tr21
		case 11:
			switch data[(p)] {
			case 39:
				goto tr21
			case 92:
				goto tr21
			}
			goto tr1
		case 12:
			switch data[(p)] {
			case 36:
				goto tr24
			case 58:
				goto tr25
			case 95:
				goto tr24
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr24
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr24
				}
			default:
				goto tr24
			}
			goto tr1
		case 13:
			switch data[(p)] {
			case 36:
				goto tr26
			case 58:
				goto tr27
			case 95:
				goto tr26
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr26
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr26
				}
			default:
				goto tr26
			}
			goto tr1
		case 15:
			switch data[(p)] {
			case 32:
				goto tr29
			case 40:
				goto tr1
			case 42:
				goto tr30
			case 95:
				goto tr31
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr29
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr31
					}
				case data[(p)] >= 65:
					goto tr31
				}
			default:
				goto tr31
			}
			goto tr28
		case 16:
			if data[(p)] == 40 {
				goto tr1
			}
			goto tr32
		case 17:
			switch data[(p)] {
			case 32:
				goto tr33
			case 40:
				goto tr1
			case 42:
				goto tr34
			case 95:
				goto tr35
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr33
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr35
					}
				case data[(p)] >= 65:
					goto tr35
				}
			default:
				goto tr35
			}
			goto tr32
		case 18:
			switch data[(p)] {
			case 40:
				goto tr1
			case 95:
				goto tr35
			}
			switch {
			case data[(p)] < 65:
				if 48 <= data[(p)] && data[(p)] <= 57 {
					goto tr35
				}
			case data[(p)] > 90:
				if 97 <= data[(p)] && data[(p)] <= 122 {
					goto tr35
				}
			default:
				goto tr35
			}
			goto tr32
		case 19:
			switch data[(p)] {
			case 32:
				goto tr36
			case 40:
				goto tr6
			case 95:
				goto tr37
			}
			switch {
			case data[(p)] < 48:
				if 9 <= data[(p)] && data[(p)] <= 13 {
					goto tr36
				}
			case data[(p)] > 57:
				switch {
				case data[(p)] > 90:
					if 97 <= data[(p)] && data[(p)] <= 122 {
						goto tr37
					}
				case data[(p)] >= 65:
					goto tr37
				}
			default:
				goto tr37
			}
			goto tr32
		case 20:
			switch data[(p)] {
			case 32:
				goto tr38
			case 40:
				goto tr9
			}
			if 9 <= data[(p)] && data[(p)] <= 13 {
				goto tr38
			}
			goto tr32
		}

	tr1:
		cs = 0
		goto _again
	tr0:
		cs = 2
		goto _again
	tr2:
		cs = 3
		goto _again
	tr7:
		cs = 4
		goto _again
	tr3:
		cs = 4
		goto f0
	tr8:
		cs = 5
		goto _again
	tr5:
		cs = 5
		goto f1
	tr9:
		cs = 6
		goto _again
	tr6:
		cs = 6
		goto f1
	tr15:
		cs = 6
		goto f2
	tr13:
		cs = 7
		goto _again
	tr10:
		cs = 7
		goto f0
	tr16:
		cs = 8
		goto _again
	tr12:
		cs = 8
		goto f2
	tr19:
		cs = 8
		goto f3
	tr22:
		cs = 8
		goto f4
	tr11:
		cs = 9
		goto _again
	tr21:
		cs = 10
		goto _again
	tr18:
		cs = 10
		goto f0
	tr23:
		cs = 11
		goto _again
	tr20:
		cs = 11
		goto f0
	tr4:
		cs = 12
		goto _again
	tr26:
		cs = 13
		goto _again
	tr24:
		cs = 13
		goto f0
	tr17:
		cs = 14
		goto _again
	tr14:
		cs = 14
		goto f2
	tr27:
		cs = 15
		goto _again
	tr25:
		cs = 15
		goto f0
	tr32:
		cs = 16
		goto _again
	tr28:
		cs = 16
		goto f7
	tr33:
		cs = 17
		goto _again
	tr29:
		cs = 17
		goto f7
	tr34:
		cs = 18
		goto _again
	tr30:
		cs = 18
		goto f7
	tr37:
		cs = 19
		goto _again
	tr35:
		cs = 19
		goto f0
	tr31:
		cs = 19
		goto f7
	tr38:
		cs = 20
		goto _again
	tr36:
		cs = 20
		goto f1

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
	f7:
//line call.go.rl:49

		call.Target = data[start : p-1]

//line call.go.rl:37

		start = p

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

			case 9:
//line call.go.rl:55

				call.Function = "$set$"
				call.Args = []Value{Constant(data[start:p])}
				err = nil

			case 7:
//line call.go.rl:49

				call.Target = data[start : p-1]

//line call.go.rl:37

				start = p

//line call.go.rl:55

				call.Function = "$set$"
				call.Args = []Value{Constant(data[start:p])}
				err = nil

//line call_generated.go:565
			}
		}

	_out:
		{
		}
	}

//line call.go.rl:78

	if err != nil {
		return nil, err
	}
	return &call, nil
}
