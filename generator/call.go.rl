//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

%%{
    machine parse_call;
    write data;
    variable p p;
    variable pe pe;
}%%

var ErrBadCall = errors.New("malformed function call")

// ParseCall is the first step on parsing a function call.
// Input: "STRCAT('header_', msgIdPart2)"
// Output: Call(Function:"STRCAT", Args: [ Constant("header_"), Field("msgIdPart2")])
func ParseCall(data string) (pCall *Call, err error) {
    cs, p, pe, eof := 0, 0, len(data), len(data)
    start := -1
    err = ErrBadCall;

    var call Call

    %%{
        # Define what header characters are allowed.
        comma = ",";
        str_chars = (any -- comma);

        action mark {
            start = p;
        }
        action capture_fn {
            call.Function = data[start:p]
        }
        action capture_constant {
            call.Args = append(call.Args, disambiguateFieldOrConstant(data[start:p]))
        }
        action commit {
            err = nil
        }

        # TODO: Don't be so strict...
        fn_chars = [A-Za-z0-9_];
        sp = " ";
        function = (fn_chars+ >mark %capture_fn);
        constant_str = str_chars+ >mark %capture_constant;
        argument = constant_str;
        function_call = sp* function "(" argument ( comma argument)* ")" space* %commit;
        main := function_call;
        write init;
        write exec;
    }%%
    if err != nil {
        return nil, err
    }
    return &call, nil;
}

