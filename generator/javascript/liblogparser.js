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


// TODO: PARMVAL can be replaced by a reference to a field, so function handlers
//       don't really need the evt arg (unless there's another reason).
function PARMVAL(evt, args) {
    return evt.Get(args[0]);
}

function STRCAT(evt, args) {
    var s = "";
    var i;
    for (i=0; i<args.length; i++) {
        s += args[i];
    }
    return s;
}

function EVNTTIME(evt, args) {
    // TODO
    return null;
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
        var key = evt.Get(opts.key);
        if (key == null) return;
        var value = evt.map.keyvaluepairs[key];
        if (value === undefined) {
            value = evt.map.default;
        }
        if (value !== undefined) {
            evt.Put(opts.dest, value);
        }
    }
}

function set_field(opts) {
    return function(evt) {
        evt.Put(FIELDS_PREFIX + opts.dest, opts.value(evt));
    }
}
