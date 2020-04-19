//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

package parser

%%{
    machine parse_alternatives;
    write data;
    variable p p;
    variable pe pe;
}%%

var errSplitAltFailed = errors.New("failed to split alternatives");

func splitAlternatives(data string) (out []interface{}, err error) {
    cs, p, pe, eof := 0, 0, len(data), len(data)

    start_plain, end_plain := 0, 0
    start_alt := 0
    err = errSplitAltFailed;
    var alts []interface{}

    %%{

        action commit {
            err = nil
        }
        action plain_start {
            if len(alts) > 0 {
                out = append(out, alts)
                alts = nil
            }
            start_plain = p
        }
        action plain_end {
            end_plain = p
        }
        action plain_save {
            if start_plain < end_plain {
                out = append(out, data[start_plain:end_plain])
            }
            start_plain = p
            end_plain = p
        }
        action alt_start {
            start_alt = p
            alts = nil
        }
        action alt_end {
            // TODO
            alts = append(alts, data[start_alt:p])
            start_alt = p+1
        }
        alt_open = "{";
        alt_close = "}";
        alt_sep = "|";
        open_bracket = "{{";

        # pipe = "||"; -- my understanding is that pipe inside alternatives is not supported.
        # regular '|' pipe outside alternatives is treated as any other char.

        dissect_chars = open_bracket | (any -- alt_open);
        inner_chars   = any -- alt_open -- alt_sep -- alt_close;

        inner_expr = inner_chars+;

        plain_expr = dissect_chars+;
        alt_expr = alt_open >plain_end %alt_start inner_expr alt_sep >plain_save >alt_end inner_expr (alt_sep >alt_end inner_expr)* alt_close >alt_end %plain_start;

        expression = alt_expr | plain_expr;

        main := (expression)* %plain_end %plain_save %commit;

        write init;
        write exec;
    }%%
    /*if err == errSplitAltFailed {
        out = nil
    }*/
    return
}
