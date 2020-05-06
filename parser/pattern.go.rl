//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

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
    mark := 0

    isPayload := false

    %%{
        action mark {
            mark = p;
        }
        action capture_constant {
            if mark < p-1 {
                pattern = append(pattern, Constant(data[mark:p-1]))
            }
            mark = p
        }
        action capture_field {
            if !isPayload {
                pattern = append(pattern, Field{Name: data[mark:p]})
            } else {
                pattern = append(pattern, Payload(Field{Name: data[mark:p]}))
                isPayload = false
            }
            mark = p
        }
        action commit {
            err = nil
            if mark < p {
                pattern = append(pattern, Constant(data[mark:p]))
            }
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
                pattern = append(pattern, Payload(Field{}))
                isPayload = false
            }
        }
        const_chars = "<<" | (any -- "<");
        field_chars = [A-Za-z_0-9\.];
        payload_field_chars = [$A-Za-z_0-9\.];
        field_name = field_chars+ >mark %capture_field;
        payload_field_name = payload_field_chars+ >mark %capture_field;
        payload_custom = ":" payload_field_name;
        payload_decl = "!payload" %enter_payload payload_custom? %leave_payload;
        field = "<"  %capture_constant (payload_decl | field_name) $/onerror ">" %mark @^onerror;
        constant = const_chars++;
        pattern = ( field | constant );

        main := pattern* %commit;

        write init;
        write exec;
    }%%
    if err != nil {
        return nil, err
    }
    return pattern, nil
}
