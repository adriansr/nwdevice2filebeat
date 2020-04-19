//line alt.go.rl:1
//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

import "github.com/pkg/errors"

//line alt_generated.go:11
var _parse_alternatives_eof_actions []byte = []byte{
	0, 0, 0, 0, 0, 4, 6,
}

const parse_alternatives_start int = 5
const parse_alternatives_first_final int = 5
const parse_alternatives_error int = 0

const parse_alternatives_en_main int = 5

//line alt.go.rl:12

var errSplitAltFailed = errors.New("failed to split alternatives")

func splitAlternatives(data string) (out []interface{}, err error) {
	cs, p, pe, eof := 0, 0, len(data), len(data)

	start_plain, end_plain := 0, 0
	start_alt := 0
	err = errSplitAltFailed
	var alts []interface{}

//line alt_generated.go:37
	{
		cs = parse_alternatives_start
	}

//line alt_generated.go:42
	{
		if (p) == (pe) {
			goto _test_eof
		}
		if cs == 0 {
			goto _out
		}
	_resume:
		switch cs {
		case 5:
			if data[(p)] == 123 {
				goto tr8
			}
			goto tr1
		case 1:
			if data[(p)] == 123 {
				goto tr1
			}
			if 124 <= data[(p)] && data[(p)] <= 125 {
				goto tr2
			}
			goto tr0
		case 2:
			if data[(p)] == 124 {
				goto tr4
			}
			if 123 <= data[(p)] && data[(p)] <= 125 {
				goto tr2
			}
			goto tr3
		case 0:
			goto _out
		case 3:
			if 123 <= data[(p)] && data[(p)] <= 125 {
				goto tr2
			}
			goto tr5
		case 4:
			switch data[(p)] {
			case 123:
				goto tr2
			case 124:
				goto tr6
			case 125:
				goto tr7
			}
			goto tr5
		case 6:
			if data[(p)] == 123 {
				goto tr10
			}
			goto tr9
		}

	tr2:
		cs = 0
		goto _again
	tr8:
		cs = 1
		goto f4
	tr10:
		cs = 1
		goto f7
	tr3:
		cs = 2
		goto _again
	tr0:
		cs = 2
		goto f0
	tr4:
		cs = 3
		goto f1
	tr6:
		cs = 3
		goto f2
	tr5:
		cs = 4
		goto _again
	tr1:
		cs = 5
		goto _again
	tr9:
		cs = 5
		goto f6
	tr7:
		cs = 6
		goto f2

	f6:
//line alt.go.rl:29

		if len(alts) > 0 {
			out = append(out, alts)
			alts = nil
		}
		start_plain = p

		goto _again
	f4:
//line alt.go.rl:36

		end_plain = p

		goto _again
	f0:
//line alt.go.rl:46

		start_alt = p
		alts = nil

		goto _again
	f2:
//line alt.go.rl:50

		// TODO
		alts = append(alts, data[start_alt:p])
		start_alt = p + 1

		goto _again
	f7:
//line alt.go.rl:29

		if len(alts) > 0 {
			out = append(out, alts)
			alts = nil
		}
		start_plain = p

//line alt.go.rl:36

		end_plain = p

		goto _again
	f1:
//line alt.go.rl:39

		if start_plain < end_plain {
			out = append(out, data[start_plain:end_plain])
		}
		start_plain = p
		end_plain = p

//line alt.go.rl:50

		// TODO
		alts = append(alts, data[start_alt:p])
		start_alt = p + 1

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
			switch _parse_alternatives_eof_actions[cs] {
			case 4:
//line alt.go.rl:36

				end_plain = p

//line alt.go.rl:39

				if start_plain < end_plain {
					out = append(out, data[start_plain:end_plain])
				}
				start_plain = p
				end_plain = p

//line alt.go.rl:26

				err = nil

			case 6:
//line alt.go.rl:29

				if len(alts) > 0 {
					out = append(out, alts)
					alts = nil
				}
				start_plain = p

//line alt.go.rl:36

				end_plain = p

//line alt.go.rl:39

				if start_plain < end_plain {
					out = append(out, data[start_plain:end_plain])
				}
				start_plain = p
				end_plain = p

//line alt.go.rl:26

				err = nil

//line alt_generated.go:223
			}
		}

	_out:
		{
		}
	}

//line alt.go.rl:77

	/*if err == errSplitAltFailed {
	    out = nil
	}*/
	return
}
