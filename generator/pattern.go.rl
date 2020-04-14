//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package generator

import "github.com/pkg/errors"

%%{
    machine parse_pattern;
    write data;
    variable p p;
    variable pe pe;
}%%

var ErrBadPattern = errors.New("malformed pattern")

// ParsePattern parses a device log parser pattern.
func ParsePattern(data string) (pattern Pattern, err error) {
    cs, p, pe, eof := 0, 0, len(data), len(data)
    mark := -1

    isPayload := false

    %%{
        action mark {
            mark = p;
        }
        #action capture_constant {
        #    if mark>-1 && p-mark > 0 {
        #    pattern = append(pattern, Constant(data[mark:p]))
        #    }
        #}
        action capture_constant {
            //fmt.Fprintf(os.Stderr, "XXX: capture_constant at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
            if mark < p {
                pattern = append(pattern, Constant(data[mark:p]))
            }
        }
        action capture_field {
            //fmt.Fprintf(os.Stderr, "XXX: capture_field at %d (len %d): '%s'\n", p, p-mark, data[mark:p])
            if !isPayload {
                pattern = append(pattern, Field(data[mark:p]))
            } else {
                pattern = append(pattern, Payload(Field(data[mark:p])))
                isPayload = false
            }
        }
        action commit {
            err = nil
        }
        action onerror {
            mark = eof - p
            if mark > 20 {
                mark = 20
            }
            if mark > 0 {
                err = errors.Errorf("malformed pattern at position %d near '%s'", p, data[p:p+mark])
            } else {
                err = errors.Errorf("malformed pattern at position %d (EOF)", p)
            }
        }
        action enter_payload {
            isPayload = true
        }
        action leave_payload {
            if isPayload {
                pattern = append(pattern, Payload(Field("")))
                isPayload = false
            }
        }
        const_chars = "<<" | (any -- "<");
        field_chars = [A-Za-z_0-9];
        field_name = field_chars+ >mark %capture_field;
        payload_custom = ":" field_name;
        payload_decl = "!payload" %enter_payload payload_custom? %leave_payload;
        field = "<" (payload_decl | field_name) $/onerror ">" @^onerror;
        constant = const_chars* >mark %capture_constant;
        pattern = ( field | constant ) ;

        main := pattern* %commit;

        write init;
        write exec;
    }%%
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
            next := i+1
            for ;next < nn && isConstant(pattern[next]); next++ {
                pattern[i] = Constant(string(pattern[i].(Constant)) + string(pattern[next].(Constant)))
            }
            out = append(out, pattern[i])
            i = next - 1
        } else {
            out = append(out, pattern[i])
        }
    }
    return out, nil;
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
