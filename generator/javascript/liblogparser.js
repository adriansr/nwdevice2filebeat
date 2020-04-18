// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

var FLAG_FIELD = "log.flags";
var FIELDS_PREFIX = "nwparser.";

var saved_flags = null;

function processor_chain(subprocessors) {
    var builder = new processor.Chain();
    for (var i=0; i<subprocessors.length; i++) {
        builder.Add(subprocessors[i]);
    }
    return builder.Build().Run;
}

function linear_select(subprocessors) {
    return function(evt) {
        var saved_flags = evt.Get(FLAG_FIELD);
        var i;
        for (i=0; i<subprocessors.length; i++) {
            evt.Delete(FLAG_FIELD);
            // TODO: What if dissect sets FLAG_FIELD? :)
            subprocessors[i](evt);
            // Dissect processor succeeded?
            if (evt.Get(FLAG_FIELD) == null) break;
        }
        if (saved_flags !== null) {
            evt.Put(FLAG_FIELD, saved_flags);
        }
    }
}

function match(options) {
    var dissect = new processor.Dissect(options.dissect);
    return function(evt) {
        dissect.Run(evt);
        if (options.on_success != null && evt.Get(FLAG_FIELD) === null) {
            options.on_success(evt);
        }
    }
}

function save_flags(evt) {
    saved_flags = evt.Get(FLAG_FIELD);
}

function restore_flags(evt) {
    if (saved_flags !== null) {
        evt.Put(FLAG_FIELD, saved_flags);
    }
}

function constant(value) {
    return function(evt) {
        return value;
    }
}

function field(name) {
    var fullname = FIELDS_PREFIX + name;
    return function(evt) {
        return evt.Get(fullname);
    }
}

function STRCAT(evt, args) {
    var s = "";
    var i;
    for (i=0; i<args.length; i++) {
        s += args[i];
    }
    return s;
}

function call(opts) {
    return function(evt) {
        // TODO: Optimize this
        var args = new Array(opts.args.length);
        var i;
        for (i=0; i<opts.args.length; i++) {
            args[i] = opts.args[i](evt);
        }
        var result = opts.fn(evt, args);
        if (result != null) {
            evt.Put(opts.dest, result);
        }
    }
}

function lookup(opts) {
    return function(evt) {
        var key = opts.key(evt);
        if (key == null) return;
        var value = opts.map.keyvaluepairs[key];
        if (value === undefined) {
            value = opts.map.default;
        }
        if (value !== undefined) {
            evt.Put(opts.dest, value(evt));
        }
    }
}

function set_field(opts) {
    return function(evt) {
        evt.Put(opts.dest, opts.value(evt));
    }
}

function date_time(opts) {
    return function(evt) {
        var str = "";
        for (var i = 0; i < opts.args.length; i++) {
            var fname = FIELDS_PREFIX + opts.args[i];
            var val = evt.Get(fname);
            if (val != null) {
                if (str != "") str += " ";
                str += val;
            } else {
                console.warn("in date_time: input arg " + fname + " is not set");
            }
        }
        var date = new Date();
        var pos = 0;
        var len = str.length;
        for (var proc=0; pos!==undefined && pos<len && proc<opts.fmt.length; proc++) {
            //console.warn("in date_time: enter proc["+proc+"]='" + str + "' pos=" + pos + " date="+date);
            pos = opts.fmt[proc](str, pos, date);
            //console.warn("in date_time: leave proc["+proc+"]='" + str + "' pos=" + pos + " date="+date);
        }
        if (pos !== undefined) {
            evt.Put(FIELDS_PREFIX + opts.dest, date);
        }
        //console.warn("in date_time: result=" + date);
    }
}

function dc(ct) {
    return function(str, pos, date) {
        var n = str.length;
        if (n - pos < ct.length) return;
        var part = str.substr(pos, ct.length);
        if (part != ct) {
            console.warn("parsing date_time '" + str + "' at " + pos + ": '" + part + "' is not '" + ct + "'");
            return;
        }
        return pos + ct.length;
    }
}


var shortMonths = {
    // mon => [ month_id , how many chars to skip if month in long form ]
    "Jan": [0, 4],
    "Feb": [1, 5],
    "Mar": [2, 2],
    "Apr": [3, 2],
    "May": [4, 0],
    "Jun": [5, 1],
    "Jul": [6, 1],
    "Aug": [7, 3],
    "Sep": [8, 6],
    "Oct": [9, 4],
    "Nov": [10, 5],
    "Dec": [11, 4],
    "jan": [0, 4],
    "feb": [1, 5],
    "mar": [2, 2],
    "apr": [3, 2],
    "may": [4, 0],
    "jun": [5, 1],
    "jul": [6, 1],
    "aug": [7, 3],
    "sep": [8, 6],
    "oct": [9, 4],
    "nov": [10, 5],
    "dec": [11, 4],
};

// var dC = undefined;
var dR = dateMonthName(true);
var dB = dateMonthName(false);
var dM = dateFixedWidthNumber('M', 2, 1, 12, function(date, value) { date.SetMonth(value-1); });
var dG = dateVariableWidthNumber('G', 1, 12, function(date, value) { date.SetMonth(value-1); });
var dD = dateFixedWidthNumber('D',2, 1, 31, Date.prototype.setDate);
var dF = dateVariableWidthNumber('F', 1, 31, Date.prototype.setDate);
var dH = dateFixedWidthNumber('H', 2, 0, 24, Date.prototype.setHours);
// TODO: var dI = ...
var dN = dateVariableWidthNumber('N', 0, 24, Date.prototype.setHours);
var dT = dateFixedWidthNumber('T', 2, 0, 59, Date.prototype.setMinutes);
var dU = dateVariableWidthNumber('U', 0, 59, Date.prototype.setMinutes);
// TODO: var dJ = ...Julian day...
// TODO: var dP = ...AM|PM...
// TODO: var dQ = ...A.M.|P.M....
var dS = dateFixedWidthNumber('S', 2,0, 60, Date.prototype.setSeconds);
var dO = dateVariableWidthNumber('O', 0, 60, Date.prototype.setSeconds);
// TODO: var dY = ...YY...
var dW = dateFixedWidthNumber('W', 4, 1000, 9999, Date.prototype.setFullYear);
// TODO: var dZ = ...
// TODO: var dA = ...
// TODO: var dX = ...

function skipws(str, pos) {
    for ( var n = str.length
        ; pos<n && str.charAt(pos) === ' '
        ; pos++)
        ;
    return pos;
}

function skipdigits(str, pos) {
    var c;
    for ( var n = str.length
        ; pos<n && (c=str.charAt(pos)) >= '0' && c <= '9'
        ; pos++)
        ;
    return pos;
}

function dateVariableWidthNumber(fmtChar, min, max, setter) {
    return function(str, pos, date) {
        var start = skipws(str, pos);
        pos = skipdigits(str, start);
        var s = str.substr(start, pos-start);
        var value = parseInt(s, 10);
        if (value >= min && value <= max) {
            setter.call(date, value);
            return pos;
        }
        console.warn("parsing date_time: '" + s + "' is not valid for %" + fmtChar);
        return;
    }
}


function dateFixedWidthNumber(fmtChar, width, min, max, setter) {
    return function(str, pos, date) {
        pos = skipws(str, pos);
        var n = str.length;
        if (pos + width > n) return;
        var s = str.substr(pos, width);
        var value = parseInt(s, 10);
        if (value >= min && value <= max) {
            setter.call(date, value);
            return pos + width;
        }
        console.warn("parsing date_time: '" + s + "' is not valid for %" + fmtChar);
        return;
    }
}

// Short month name (Jan..Dec).
function dateMonthName(long) {
    return function(str, pos, date) {
        pos = skipws(str, pos);
        var n = str.length;
        if (pos + 3 > n) return;
        var mon = str.substr(pos, 3);
        var idx = shortMonths[mon];
        if (idx === undefined) {
            idx = shortMonths[mon.toLowerCase()];
        }
        if (idx === undefined) {
            console.warn("parsing date_time: '" + mon + "' is not a valid short month (%B)");
            return;
        }
        date.setMonth(idx[0]);
        return pos + 3 + (long? idx[1] : 0);
    }
}
