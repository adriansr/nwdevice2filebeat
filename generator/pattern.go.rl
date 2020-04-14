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
    last := 0
    //err = ErrBadPattern

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
            if last < p {
                pattern = append(pattern, Constant(data[last:p]))
            }
        }
        action set_last {
            last = p;
        }
        action capture_field {
            if !isPayload {
                pattern = append(pattern, Field(data[mark:p]))
            } else {
                pattern = append(pattern, Payload(Field(data[mark:p])))
                isPayload = false
            }
        }
        action commit {
            if err==nil && last < p {
                pattern = append(pattern, Constant(data[last:p]))
                last = mark
            }
            //err = nil
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

        #escaped_lt = "<<";
        #pattern_chars = escaped_lt | (any -- "<");
        field_chars = [A-Za-z_0-9];
        field_name = field_chars+ >mark %capture_field;
        payload_custom = ":" field_chars+ >mark %capture_field;
        payload_decl = "!payload" %enter_payload payload_custom? %leave_payload;
        field = "<" >capture_constant (payload_decl | field_name) ">" %set_last;
        pattern = ( field | any ) ;

        main := pattern* $!onerror %commit;

        write init;
        write exec;
    }%%
    if err != nil {
        return nil, err
    }
    return pattern, nil;
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
