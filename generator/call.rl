//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

%%{
    machine parse_call;
    write data;
    variable p p;
    variable pe pe;
}%%

var ErrBadCall = errors.New("malformed function call")

// ParseCall parses a function call.
// Input: "STRCAT('header_', msgIdPart2)"
// Output: Call(Function:"STRCAT", Args: [ Constant("header_"), Field("msgIdPart2")])
func ParseCall(data string) (call Call, err error) {
    cs, p, pe, eof := 0, 0, len(data), len(data)
    start := -1
    err = ErrBadCall;

    %%{
        # Define what header characters are allowed.
        comma = ",";
        quote = "'";
        escape = "\\";
        escape_quote = escape quote;
        backslash = "\\\\";
        str_chars = backslash | escape_quote | (any -- quote);

        action mark {
            start = p;
        }
        action capture_fn {
            call.Function = data[start:p]
        }
        action capture_constant {
            call.Args = append(call.Args, Constant(data[start:p]))
        }
        action capture_field {
            call.Args = append(call.Args, Field(data[start:p]))
        }
        action commit {
            err = nil
        }

        # No function in the published parsers is outside of A-Z_.
        # TODO: Don't be so strict...
        fn_chars = [A-Z_];
        # TODO: Don't be so strict...
        field_chars = [A-Za-z_0-9];

        function = (fn_chars+ >mark %capture_fn);
        constant_str = quote (str_chars* >mark %capture_constant) quote;
        field_ref = (field_chars+ >mark %capture_field);
        argument = constant_str | field_ref;
        function_call = space* "*"? function space* "(" space* argument space* ( comma space* argument space* )* ")" space* %commit;

        main := function_call;
        write init;
        write exec;
    }%%

    return call, err;
}
