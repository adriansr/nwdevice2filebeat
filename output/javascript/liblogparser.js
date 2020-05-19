// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

var processor = require("processor");
var console = require("console");

var FLAG_FIELD = "log.flags";
var FIELDS_OBJECT = "nwparser";
var FIELDS_PREFIX = FIELDS_OBJECT + ".";

var defaults = {
    debug: false,
    ecs: true,
    rsa: false,
    keep_raw: false,
}
var saved_flags = null;
var debug;
var map_ecs;
var map_rsa;
var keep_raw;
var device;

// Register params from configuration.
function register(params) {
    debug = params.debug !== undefined ? params.debug : defaults.debug;
    map_ecs = params.ecs !== undefined ? params.ecs : defaults.ecs;
    map_rsa = params.rsa !== undefined ? params.rsa : defaults.rsa;
    keep_raw = params.keep_raw !== undefined ? params.keep_raw : defaults.keep_raw;
    device = new DeviceProcessor();
}

function process(evt) {
    // Function register is only called by the processor when `params` are set
    // in the processor config.
    if (device === undefined) {
        register(defaults);
    }
    return device.process(evt);
}

function processor_chain(subprocessors) {
    var builder = new processor.Chain();
    for (var i = 0; i < subprocessors.length; i++) {
        builder.Add(subprocessors[i]);
    }
    return builder.Build().Run;
}

function linear_select(subprocessors) {
    return function (evt) {
        var saved_flags = evt.Get(FLAG_FIELD);
        var i;
        for (i = 0; i < subprocessors.length; i++) {
            evt.Delete(FLAG_FIELD);
            if (debug) console.warn("linear_select trying entry " + i);
            subprocessors[i](evt);
            // Dissect processor succeeded?
            if (evt.Get(FLAG_FIELD) == null) break;
            if (debug) console.warn("linear_select failed entry " + i);
        }
        if (saved_flags !== null) {
            evt.Put(FLAG_FIELD, saved_flags);
        }
        if (debug) {
            if (i < subprocessors.length) {
                console.warn("linear_select matched entry " + i);
            } else {
                console.warn("linear_select didn't match");
            }
        }
    }
}

function match(id, src, pattern, on_success) {
    var dissect = new processor.Dissect({
        field: src,
        tokenizer: pattern,
        target_prefix: FIELDS_OBJECT,
        ignore_failure: true,
        overwrite_keys: true,
    });
    return function (evt) {
        var msg = evt.Get(src);
        dissect.Run(evt);
        var failed = evt.Get(FLAG_FIELD) != null;
        if (debug) {
            if (failed) {
                console.debug("dissect fail: " + id + " field:" + src);
            } else {
                console.debug("dissect   OK: " + id + " field:" + src);
            }
            console.debug("        expr: <<" + pattern + ">>");
            console.debug("       input: <<" + msg + ">>");
        }
        if (on_success != null && !failed) {
            on_success(evt);
        }
    }
}

function all_match(opts) {
    return function (evt) {
        var i;
        for (i = 0; i < opts.processors.length; i++) {
            evt.Delete(FLAG_FIELD);
            // TODO: What if dissect sets FLAG_FIELD? :)
            opts.processors[i](evt);
            // Dissect processor succeeded?
            if (evt.Get(FLAG_FIELD) != null) {
                if (debug) console.warn("all_match failure at " + i);
                if (opts.on_failure != null) opts.on_failure(evt);
                return;
            }
            if (debug) console.warn("all_match success at " + i);
        }
        if (opts.on_success != null) opts.on_success(evt);
    }
}

function msgid_select(mapping) {
    return function (evt) {
        var msgid = evt.Get(FIELDS_PREFIX + "messageid");
        if (msgid == null) {
            if (debug) console.warn("msgid_select: no messageid captured!")
            return;
        }
        var next = mapping[msgid];
        if (next === undefined) {
            if (debug) console.warn("msgid_select: no mapping for messageid:" + msgid);
            return;
        }
        if (debug) console.info("msgid_select: matched key=" + msgid);
        return next(evt);
    }
}

function msg(msg_id, match) {
    return function (evt) {
        match(evt);
        if (evt.Get(FLAG_FIELD) == null) {
            evt.Put(FIELDS_PREFIX + "msg_id1", msg_id);
        }
    }
}

var start;

function save_flags(evt) {
    saved_flags = evt.Get(FLAG_FIELD);
    evt.Put("event.original", evt.Get("message"));
}

function restore_flags(evt) {
    if (saved_flags !== null) {
        evt.Put(FLAG_FIELD, saved_flags);
    }
}

function constant(value) {
    return function (evt) {
        return value;
    }
}

function field(name) {
    var fullname = FIELDS_PREFIX + name;
    return function (evt) {
        return evt.Get(fullname);
    }
}

function STRCAT(evt, args) {
    var s = "";
    var i;
    for (i = 0; i < args.length; i++) {
        s += args[i];
    }
    return s;
}

/*
    call({dest: "nwparser.", fn: SYSVAL, args: [ field("$MSGID"),field("$ID1")]}),

    TODO:

    The above seems to indicate that in order to select MESSAGES from a header
    The value attribute "id1" must be used as key.
 */
function SYSVAL(evt, args) {
}

// TODO: Prune this from the tree.
function HDR(evt, args) {
}

// TODO: Implement?
function DIRCHK(evt, args) {
}

function DUR(evt, args) {
}

function URL(evt, args) {
}

function CALC(evt, args) {
    if (args.length != 3) {
        console.warn("skipped call to CALC with " + args.length + " arguments.");
        return;
    }
    var a = parseInt(args[0]);
    var b = parseInt(args[2]);
    if (isNaN(a) || isNaN(b)) {
        console.warn("failed evaluating CALC arguments a='" + args[0] + "' b='" + args[2] + "'.");
        return;
    }
    var result;
    switch (args[1]) {
        case "+":
            result = a + b;
            break;
        case '-':
            result = a - b;
            break;
        case '*':
            result = a * b;
            break;
        default:
            // Only * and + seen in the parsers.
            console.warn("unknown CALC operation '" + args[1] + "'.");
            return;
    }
    // Always return a string
    return result !== undefined ? "" + result : result;
}

function RMQ(evt, args) {

}

function UTC(evt, args) {

}

function call(opts) {
    return function (evt) {
        // TODO: Optimize this
        var args = new Array(opts.args.length);
        var i;
        for (i = 0; i < opts.args.length; i++) {
            args[i] = opts.args[i](evt);
        }
        var result = opts.fn(evt, args);
        if (result != null) {
            evt.Put(opts.dest, result);
        }
    }
}

function nop(evt) {
}

function lookup(opts) {
    return function (evt) {
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

function set(fields) {
    return new processor.AddFields({
        target: FIELDS_OBJECT,
        fields: fields,
    });
}

function setf(dst, src) {
    return function (evt) {
        var val = evt.Get(FIELDS_PREFIX + src);
        if (val != null) evt.Put(FIELDS_PREFIX + dst, val);
    }
}

function setc(dst, value) {
    return function (evt) {
        evt.Put(FIELDS_PREFIX + dst, value);
    }
}

function set_field(opts) {
    return function (evt) {
        var val = opts.value(evt);
        if (val != null) evt.Put(opts.dest, val);
    }
}

function dump(label) {
    return function (evt) {
        console.log("Dump of event at " + label + ": " + JSON.stringify(evt, null, '\t'))
    }
}

function date_time_join_args(evt, arglist) {
    var str = "";
    for (var i = 0; i < arglist.length; i++) {
        var fname = FIELDS_PREFIX + arglist[i];
        var val = evt.Get(fname);
        if (val != null) {
            if (str != "") str += " ";
            str += val;
        } else {
            if (debug) console.warn("in date_time: input arg " + fname + " is not set");
        }
    }
    return str;
}

function date_time_try_pattern(evt, opts, fmt, str) {
    var date = new Date();
    var pos = 0;
    var len = str.length;
    for (var proc = 0; pos !== undefined && pos < len && proc < fmt.length; proc++) {
        //console.warn("in date_time: enter proc["+proc+"]='" + str + "' pos=" + pos + " date="+date);
        pos = fmt[proc](str, pos, date);
        //console.warn("in date_time: leave proc["+proc+"]='" + str + "' pos=" + pos + " date="+date);
    }
    var done = pos !== undefined;
    if (done) evt.Put(FIELDS_PREFIX + opts.dest, date);
    return done;
}

function date_times(opts) {
    return function (evt) {
        var str = date_time_join_args(evt, opts.args);
        var i;
        for (i = 0; i < opts.fmts.length; i++) {
            if (date_time_try_pattern(evt, opts, opts.fmts[i], str)) {
                if (debug) console.warn("in date_times: succeeded: " + evt.Get(FIELDS_PREFIX + opts.dest));
                return;
            }
        }
        if (debug) console.warn("in date_time: id=" + opts.id + " (s) FAILED: " + str);
    }
}

function date_time(opts) {
    return function (evt) {
        var str = date_time_join_args(evt, opts.args);
        if (!date_time_try_pattern(evt, opts, opts.fmt, str)) {
            if (debug) console.warn("in date_time: id=" + opts.id + " FAILED: " + str);
        }
    }
}

function duration(opts) {
    // TODO: Duration
    return nop;
}

function durations(opts) {
    // TODO: Durations
    return nop;
}

function remove(fields) {
    return function (evt) {
        for (var i = 0; i < fields.length; i++) {
            evt.Delete(FIELDS_PREFIX + fields[i]);
        }
    }
}

function dc(ct) {
    return function (str, pos, date) {
        var n = str.length;
        if (n - pos < ct.length) return;
        var part = str.substr(pos, ct.length);
        if (part != ct) {
            //console.warn("parsing date_time '" + str + "' at " + pos + ": '" + part + "' is not '" + ct + "'");
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

var monthSetter = {
    call: function (date, value) {
        date.setMonth(value - 1);
    }
};

var unixSetter = {
    call: function (date, value) {
        date.setTime(value * 1000);
    }
}

// var dC = undefined;
var dR = dateMonthName(true);
var dB = dateMonthName(false);
var dM = dateFixedWidthNumber('M', 2, 1, 12, monthSetter);
var dG = dateVariableWidthNumber('G', 1, 12, monthSetter);
var dD = dateFixedWidthNumber('D', 2, 1, 31, Date.prototype.setDate);
var dF = dateVariableWidthNumber('F', 1, 31, Date.prototype.setDate);
var dH = dateFixedWidthNumber('H', 2, 0, 24, Date.prototype.setHours);
// TODO: var dI = ...
var dN = dateVariableWidthNumber('N', 0, 24, Date.prototype.setHours);
var dT = dateFixedWidthNumber('T', 2, 0, 59, Date.prototype.setMinutes);
var dU = dateVariableWidthNumber('U', 0, 59, Date.prototype.setMinutes);
// TODO: var dJ = ...Julian day...
// TODO: var dP = ...AM|PM...
// TODO: var dQ = ...A.M.|P.M....
var dS = dateFixedWidthNumber('S', 2, 0, 60, Date.prototype.setSeconds);
var dO = dateVariableWidthNumber('O', 0, 60, Date.prototype.setSeconds);
// TODO: var dY = ...YY...
var dW = dateFixedWidthNumber('W', 4, 1000, 9999, Date.prototype.setFullYear);
// TODO: var dZ = ...
// TODO: var dA = ...
var dX = dateVariableWidthNumber('X', 0, 0x10000000000, unixSetter);

function skipws(str, pos) {
    for (var n = str.length
        ; pos < n && str.charAt(pos) === ' '
        ; pos++)
        ;
    return pos;
}

function skipdigits(str, pos) {
    var c;
    for (var n = str.length
        ; pos < n && (c = str.charAt(pos)) >= '0' && c <= '9'
        ; pos++)
        ;
    return pos;
}

function dateVariableWidthNumber(fmtChar, min, max, setter) {
    return function (str, pos, date) {
        var start = skipws(str, pos);
        pos = skipdigits(str, start);
        var s = str.substr(start, pos - start);
        var value = parseInt(s, 10);
        if (value >= min && value <= max) {
            setter.call(date, value);
            return pos;
        }
        //console.warn("parsing date_time: '" + s + "' is not valid for %" + fmtChar);
        return;
    }
}

function dateFixedWidthNumber(fmtChar, width, min, max, setter) {
    return function (str, pos, date) {
        pos = skipws(str, pos);
        var n = str.length;
        if (pos + width > n) return;
        var s = str.substr(pos, width);
        var value = parseInt(s, 10);
        if (value >= min && value <= max) {
            setter.call(date, value);
            return pos + width;
        }
        //console.warn("parsing date_time: '" + s + "' is not valid for %" + fmtChar);
        return;
    }
}

// Short month name (Jan..Dec).
function dateMonthName(long) {
    return function (str, pos, date) {
        pos = skipws(str, pos);
        var n = str.length;
        if (pos + 3 > n) return;
        var mon = str.substr(pos, 3);
        var idx = shortMonths[mon];
        if (idx === undefined) {
            idx = shortMonths[mon.toLowerCase()];
        }
        if (idx === undefined) {
            //console.warn("parsing date_time: '" + mon + "' is not a valid short month (%B)");
            return;
        }
        date.setMonth(idx[0]);
        return pos + 3 + (long ? idx[1] : 0);
    }
}

function domain(dst, src) {
    return nop;
}

function ext(dst, src) {
    return nop;
}

function fqdn(dst, src) {
    return nop;
}

function page(dst, src) {
    return nop;
}

function path(dst, src) {
    return nop;
}

function port(dst, src) {
    return nop;
}

function query(dst, src) {
    return nop;
}

function root(dst, src) {
    return nop;
}

var uR = nop;
var uB = nop;
var uM = nop;
var uG = nop;
var uD = nop;
var uF = nop;
var uH = nop;
var uI = nop;
var uN = nop;
var uT = nop;
var uU = nop;
var uJ = nop;
var uP = nop;
var uQ = nop;
var uS = nop;
var uO = nop;
var uY = nop;
var uW = nop;
var uZ = nop;
var uA = nop;
var uX = nop;

var field_mappings = {
    'msg': {ecs: ['log.original'], rsa: ['rsa.internal.msg']},
    'messageid': {ecs: ['event.code'], rsa: ['rsa.internal.messageid']},
    'event_time': {convert: to_date, ecs: ['@timestamp'], rsa: ['rsa.time.event_time']},
    'username': {ecs: ['user.name']},
    'event_description': {ecs: ['message'], rsa: ['rsa.internal.event_desc']},
    'action': {ecs: ['event.action'], rsa: ['rsa.misc.action']},
    'info': {rsa: ['rsa.db.index']},
    'saddr': {convert: to_ip, ecs: ['source.ip']},
    'payload': {rsa: ['rsa.internal.payload']},
    'message': {rsa: ['rsa.internal.message']},
    'hostname': {ecs: ['host.name'], rsa: ['rsa.network.alias_host']},
    'ec_activity': {rsa: ['rsa.investigations.ec_activity']},
    'ec_theme': {rsa: ['rsa.investigations.ec_theme']},
    'ec_subject': {rsa: ['rsa.investigations.ec_subject']},
    'result': {rsa: ['rsa.misc.result']},
    'severity': {ecs: ['log.level'], rsa: ['rsa.misc.severity']},
    'ec_outcome': {ecs: ['event.outcome'], rsa: ['rsa.investigations.ec_outcome']},
    'daddr': {convert: to_ip, ecs: ['destination.ip']},
    'event_type': {ecs: ['event.category'], rsa: ['rsa.misc.event_type']},
    'id': {ecs: ['event.code'], rsa: ['rsa.misc.reference_id']},
    'protocol': {convert: to_lowercase, ecs: ['network.protocol']},
    'version': {ecs: ['observer.version'], rsa: ['rsa.misc.version']},
    'filename': {ecs: ['file.name']},
    'hostip': {convert: to_ip, ecs: ['host.ip']},
    'sport': {convert: to_long, ecs: ['source.port']},
    'dport': {convert: to_long, ecs: ['destination.port']},
    'disposition': {rsa: ['rsa.misc.disposition']},
    'resultcode': {rsa: ['rsa.misc.result_code']},
    'category': {rsa: ['rsa.misc.category']},
    'obj_name': {rsa: ['rsa.misc.obj_name']},
    'shost': {ecs: ['host.hostname', 'source.address']},
    'obj_type': {rsa: ['rsa.misc.obj_type']},
    'url': {ecs: ['url.original']},
    'application': {ecs: ['network.application']},
    'event_source': {rsa: ['rsa.misc.event_source']},
    'service': {ecs: ['service.name']},
    'domain': {ecs: ['server.domain'], rsa: ['rsa.network.domain']},
    'sessionid': {rsa: ['rsa.misc.log_session_id']},
    'group': {rsa: ['rsa.misc.group']},
    'dhost': {ecs: ['destination.address'], rsa: ['rsa.network.host_dst']},
    'dclass_counter1': {convert: to_long, rsa: ['rsa.counters.dclass_c1']},
    'policyname': {rsa: ['rsa.misc.policy_name']},
    'c_username': {ecs: ['user.name']},
    'process_id': {convert: to_long, ecs: ['process.pid']},
    'process': {ecs: ['process.name']},
    'rulename': {ecs: ['rule.name'], rsa: ['rsa.misc.rule_name']},
    'context': {rsa: ['rsa.misc.context']},
    'change_new': {rsa: ['rsa.misc.change_new']},
    'product': {ecs: ['observer.product']},
    'space': {rsa: ['rsa.misc.space']},
    'agent': {rsa: ['rsa.misc.client']},
    'duration': {convert: to_double, rsa: ['rsa.time.duration_time']},
    'msgIdPart1': {rsa: ['rsa.misc.msgIdPart1']},
    'network_service': {rsa: ['rsa.network.network_service']},
    'directory': {ecs: ['file.directory']},
    'interface': {ecs: ['network.interface.name'], rsa: ['rsa.network.interface']},
    'msgIdPart2': {rsa: ['rsa.misc.msgIdPart2']},
    'change_old': {rsa: ['rsa.misc.change_old']},
    'event_time_string': {rsa: ['rsa.time.event_time_str']},
    'time': {convert: to_date, rsa: ['rsa.internal.time']},
    'bytes': {convert: to_long, ecs: ['network.bytes']},
    'smacaddr': {convert: to_mac, ecs: ['source.mac']},
    'operation_id': {rsa: ['rsa.misc.operation_id']},
    'event_state': {rsa: ['rsa.misc.event_state']},
    'sbytes': {convert: to_long, ecs: ['source.bytes']},
    'network_port': {convert: to_long, rsa: ['rsa.network.network_port']},
    'rbytes': {convert: to_long, ecs: ['destination.bytes']},
    'starttime': {convert: to_date, rsa: ['rsa.time.starttime']},
    'uid': {ecs: ['user.name']},
    'web_method': {rsa: ['rsa.misc.action']},
    'month': {rsa: ['rsa.time.month']},
    'authmethod': {rsa: ['rsa.identity.auth_method']},
    'day': {rsa: ['rsa.time.day']},
    'level': {convert: to_long, rsa: ['rsa.internal.level']},
    'group_object': {rsa: ['rsa.misc.group_object']},
    'node': {rsa: ['rsa.misc.node']},
    'rule': {rsa: ['rsa.misc.rule']},
    'macaddr': {convert: to_mac, rsa: ['rsa.network.eth_host']},
    'sinterface': {ecs: ['observer.ingress.interface.name'], rsa: ['rsa.network.sinterface']},
    'device': {rsa: ['rsa.misc.device_name']},
    'endtime': {convert: to_date, rsa: ['rsa.time.endtime']},
    'user_agent': {ecs: ['user_agent.original']},
    'msg_id': {rsa: ['rsa.internal.msg_id']},
    'timezone': {ecs: ['event.timezone'], rsa: ['rsa.time.timezone']},
    'param': {rsa: ['rsa.misc.param']},
    'to': {rsa: ['rsa.email.email_dst']},
    'dmacaddr': {convert: to_mac, ecs: ['destination.mac']},
    'change_attribute': {rsa: ['rsa.misc.change_attrib']},
    'direction': {ecs: ['network.direction']},
    'event_cat': {convert: to_long, rsa: ['rsa.investigations.event_cat']},
    'event_cat_name': {rsa: ['rsa.investigations.event_cat_name']},
    'event_computer': {rsa: ['rsa.misc.event_computer']},
    'from': {rsa: ['rsa.email.email_src']},
    'id1': {rsa: ['rsa.misc.reference_id1']},
    'stransaddr': {convert: to_ip, ecs: ['source.nat.ip']},
    'vid': {rsa: ['rsa.internal.msg_vid']},
    'privilege': {rsa: ['rsa.file.privilege']},
    'user_role': {rsa: ['rsa.identity.user_role']},
    'event_log': {rsa: ['rsa.misc.event_log']},
    'fqdn': {rsa: ['rsa.web.fqdn']},
    'administrator': {ecs: ['user.name']},
    'hostid': {rsa: ['rsa.network.alias_host']},
    'data': {rsa: ['rsa.internal.data']},
    'dclass_counter2': {convert: to_long, rsa: ['rsa.counters.dclass_c2']},
    'dinterface': {ecs: ['observer.egress.interface.name'], rsa: ['rsa.network.dinterface']},
    'os': {rsa: ['rsa.misc.OS']},
    'webpage': {ecs: ['http.response.body.content']},
    'terminal': {rsa: ['rsa.misc.terminal']},
    'msgIdPart3': {rsa: ['rsa.misc.msgIdPart3']},
    'filter': {rsa: ['rsa.misc.filter']},
    'serial_number': {rsa: ['rsa.misc.serial_number']},
    'subject': {rsa: ['rsa.email.subject']},
    'dn': {rsa: ['rsa.identity.dn']},
    'duration_string': {rsa: ['rsa.time.duration_str']},
    'instance': {rsa: ['rsa.db.instance']},
    'signame': {rsa: ['rsa.misc.policy_name']},
    'web_query': {ecs: ['url.query']},
    'date': {rsa: ['rsa.time.date']},
    'logon_type': {rsa: ['rsa.identity.logon_type']},
    'web_referer': {ecs: ['http.request.referrer']},
    'dtransaddr': {ecs: ['destination.nat.ip']},
    'threat_name': {rsa: ['rsa.threat.threat_category']},
    'vlan': {convert: to_long, rsa: ['rsa.network.vlan']},
    'checksum': {rsa: ['rsa.misc.checksum']},
    'event_user': {rsa: ['rsa.misc.event_user']},
    'year': {rsa: ['rsa.time.year']},
    'location_desc': {ecs: ['geo.name']},
    'virusname': {rsa: ['rsa.misc.virusname']},
    'user_address': {rsa: ['rsa.email.email']},
    'filename_size': {convert: to_long, ecs: ['file.size']},
    'stransport': {convert: to_long, ecs: ['source.nat.port']},
    'content_type': {rsa: ['rsa.misc.content_type']},
    'db_name': {rsa: ['rsa.db.database']},
    'dtransport': {convert: to_long, ecs: ['destination.nat.port']},
    'groupid': {rsa: ['rsa.misc.group_id']},
    'policy_id': {rsa: ['rsa.misc.policy_id']},
    'encryption_type': {rsa: ['rsa.crypto.crypto']},
    'recorded_time': {convert: to_date, rsa: ['rsa.time.recorded_time']},
    'vsys': {rsa: ['rsa.misc.vsys']},
    'web_domain': {ecs: ['url.domain']},
    'connectionid': {rsa: ['rsa.misc.connection_id']},
    'vendor_event_cat': {rsa: ['rsa.investigations.event_vcat']},
    'packets': {convert: to_long, ecs: ['network.packets']},
    'parent_pid': {convert: to_long, ecs: ['process.ppid']},
    'profile': {rsa: ['rsa.identity.profile']},
    'id2': {rsa: ['rsa.misc.reference_id2']},
    'sensor': {rsa: ['rsa.misc.sensor']},
    'sigid': {convert: to_long, rsa: ['rsa.misc.sig_id']},
    'logon_id': {ecs: ['user.name']},
    'datetime': {rsa: ['rsa.time.datetime']},
    'src_zone': {rsa: ['rsa.network.zone_src']},
    'user_fullname': {ecs: ['user.full_name']},
    'portname': {rsa: ['rsa.misc.port_name']},
    'rule_group': {rsa: ['rsa.misc.rule_group']},
    'owner': {ecs: ['user.name']},
    'ssid': {rsa: ['rsa.wireless.wlan_ssid']},
    'zone': {rsa: ['rsa.network.zone']},
    'dst_zone': {rsa: ['rsa.network.zone_dst']},
    'accesses': {rsa: ['rsa.identity.accesses']},
    'gateway': {rsa: ['rsa.network.gateway']},
    'risk_num': {convert: to_double, rsa: ['rsa.misc.risk_num']},
    'trigger_val': {rsa: ['rsa.misc.trigger_val']},
    's_cipher': {rsa: ['rsa.crypto.cipher_src']},
    'icmptype': {convert: to_long, rsa: ['rsa.network.icmp_type']},
    'sessionid1': {rsa: ['rsa.misc.log_session_id1']},
    'obj_server': {rsa: ['rsa.internal.obj_server']},
    'threat_val': {rsa: ['rsa.threat.threat_desc']},
    'web_cookie': {rsa: ['rsa.web.web_cookie']},
    'web_root': {ecs: ['url.path']},
    'web_host': {rsa: ['rsa.web.alias_host']},
    'component_version': {rsa: ['rsa.misc.comp_version']},
    'content_version': {rsa: ['rsa.misc.content_version']},
    'event_counter': {convert: to_long, rsa: ['rsa.counters.event_counter']},
    'hardware_id': {rsa: ['rsa.misc.hardware_id']},
    'mask': {rsa: ['rsa.network.mask']},
    'risk': {rsa: ['rsa.misc.risk']},
    'event_id': {rsa: ['rsa.misc.event_id']},
    'reason': {rsa: ['rsa.misc.reason']},
    'status': {rsa: ['rsa.misc.status']},
    'dclass_ratio1': {rsa: ['rsa.counters.dclass_r1']},
    'ddomain': {ecs: ['destination.domain']},
    'filetype': {ecs: ['file.type']},
    'icmpcode': {convert: to_long, rsa: ['rsa.network.icmp_code']},
    'mail_id': {rsa: ['rsa.misc.mail_id']},
    'realm': {rsa: ['rsa.identity.realm']},
    'sdomain': {ecs: ['source.domain']},
    'sid': {rsa: ['rsa.identity.user_sid_dst']},
    'cert_subject': {rsa: ['rsa.crypto.cert_subject']},
    'dclass_counter3': {convert: to_long, rsa: ['rsa.counters.dclass_c3']},
    'disk_volume': {rsa: ['rsa.storage.disk_volume']},
    'reputation_num': {convert: to_double, rsa: ['rsa.web.reputation_num']},
    'access_point': {rsa: ['rsa.wireless.access_point']},
    'dclass_counter1_string': {rsa: ['rsa.counters.dclass_c1_str']},
    'src_dn': {rsa: ['rsa.identity.dn_src']},
    'peer': {rsa: ['rsa.crypto.peer']},
    'protocol_detail': {rsa: ['rsa.network.protocol_detail']},
    'rule_uid': {rsa: ['rsa.misc.rule_uid']},
    'c_domain': {ecs: ['source.domain']},
    'trigger_desc': {rsa: ['rsa.misc.trigger_desc']},
    'host': {ecs: ['host.name']},
    'inout': {rsa: ['rsa.misc.inout']},
    'p_msgid': {rsa: ['rsa.misc.p_msgid']},
    'child_pid': {convert: to_long, ecs: ['process.pid']},
    'location_src': {ecs: ['source.geo.country_name']},
    'dmask': {rsa: ['rsa.network.dmask']},
    'effective_time': {convert: to_date, rsa: ['rsa.time.effective_time']},
    'saddr_v6': {convert: to_ip, ecs: ['source.ip']},
    'port': {convert: to_long, rsa: ['rsa.network.port']},
    'process_src': {ecs: ['process.parent.name']},
    'smask': {rsa: ['rsa.network.smask']},
    'trans_id': {rsa: ['rsa.db.transact_id']},
    'web_ref_domain': {rsa: ['rsa.web.web_ref_domain']},
    'data_type': {rsa: ['rsa.misc.data_type']},
    'msgIdPart4': {rsa: ['rsa.misc.msgIdPart4']},
    's_ciphersize': {convert: to_long, rsa: ['rsa.crypto.cipher_size_src']},
    'location_dst': {ecs: ['destination.geo.country_name']},
    'error': {rsa: ['rsa.misc.error']},
    'expiration_time': {convert: to_date, rsa: ['rsa.time.expire_time']},
    'ike': {rsa: ['rsa.crypto.ike']},
    'index': {rsa: ['rsa.misc.index']},
    'listnum': {rsa: ['rsa.misc.listnum']},
    'location_country': {ecs: ['geo.country_name']},
    'lun': {rsa: ['rsa.storage.lun']},
    'obj_value': {rsa: ['rsa.internal.obj_val']},
    'user_org': {rsa: ['rsa.identity.org']},
    'resource': {rsa: ['rsa.internal.resource']},
    'scheme': {rsa: ['rsa.crypto.scheme']},
    'service_account': {ecs: ['user.name']},
    'ntype': {rsa: ['rsa.misc.ntype']},
    'dst_dn': {rsa: ['rsa.identity.dn_dst']},
    'domain_id': {ecs: ['user.domain']},
    'user_fname': {rsa: ['rsa.identity.firstname']},
    'user_lname': {rsa: ['rsa.identity.lastname']},
    'observed_val': {rsa: ['rsa.misc.observed_val']},
    'policy_value': {rsa: ['rsa.misc.policy_value']},
    'pool_name': {rsa: ['rsa.misc.pool_name']},
    'process_id_src': {convert: to_long, ecs: ['process.ppid']},
    'rule_template': {rsa: ['rsa.misc.rule_template']},
    'count': {rsa: ['rsa.misc.count']},
    'number': {rsa: ['rsa.misc.number']},
    'sigcat': {rsa: ['rsa.misc.sigcat']},
    'type': {rsa: ['rsa.misc.type']},
    'r_hostid': {rsa: ['rsa.network.alias_host']},
    'comments': {rsa: ['rsa.misc.comments']},
    'dns_querytype': {ecs: ['dns.question.type']},
    'doc_number': {convert: to_long, rsa: ['rsa.misc.doc_number']},
    'cc': {rsa: ['rsa.email.email']},
    'expected_val': {rsa: ['rsa.misc.expected_val']},
    'daddr_v6': {convert: to_ip, ecs: ['destination.ip']},
    'jobnum': {rsa: ['rsa.misc.job_num']},
    'obj_id': {rsa: ['rsa.internal.obj_id']},
    'peer_id': {rsa: ['rsa.crypto.peer_id']},
    'permissions': {rsa: ['rsa.db.permissions']},
    'processing_time': {rsa: ['rsa.time.process_time']},
    'sigtype': {rsa: ['rsa.crypto.sig_type']},
    'dst_spi': {rsa: ['rsa.misc.spi_dst']},
    'src_spi': {rsa: ['rsa.misc.spi_src']},
    'statement': {rsa: ['rsa.internal.statement']},
    'user_dept': {rsa: ['rsa.identity.user_dept']},
    'c_sid': {rsa: ['rsa.identity.user_sid_src']},
    'web_ref_query': {rsa: ['rsa.web.web_ref_query']},
    'wifi_channel': {convert: to_long, rsa: ['rsa.wireless.wlan_channel']},
    'bssid': {rsa: ['rsa.wireless.wlan_ssid']},
    'cert_issuer': {rsa: ['rsa.crypto.cert_issuer']},
    'code': {rsa: ['rsa.misc.code']},
    'method': {ecs: ['http.request.method']},
    'remote_domain': {rsa: ['rsa.web.remote_domain']},
    'agent.id': {rsa: ['rsa.misc.agent_id']},
    'cert_hostname': {rsa: ['rsa.crypto.cert_host_name']},
    'ip.orig': {convert: to_ip, ecs: ['network.forwarded_ip']},
    'location_city': {ecs: ['geo.city_name']},
    'message_body': {rsa: ['rsa.misc.message_body']},
    'calling_to': {rsa: ['rsa.misc.phone']},
    'sigid_string': {rsa: ['rsa.misc.sig_id_str']},
    'tbl_name': {rsa: ['rsa.db.table_name']},
    'c_user_name': {ecs: ['user.name']},
    'cmd': {rsa: ['rsa.misc.cmd']},
    'misc': {rsa: ['rsa.misc.misc']},
    'name': {rsa: ['rsa.misc.name']},
    'web_ref_host': {rsa: ['rsa.network.alias_host']},
    'audit_class': {rsa: ['rsa.internal.audit_class']},
    'cert_error': {rsa: ['rsa.crypto.cert_error']},
    'd_cipher': {rsa: ['rsa.crypto.cipher_dst']},
    'd_ciphersize': {convert: to_long, rsa: ['rsa.crypto.cipher_size_dst']},
    'cpu': {convert: to_long, rsa: ['rsa.misc.cpu']},
    'db_id': {rsa: ['rsa.db.db_id']},
    'db_pid': {convert: to_long, rsa: ['rsa.db.db_pid']},
    'entry': {rsa: ['rsa.internal.entry']},
    'detail': {rsa: ['rsa.misc.event_desc']},
    'federated_sp': {rsa: ['rsa.identity.federated_sp']},
    'netname': {rsa: ['rsa.network.netname']},
    'paddr': {convert: to_ip, rsa: ['rsa.network.paddr']},
    'calling_from': {rsa: ['rsa.misc.phone']},
    'child_process': {ecs: ['process.name']},
    'parent_process': {ecs: ['process.parent.name']},
    'sigid1': {convert: to_long, rsa: ['rsa.misc.sig_id1']},
    's_sslver': {rsa: ['rsa.crypto.ssl_ver_src']},
    'trans_from': {rsa: ['rsa.email.trans_from']},
    'web_ref_page': {rsa: ['rsa.web.web_ref_page']},
    'web_ref_root': {rsa: ['rsa.web.web_ref_root']},
    'wlan': {rsa: ['rsa.wireless.wlan_name']},
    'd_certauth': {rsa: ['rsa.crypto.d_certauth']},
    'faddr': {rsa: ['rsa.network.faddr']},
    'hour': {rsa: ['rsa.time.hour']},
    'im_buddyid': {rsa: ['rsa.misc.im_buddyid']},
    'im_client': {rsa: ['rsa.misc.im_client']},
    'im_userid': {rsa: ['rsa.misc.im_userid']},
    'lhost': {rsa: ['rsa.network.lhost']},
    'min': {rsa: ['rsa.time.min']},
    'origin': {rsa: ['rsa.network.origin']},
    'pid': {rsa: ['rsa.misc.pid']},
    'priority': {rsa: ['rsa.misc.priority']},
    'remote_domain_id': {rsa: ['rsa.network.remote_domain_id']},
    's_certauth': {rsa: ['rsa.crypto.s_certauth']},
    'timestamp': {rsa: ['rsa.time.timestamp']},
    'urldomain': {ecs: ['url.domain']},
    'attachment': {rsa: ['rsa.file.attachment']},
    's_context': {rsa: ['rsa.misc.context_subject']},
    't_context': {rsa: ['rsa.misc.context_target']},
    'cve': {rsa: ['rsa.misc.cve']},
    'dclass_counter2_string': {rsa: ['rsa.counters.dclass_c2_str']},
    'dclass_ratio1_string': {rsa: ['rsa.counters.dclass_r1_str']},
    'dclass_ratio2': {rsa: ['rsa.counters.dclass_r2']},
    'event_queue_time': {convert: to_date, rsa: ['rsa.time.event_queue_time']},
    'web_extension': {ecs: ['file.extension']},
    'fcatnum': {rsa: ['rsa.misc.fcatnum']},
    'federated_idp': {rsa: ['rsa.identity.federated_idp']},
    'h_code': {rsa: ['rsa.internal.hcode']},
    'ike_cookie1': {rsa: ['rsa.crypto.ike_cookie1']},
    'ike_cookie2': {rsa: ['rsa.crypto.ike_cookie2']},
    'inode': {convert: to_long, rsa: ['rsa.internal.inode']},
    'hostip_v6': {convert: to_ip, ecs: ['host.ip']},
    'library': {rsa: ['rsa.misc.library']},
    'location_state': {ecs: ['geo.region_name']},
    'lread': {convert: to_long, rsa: ['rsa.db.lread']},
    'lwrite': {convert: to_long, rsa: ['rsa.db.lwrite']},
    'parent_node': {rsa: ['rsa.misc.parent_node']},
    'phone_number': {rsa: ['rsa.misc.phone']},
    'pwwn': {rsa: ['rsa.storage.pwwn']},
    'referer': {ecs: ['http.request.referrer']},
    'resource_class': {rsa: ['rsa.internal.resource_class']},
    'risk_info': {rsa: ['rsa.misc.risk_info']},
    'tcp_flags': {convert: to_long, rsa: ['rsa.misc.tcp_flags']},
    'tos': {convert: to_long, rsa: ['rsa.misc.tos']},
    'trans_to': {rsa: ['rsa.email.trans_to']},
    'user': {ecs: ['user.name']},
    'vm_target': {rsa: ['rsa.misc.vm_target']},
    'workspace_desc': {rsa: ['rsa.misc.workspace']},
    'addr': {rsa: ['rsa.network.addr']},
    'cn_asn_dst': {rsa: ['rsa.web.cn_asn_dst']},
    'cn_rpackets': {rsa: ['rsa.web.cn_rpackets']},
    'command': {rsa: ['rsa.misc.command']},
    'dns_a_record': {rsa: ['rsa.network.dns_a_record']},
    'dns_ptr_record': {rsa: ['rsa.network.dns_ptr_record']},
    'event_category': {rsa: ['rsa.misc.event_category']},
    'facilityname': {rsa: ['rsa.misc.facilityname']},
    'fhost': {rsa: ['rsa.network.fhost']},
    'filepath': {ecs: ['file.path']},
    'filesystem': {rsa: ['rsa.file.filesystem']},
    'forensic_info': {rsa: ['rsa.misc.forensic_info']},
    'fport': {rsa: ['rsa.network.fport']},
    'jobname': {rsa: ['rsa.misc.jobname']},
    'laddr': {rsa: ['rsa.network.laddr']},
    'linterface': {rsa: ['rsa.network.linterface']},
    'mode': {rsa: ['rsa.misc.mode']},
    'p_time1': {rsa: ['rsa.time.p_time1']},
    'phost': {rsa: ['rsa.network.phost']},
    'policy': {rsa: ['rsa.misc.policy']},
    'policy_waiver': {rsa: ['rsa.misc.policy_waiver']},
    'second': {rsa: ['rsa.misc.second']},
    'space1': {rsa: ['rsa.misc.space1']},
    'subcategory': {rsa: ['rsa.misc.subcategory']},
    'tbdstr2': {rsa: ['rsa.misc.tbdstr2']},
    'tzone': {rsa: ['rsa.time.tzone']},
    'urlpage': {rsa: ['rsa.web.urlpage']},
    'urlquery': {ecs: ['url.query']},
    'urlroot': {rsa: ['rsa.web.urlroot']},
    'user_id': {ecs: ['user.id']},
    'ad_computer_dst': {rsa: ['rsa.network.ad_computer_dst']},
    'alert': {rsa: ['rsa.threat.alert']},
    'alert_id': {rsa: ['rsa.misc.alert_id']},
    'devicehostname': {rsa: ['rsa.network.alias_host']},
    'binary': {rsa: ['rsa.file.binary']},
    'cert_checksum': {rsa: ['rsa.crypto.cert_checksum']},
    'cert_hostname_cat': {rsa: ['rsa.crypto.cert_host_cat']},
    'cert.serial': {rsa: ['rsa.crypto.cert_serial']},
    'cert_status': {rsa: ['rsa.crypto.cert_status']},
    'checksum.dst': {rsa: ['rsa.misc.checksum_dst']},
    'checksum.src': {rsa: ['rsa.misc.checksum_src']},
    'dclass_counter3_string': {rsa: ['rsa.counters.dclass_c3_str']},
    'dclass_ratio3': {rsa: ['rsa.counters.dclass_r3']},
    'dead': {convert: to_long, rsa: ['rsa.internal.dead']},
    'dns.resptext': {ecs: ['dns.answers.name']},
    'domainname': {ecs: ['server.domain']},
    'bcc': {rsa: ['rsa.email.email']},
    'email': {rsa: ['rsa.email.email']},
    'eth_type': {convert: to_long, rsa: ['rsa.network.eth_type']},
    'extension': {ecs: ['file.extension']},
    'feed_desc': {rsa: ['rsa.internal.feed_desc']},
    'feed_name': {rsa: ['rsa.internal.feed_name']},
    'filename_dst': {rsa: ['rsa.file.filename_dst']},
    'filename_src': {rsa: ['rsa.file.filename_src']},
    'fresult': {convert: to_long, rsa: ['rsa.misc.fresult']},
    'patient_fullname': {ecs: ['user.full_name']},
    'ip_proto': {convert: to_long, rsa: ['rsa.network.ip_proto']},
    'latdec_dst': {convert: to_double, ecs: ['destination.geo.location.lat']},
    'latdec_src': {convert: to_double, ecs: ['source.geo.location.lat']},
    'logon_type_desc': {rsa: ['rsa.identity.logon_type_desc']},
    'longdec_src': {convert: to_double, ecs: ['source.geo.location.lon']},
    'user_mname': {rsa: ['rsa.identity.middlename']},
    'org_dst': {rsa: ['rsa.physical.org_dst']},
    'orig_ip': {ecs: ['network.forwarded_ip']},
    'password': {rsa: ['rsa.identity.password']},
    'patient_fname': {rsa: ['rsa.healthcare.patient_fname']},
    'patient_id': {rsa: ['rsa.healthcare.patient_id']},
    'patient_lname': {rsa: ['rsa.healthcare.patient_lname']},
    'patient_mname': {rsa: ['rsa.healthcare.patient_mname']},
    'dst_payload': {rsa: ['rsa.misc.payload_dst']},
    'src_payload': {rsa: ['rsa.misc.payload_src']},
    'pool_id': {rsa: ['rsa.misc.pool_id']},
    'pread': {convert: to_long, rsa: ['rsa.db.pread']},
    'process_id_val': {rsa: ['rsa.misc.process_id_val']},
    'risk_num_comm': {convert: to_double, rsa: ['rsa.misc.risk_num_comm']},
    'risk_num_next': {convert: to_double, rsa: ['rsa.misc.risk_num_next']},
    'risk_num_sand': {convert: to_double, rsa: ['rsa.misc.risk_num_sand']},
    'risk_num_static': {convert: to_double, rsa: ['rsa.misc.risk_num_static']},
    'risk_suspicious': {rsa: ['rsa.misc.risk_suspicious']},
    'risk_warning': {rsa: ['rsa.misc.risk_warning']},
    'service.name': {ecs: ['service.name']},
    'snmp.oid': {rsa: ['rsa.misc.snmp_oid']},
    'sql': {rsa: ['rsa.misc.sql']},
    'd_sslver': {rsa: ['rsa.crypto.ssl_ver_dst']},
    'threat_source': {rsa: ['rsa.threat.threat_source']},
    'url_raw': {ecs: ['url.original']},
    'user.id': {ecs: ['user.id']},
    'vuln_ref': {rsa: ['rsa.misc.vuln_ref']},
    'acl_id': {rsa: ['rsa.misc.acl_id']},
    'acl_op': {rsa: ['rsa.misc.acl_op']},
    'acl_pos': {rsa: ['rsa.misc.acl_pos']},
    'acl_table': {rsa: ['rsa.misc.acl_table']},
    'admin': {rsa: ['rsa.misc.admin']},
    'alarm_id': {rsa: ['rsa.misc.alarm_id']},
    'alarmname': {rsa: ['rsa.misc.alarmname']},
    'app_id': {rsa: ['rsa.misc.app_id']},
    'audit': {rsa: ['rsa.misc.audit']},
    'audit_object': {rsa: ['rsa.misc.audit_object']},
    'auditdata': {rsa: ['rsa.misc.auditdata']},
    'benchmark': {rsa: ['rsa.misc.benchmark']},
    'bypass': {rsa: ['rsa.misc.bypass']},
    'c_logon_id': {ecs: ['user.id']},
    'cache': {rsa: ['rsa.misc.cache']},
    'cache_hit': {rsa: ['rsa.misc.cache_hit']},
    'cefversion': {rsa: ['rsa.misc.cefversion']},
    'cert_keysize': {rsa: ['rsa.crypto.cert_keysize']},
    'cert_username': {rsa: ['rsa.crypto.cert_username']},
    'cfg.attr': {rsa: ['rsa.misc.cfg_attr']},
    'cfg.obj': {rsa: ['rsa.misc.cfg_obj']},
    'cfg.path': {rsa: ['rsa.misc.cfg_path']},
    'changes': {rsa: ['rsa.misc.changes']},
    'client': {rsa: ['rsa.misc.client']},
    'client_ip': {rsa: ['rsa.misc.client_ip']},
    'clustermembers': {rsa: ['rsa.misc.clustermembers']},
    'cn_acttimeout': {rsa: ['rsa.misc.cn_acttimeout']},
    'cn_asn_src': {rsa: ['rsa.misc.cn_asn_src']},
    'cn_bgpv4nxthop': {rsa: ['rsa.misc.cn_bgpv4nxthop']},
    'cn_ctr_dst_code': {rsa: ['rsa.misc.cn_ctr_dst_code']},
    'cn_dst_tos': {rsa: ['rsa.misc.cn_dst_tos']},
    'cn_dst_vlan': {rsa: ['rsa.misc.cn_dst_vlan']},
    'cn_engine_id': {rsa: ['rsa.misc.cn_engine_id']},
    'cn_engine_type': {rsa: ['rsa.misc.cn_engine_type']},
    'cn_f_switch': {rsa: ['rsa.misc.cn_f_switch']},
    'cn_flowsampid': {rsa: ['rsa.misc.cn_flowsampid']},
    'cn_flowsampintv': {rsa: ['rsa.misc.cn_flowsampintv']},
    'cn_flowsampmode': {rsa: ['rsa.misc.cn_flowsampmode']},
    'cn_inacttimeout': {rsa: ['rsa.misc.cn_inacttimeout']},
    'cn_inpermbyts': {rsa: ['rsa.misc.cn_inpermbyts']},
    'cn_inpermpckts': {rsa: ['rsa.misc.cn_inpermpckts']},
    'cn_invalid': {rsa: ['rsa.misc.cn_invalid']},
    'cn_ip_proto_ver': {rsa: ['rsa.misc.cn_ip_proto_ver']},
    'cn_ipv4_ident': {rsa: ['rsa.misc.cn_ipv4_ident']},
    'cn_l_switch': {rsa: ['rsa.misc.cn_l_switch']},
    'cn_log_did': {rsa: ['rsa.misc.cn_log_did']},
    'cn_log_rid': {rsa: ['rsa.misc.cn_log_rid']},
    'cn_max_ttl': {rsa: ['rsa.misc.cn_max_ttl']},
    'cn_maxpcktlen': {rsa: ['rsa.misc.cn_maxpcktlen']},
    'cn_min_ttl': {rsa: ['rsa.misc.cn_min_ttl']},
    'cn_minpcktlen': {rsa: ['rsa.misc.cn_minpcktlen']},
    'cn_mpls_lbl_1': {rsa: ['rsa.misc.cn_mpls_lbl_1']},
    'cn_mpls_lbl_10': {rsa: ['rsa.misc.cn_mpls_lbl_10']},
    'cn_mpls_lbl_2': {rsa: ['rsa.misc.cn_mpls_lbl_2']},
    'cn_mpls_lbl_3': {rsa: ['rsa.misc.cn_mpls_lbl_3']},
    'cn_mpls_lbl_4': {rsa: ['rsa.misc.cn_mpls_lbl_4']},
    'cn_mpls_lbl_5': {rsa: ['rsa.misc.cn_mpls_lbl_5']},
    'cn_mpls_lbl_6': {rsa: ['rsa.misc.cn_mpls_lbl_6']},
    'cn_mpls_lbl_7': {rsa: ['rsa.misc.cn_mpls_lbl_7']},
    'cn_mpls_lbl_8': {rsa: ['rsa.misc.cn_mpls_lbl_8']},
    'cn_mpls_lbl_9': {rsa: ['rsa.misc.cn_mpls_lbl_9']},
    'cn_mplstoplabel': {rsa: ['rsa.misc.cn_mplstoplabel']},
    'cn_mplstoplabip': {rsa: ['rsa.misc.cn_mplstoplabip']},
    'cn_mul_dst_byt': {rsa: ['rsa.misc.cn_mul_dst_byt']},
    'cn_mul_dst_pks': {rsa: ['rsa.misc.cn_mul_dst_pks']},
    'cn_muligmptype': {rsa: ['rsa.misc.cn_muligmptype']},
    'cn_sampalgo': {rsa: ['rsa.misc.cn_sampalgo']},
    'cn_sampint': {rsa: ['rsa.misc.cn_sampint']},
    'cn_seqctr': {rsa: ['rsa.misc.cn_seqctr']},
    'cn_spackets': {rsa: ['rsa.misc.cn_spackets']},
    'cn_src_tos': {rsa: ['rsa.misc.cn_src_tos']},
    'cn_src_vlan': {rsa: ['rsa.misc.cn_src_vlan']},
    'cn_sysuptime': {rsa: ['rsa.misc.cn_sysuptime']},
    'cn_template_id': {rsa: ['rsa.misc.cn_template_id']},
    'cn_totbytsexp': {rsa: ['rsa.misc.cn_totbytsexp']},
    'cn_totflowexp': {rsa: ['rsa.misc.cn_totflowexp']},
    'cn_totpcktsexp': {rsa: ['rsa.misc.cn_totpcktsexp']},
    'cn_unixnanosecs': {rsa: ['rsa.misc.cn_unixnanosecs']},
    'cn_v6flowlabel': {rsa: ['rsa.misc.cn_v6flowlabel']},
    'cn_v6optheaders': {rsa: ['rsa.misc.cn_v6optheaders']},
    'comp_class': {rsa: ['rsa.misc.comp_class']},
    'comp_name': {rsa: ['rsa.misc.comp_name']},
    'comp_rbytes': {rsa: ['rsa.misc.comp_rbytes']},
    'comp_sbytes': {rsa: ['rsa.misc.comp_sbytes']},
    'connection_id': {rsa: ['rsa.misc.connection_id']},
    'cpu_data': {rsa: ['rsa.misc.cpu_data']},
    'criticality': {rsa: ['rsa.misc.criticality']},
    'cs_agency_dst': {rsa: ['rsa.misc.cs_agency_dst']},
    'cs_analyzedby': {rsa: ['rsa.misc.cs_analyzedby']},
    'cs_av_other': {rsa: ['rsa.misc.cs_av_other']},
    'cs_av_primary': {rsa: ['rsa.misc.cs_av_primary']},
    'cs_av_secondary': {rsa: ['rsa.misc.cs_av_secondary']},
    'cs_bgpv6nxthop': {rsa: ['rsa.misc.cs_bgpv6nxthop']},
    'cs_bit9status': {rsa: ['rsa.misc.cs_bit9status']},
    'cs_context': {rsa: ['rsa.misc.cs_context']},
    'cs_control': {rsa: ['rsa.misc.cs_control']},
    'cs_data': {rsa: ['rsa.misc.cs_data']},
    'cs_datecret': {rsa: ['rsa.misc.cs_datecret']},
    'cs_dst_tld': {rsa: ['rsa.misc.cs_dst_tld']},
    'cs_eth_dst_ven': {rsa: ['rsa.misc.cs_eth_dst_ven']},
    'cs_eth_src_ven': {rsa: ['rsa.misc.cs_eth_src_ven']},
    'cs_event_uuid': {rsa: ['rsa.misc.cs_event_uuid']},
    'cs_filetype': {rsa: ['rsa.misc.cs_filetype']},
    'cs_fld': {rsa: ['rsa.misc.cs_fld']},
    'cs_if_desc': {rsa: ['rsa.misc.cs_if_desc']},
    'cs_if_name': {rsa: ['rsa.misc.cs_if_name']},
    'cs_ip_next_hop': {rsa: ['rsa.misc.cs_ip_next_hop']},
    'cs_ipv4dstpre': {rsa: ['rsa.misc.cs_ipv4dstpre']},
    'cs_ipv4srcpre': {rsa: ['rsa.misc.cs_ipv4srcpre']},
    'cs_lifetime': {rsa: ['rsa.misc.cs_lifetime']},
    'cs_log_medium': {rsa: ['rsa.misc.cs_log_medium']},
    'cs_loginname': {rsa: ['rsa.misc.cs_loginname']},
    'cs_modulescore': {rsa: ['rsa.misc.cs_modulescore']},
    'cs_modulesign': {rsa: ['rsa.misc.cs_modulesign']},
    'cs_opswatresult': {rsa: ['rsa.misc.cs_opswatresult']},
    'cs_payload': {rsa: ['rsa.misc.cs_payload']},
    'cs_registrant': {rsa: ['rsa.misc.cs_registrant']},
    'cs_registrar': {rsa: ['rsa.misc.cs_registrar']},
    'cs_represult': {rsa: ['rsa.misc.cs_represult']},
    'cs_rpayload': {rsa: ['rsa.misc.cs_rpayload']},
    'cs_sampler_name': {rsa: ['rsa.misc.cs_sampler_name']},
    'cs_sourcemodule': {rsa: ['rsa.misc.cs_sourcemodule']},
    'cs_streams': {rsa: ['rsa.misc.cs_streams']},
    'cs_targetmodule': {rsa: ['rsa.misc.cs_targetmodule']},
    'cs_v6nxthop': {rsa: ['rsa.misc.cs_v6nxthop']},
    'cs_whois_server': {rsa: ['rsa.misc.cs_whois_server']},
    'cs_yararesult': {rsa: ['rsa.misc.cs_yararesult']},
    'description': {rsa: ['rsa.misc.description']},
    'devvendor': {rsa: ['rsa.misc.devvendor']},
    'distance': {rsa: ['rsa.misc.distance']},
    'dns_cname_record': {rsa: ['rsa.network.dns_cname_record']},
    'dns_id': {rsa: ['rsa.network.dns_id']},
    'dns_opcode': {rsa: ['rsa.network.dns_opcode']},
    'dns_resp': {rsa: ['rsa.network.dns_resp']},
    'dns_type': {rsa: ['rsa.network.dns_type']},
    'domain1': {rsa: ['rsa.network.domain1']},
    'dstburb': {rsa: ['rsa.misc.dstburb']},
    'edomain': {rsa: ['rsa.misc.edomain']},
    'edomaub': {rsa: ['rsa.misc.edomaub']},
    'euid': {rsa: ['rsa.misc.euid']},
    'event_time_str': {rsa: ['rsa.time.event_time_str']},
    'eventtime': {rsa: ['rsa.time.eventtime']},
    'facility': {rsa: ['rsa.misc.facility']},
    'filename_tmp': {rsa: ['rsa.file.filename_tmp']},
    'finterface': {rsa: ['rsa.misc.finterface']},
    'flags': {rsa: ['rsa.misc.flags']},
    'gaddr': {rsa: ['rsa.misc.gaddr']},
    'gmtdate': {rsa: ['rsa.time.gmtdate']},
    'gmttime': {rsa: ['rsa.time.gmttime']},
    'host.type': {rsa: ['rsa.network.host_type']},
    'https.insact': {rsa: ['rsa.crypto.https_insact']},
    'https.valid': {rsa: ['rsa.crypto.https_valid']},
    'id3': {rsa: ['rsa.misc.id3']},
    'im_buddyname': {rsa: ['rsa.misc.im_buddyname']},
    'im_croomid': {rsa: ['rsa.misc.im_croomid']},
    'im_croomtype': {rsa: ['rsa.misc.im_croomtype']},
    'im_members': {rsa: ['rsa.misc.im_members']},
    'im_username': {rsa: ['rsa.misc.im_username']},
    'ipkt': {rsa: ['rsa.misc.ipkt']},
    'ipscat': {rsa: ['rsa.misc.ipscat']},
    'ipspri': {rsa: ['rsa.misc.ipspri']},
    'latitude': {rsa: ['rsa.misc.latitude']},
    'linenum': {rsa: ['rsa.misc.linenum']},
    'list_name': {rsa: ['rsa.misc.list_name']},
    'load_data': {rsa: ['rsa.misc.load_data']},
    'location_floor': {rsa: ['rsa.misc.location_floor']},
    'location_mark': {rsa: ['rsa.misc.location_mark']},
    'log_id': {rsa: ['rsa.misc.log_id']},
    'log_type': {rsa: ['rsa.misc.log_type']},
    'logid': {rsa: ['rsa.misc.logid']},
    'logip': {rsa: ['rsa.misc.logip']},
    'logname': {rsa: ['rsa.misc.logname']},
    'longitude': {rsa: ['rsa.misc.longitude']},
    'lport': {rsa: ['rsa.misc.lport']},
    'mbug_data': {rsa: ['rsa.misc.mbug_data']},
    'misc_name': {rsa: ['rsa.misc.misc_name']},
    'msg_type': {rsa: ['rsa.misc.msg_type']},
    'msgid': {rsa: ['rsa.misc.msgid']},
    'netsessid': {rsa: ['rsa.misc.netsessid']},
    'num': {rsa: ['rsa.misc.num']},
    'number1': {rsa: ['rsa.misc.number1']},
    'number2': {rsa: ['rsa.misc.number2']},
    'nwwn': {rsa: ['rsa.misc.nwwn']},
    'object': {rsa: ['rsa.misc.object']},
    'operation': {rsa: ['rsa.misc.operation']},
    'opkt': {rsa: ['rsa.misc.opkt']},
    'orig_from': {rsa: ['rsa.misc.orig_from']},
    'owner_id': {rsa: ['rsa.misc.owner_id']},
    'p_action': {rsa: ['rsa.misc.p_action']},
    'p_date': {rsa: ['rsa.time.p_date']},
    'p_filter': {rsa: ['rsa.misc.p_filter']},
    'p_group_object': {rsa: ['rsa.misc.p_group_object']},
    'p_id': {rsa: ['rsa.misc.p_id']},
    'p_month': {rsa: ['rsa.time.p_month']},
    'p_msgid1': {rsa: ['rsa.misc.p_msgid1']},
    'p_msgid2': {rsa: ['rsa.misc.p_msgid2']},
    'p_result1': {rsa: ['rsa.misc.p_result1']},
    'p_time': {rsa: ['rsa.time.p_time']},
    'p_time2': {rsa: ['rsa.time.p_time2']},
    'p_url': {rsa: ['rsa.web.p_url']},
    'p_user_agent': {rsa: ['rsa.web.p_user_agent']},
    'p_web_cookie': {rsa: ['rsa.web.p_web_cookie']},
    'p_web_method': {rsa: ['rsa.web.p_web_method']},
    'p_web_referer': {rsa: ['rsa.web.p_web_referer']},
    'p_year': {rsa: ['rsa.time.p_year']},
    'packet_length': {rsa: ['rsa.network.packet_length']},
    'password_chg': {rsa: ['rsa.misc.password_chg']},
    'password_expire': {rsa: ['rsa.misc.password_expire']},
    'permgranted': {rsa: ['rsa.misc.permgranted']},
    'permwanted': {rsa: ['rsa.misc.permwanted']},
    'pgid': {rsa: ['rsa.misc.pgid']},
    'policyUUID': {rsa: ['rsa.misc.policyUUID']},
    'prog_asp_num': {rsa: ['rsa.misc.prog_asp_num']},
    'program': {rsa: ['rsa.misc.program']},
    'real_data': {rsa: ['rsa.misc.real_data']},
    'rec_asp_device': {rsa: ['rsa.misc.rec_asp_device']},
    'rec_asp_num': {rsa: ['rsa.misc.rec_asp_num']},
    'rec_library': {rsa: ['rsa.misc.rec_library']},
    'recordnum': {rsa: ['rsa.misc.recordnum']},
    'result_code': {rsa: ['rsa.misc.result_code']},
    'ruid': {rsa: ['rsa.misc.ruid']},
    'sburb': {rsa: ['rsa.misc.sburb']},
    'sdomain_fld': {rsa: ['rsa.misc.sdomain_fld']},
    'sec': {rsa: ['rsa.misc.sec']},
    'sensorname': {rsa: ['rsa.misc.sensorname']},
    'seqnum': {rsa: ['rsa.misc.seqnum']},
    'session': {rsa: ['rsa.misc.session']},
    'sessiontype': {rsa: ['rsa.misc.sessiontype']},
    'sigUUID': {rsa: ['rsa.misc.sigUUID']},
    'spi': {rsa: ['rsa.misc.spi']},
    'srcburb': {rsa: ['rsa.misc.srcburb']},
    'srcdom': {rsa: ['rsa.misc.srcdom']},
    'srcservice': {rsa: ['rsa.misc.srcservice']},
    'state': {rsa: ['rsa.misc.state']},
    'status1': {rsa: ['rsa.misc.status1']},
    'svcno': {rsa: ['rsa.misc.svcno']},
    'system': {rsa: ['rsa.misc.system']},
    'tbdstr1': {rsa: ['rsa.misc.tbdstr1']},
    'tgtdom': {rsa: ['rsa.misc.tgtdom']},
    'tgtdomain': {rsa: ['rsa.misc.tgtdomain']},
    'threshold': {rsa: ['rsa.misc.threshold']},
    'type1': {rsa: ['rsa.misc.type1']},
    'udb_class': {rsa: ['rsa.misc.udb_class']},
    'url_fld': {rsa: ['rsa.misc.url_fld']},
    'user_div': {rsa: ['rsa.misc.user_div']},
    'userid': {rsa: ['rsa.misc.userid']},
    'username_fld': {rsa: ['rsa.misc.username_fld']},
    'utcstamp': {rsa: ['rsa.misc.utcstamp']},
    'v_instafname': {rsa: ['rsa.misc.v_instafname']},
    'virt_data': {rsa: ['rsa.misc.virt_data']},
    'vpnid': {rsa: ['rsa.misc.vpnid']},
    'web_extension_tmp': {rsa: ['rsa.web.web_extension_tmp']},
    'web_page': {rsa: ['rsa.web.web_page']},
    'alias.host': {rsa: ['rsa.network.alias_host']},
    'workstation': {rsa: ['rsa.network.alias_host']},
    'devicehostip': {convert: to_ip, ecs: ['host.ip']},
    'alias.ip': {convert: to_ip, ecs: ['host.ip']},
    'alias.ipv6': {convert: to_ip, ecs: ['host.ip']},
    'devicehostmac': {convert: to_mac, ecs: ['host.mac']},
    'alias.mac': {convert: to_mac, ecs: ['host.mac']},
    'analysis.file': {rsa: ['rsa.investigations.analysis_file']},
    'analysis.service': {rsa: ['rsa.investigations.analysis_service']},
    'analysis.session': {rsa: ['rsa.investigations.analysis_session']},
    'autorun_type': {rsa: ['rsa.misc.autorun_type']},
    'boc': {rsa: ['rsa.investigations.boc']},
    'cc.number': {convert: to_long, rsa: ['rsa.misc.cc_number']},
    'cctld': {ecs: ['url.top_level_domain']},
    'cert_ca': {rsa: ['rsa.crypto.cert_ca']},
    'cert_common': {rsa: ['rsa.crypto.cert_common']},
    'child_pid_val': {ecs: ['process.title']},
    'cid': {rsa: ['rsa.internal.cid']},
    'city.dst': {ecs: ['destination.geo.city_name']},
    'city.src': {ecs: ['source.geo.city_name']},
    'content': {rsa: ['rsa.misc.content']},
    'dclass_ratio2_string': {rsa: ['rsa.counters.dclass_r2_str']},
    'dclass_ratio3_string': {rsa: ['rsa.counters.dclass_r3_str']},
    'device.class': {rsa: ['rsa.internal.device_class']},
    'device.group': {rsa: ['rsa.internal.device_group']},
    'device.host': {rsa: ['rsa.internal.device_host']},
    'device.ip': {convert: to_ip, rsa: ['rsa.internal.device_ip']},
    'device.ipv6': {convert: to_ip, rsa: ['rsa.internal.device_ipv6']},
    'device.type': {rsa: ['rsa.internal.device_type']},
    'device.type.id': {convert: to_long, rsa: ['rsa.internal.device_type_id']},
    'did': {rsa: ['rsa.internal.did']},
    'directory.dst': {rsa: ['rsa.file.directory_dst']},
    'directory.src': {rsa: ['rsa.file.directory_src']},
    'dns.responsetype': {ecs: ['dns.answers.type']},
    'domain.dst': {ecs: ['destination.domain']},
    'domain.src': {ecs: ['source.domain']},
    'ein.number': {convert: to_long, rsa: ['rsa.misc.ein_number']},
    'entropy.req': {convert: to_long, rsa: ['rsa.internal.entropy_req']},
    'entropy.res': {convert: to_long, rsa: ['rsa.internal.entropy_res']},
    'eoc': {rsa: ['rsa.investigations.eoc']},
    'event.cat': {convert: to_long, rsa: ['rsa.investigations.event_cat']},
    'event.cat.name': {rsa: ['rsa.investigations.event_cat_name']},
    'event_name': {rsa: ['rsa.internal.event_name']},
    'expiration_time_string': {rsa: ['rsa.time.expire_time_str']},
    'feed.category': {rsa: ['rsa.internal.feed_category']},
    'file.attributes': {ecs: ['file.attributes']},
    'file_entropy': {convert: to_double, rsa: ['rsa.file.file_entropy']},
    'file_vendor': {rsa: ['rsa.file.file_vendor']},
    'forward.ip': {convert: to_ip, rsa: ['rsa.internal.forward_ip']},
    'forward.ipv6': {convert: to_ip, rsa: ['rsa.internal.forward_ipv6']},
    'found': {rsa: ['rsa.misc.found']},
    'header.id': {rsa: ['rsa.internal.header_id']},
    'host.orig': {rsa: ['rsa.network.host_orig']},
    'host_role': {rsa: ['rsa.identity.host_role']},
    'host.state': {rsa: ['rsa.endpoint.host_state']},
    'inv.category': {rsa: ['rsa.investigations.inv_category']},
    'inv.context': {rsa: ['rsa.investigations.inv_context']},
    'ioc': {rsa: ['rsa.investigations.ioc']},
    'ip.trans.dst': {convert: to_ip, ecs: ['destination.nat.ip']},
    'ip.trans.src': {convert: to_ip, ecs: ['source.nat.ip']},
    'ipv6.orig': {convert: to_ip, ecs: ['network.forwarded_ip']},
    'language': {rsa: ['rsa.misc.language']},
    'lc.cid': {rsa: ['rsa.internal.lc_cid']},
    'lc.ctime': {convert: to_date, rsa: ['rsa.internal.lc_ctime']},
    'ldap': {rsa: ['rsa.identity.ldap']},
    'ldap.query': {rsa: ['rsa.identity.ldap_query']},
    'ldap.response': {rsa: ['rsa.identity.ldap_response']},
    'lifetime': {convert: to_long, rsa: ['rsa.misc.lifetime']},
    'link': {rsa: ['rsa.misc.link']},
    'longdec_dst': {convert: to_double, ecs: ['destination.geo.location.lon']},
    'match': {rsa: ['rsa.misc.match']},
    'mcb.req': {convert: to_long, rsa: ['rsa.internal.mcb_req']},
    'mcb.res': {convert: to_long, rsa: ['rsa.internal.mcb_res']},
    'mcbc.req': {convert: to_long, rsa: ['rsa.internal.mcbc_req']},
    'mcbc.res': {convert: to_long, rsa: ['rsa.internal.mcbc_res']},
    'medium': {convert: to_long, rsa: ['rsa.internal.medium']},
    'nodename': {rsa: ['rsa.internal.node_name']},
    'nwe.callback_id': {rsa: ['rsa.internal.nwe_callback_id']},
    'org.dst': {rsa: ['rsa.physical.org_dst']},
    'org.src': {rsa: ['rsa.physical.org_src']},
    'original_owner': {rsa: ['rsa.identity.owner']},
    'param.dst': {rsa: ['rsa.misc.param_dst']},
    'param.src': {rsa: ['rsa.misc.param_src']},
    'parent_pid_val': {ecs: ['process.parent.title']},
    'parse.error': {rsa: ['rsa.internal.parse_error']},
    'payload.req': {convert: to_long, rsa: ['rsa.internal.payload_req']},
    'payload.res': {convert: to_long, rsa: ['rsa.internal.payload_res']},
    'port.dst': {convert: to_long, ecs: ['destination.port']},
    'port.src': {convert: to_long, ecs: ['source.port']},
    'port.trans.dst': {convert: to_long, ecs: ['destination.nat.port']},
    'port.trans.src': {convert: to_long, ecs: ['source.nat.port']},
    'process.vid.dst': {rsa: ['rsa.internal.process_vid_dst']},
    'process.vid.src': {rsa: ['rsa.internal.process_vid_src']},
    'query': {ecs: ['url.query']},
    'registry.key': {rsa: ['rsa.endpoint.registry_key']},
    'registry.value': {rsa: ['rsa.endpoint.registry_value']},
    'rid': {convert: to_long, rsa: ['rsa.internal.rid']},
    'rpayload': {rsa: ['rsa.network.rpayload']},
    'search.text': {rsa: ['rsa.misc.search_text']},
    'service.account': {rsa: ['rsa.identity.service_account']},
    'session.split': {rsa: ['rsa.internal.session_split']},
    'sig.name': {rsa: ['rsa.misc.sig_name']},
    'site': {rsa: ['rsa.internal.site']},
    'size': {convert: to_long, rsa: ['rsa.internal.size']},
    'sld': {ecs: ['url.registered_domain']},
    'snmp.value': {rsa: ['rsa.misc.snmp_value']},
    'sourcefile': {rsa: ['rsa.internal.sourcefile']},
    'stamp': {convert: to_date, rsa: ['rsa.time.stamp']},
    'streams': {convert: to_long, rsa: ['rsa.misc.streams']},
    'task_name': {rsa: ['rsa.file.task_name']},
    'tcp.dstport': {convert: to_long, ecs: ['destination.port']},
    'tcp.srcport': {convert: to_long, ecs: ['source.port']},
    'tld': {ecs: ['url.top_level_domain']},
    'ubc.req': {convert: to_long, rsa: ['rsa.internal.ubc_req']},
    'ubc.res': {convert: to_long, rsa: ['rsa.internal.ubc_res']},
    'udp.dstport': {convert: to_long, ecs: ['destination.port']},
    'udp.srcport': {convert: to_long, ecs: ['source.port']},
    'vlan.name': {rsa: ['rsa.network.vlan_name']},
    'word': {rsa: ['rsa.internal.word']},
}

function to_date(value) {
    switch (typeof (value)) {
        case "object":
            // This is a Date. But as it was obtained from evt.Get(), the VM
            // doesn't see it as a JS Date anymore, thus value instanceof Date === false.
            // Have to trust that any object here is a valid Date for Go.
            return value;
        case "string":
            var asDate = new Date(value);
            if (!isNaN(asDate)) return asDate;
    }
}

// ECMAScript 5.1 doesn't have Object.MAX_SAFE_INTEGER / Object.MIN_SAFE_INTEGER.
var maxSafeInt = Math.pow(2, 53) - 1;
var minSafeInt = -maxSafeInt;

function to_long(value) {
    var num = parseInt(value);
    // Better not to index a number if it's not safe (above 53 bits).
    return !isNaN(num) && minSafeInt <= num && num <= maxSafeInt ? num : undefined;
}

function to_ip(value) {
    if (value.indexOf(":") === -1)
        return to_ipv4(value);
    return to_ipv6(value);
}

var ipv4_regex = /^(\d+)\.(\d+)\.(\d+)\.(\d+)$/;
var ipv6_hex_regex = /^[0-9A-Fa-f]{1,4}$/;

function to_ipv4(value) {
    var result = ipv4_regex.exec(value);
    if (result == null || result.length !== 5) return;
    for (var i = 1; i < 5; i++) {
        var num = parseInt(result[i]);
        if (isNaN(num) || num < 0 || num > 255) return;
    }
    return value;
}

function to_ipv6(value) {
    var sqEnd = value.indexOf("]");
    if (sqEnd > -1) {
        if (value.charAt(0) != '[') return;
        value = value.substr(1, sqEnd - 1);
    }
    var zoneOffset = value.indexOf('%');
    if (zoneOffset > -1) {
        value = value.substr(0, zoneOffset);
    }
    var parts = value.split(':');
    if (parts == null || parts.length < 3 || parts.length > 8) return;
    var numEmpty = 0;
    var innerEmpty = 0;
    for (var i = 0; i < parts.length; i++) {
        if (parts[i].length === 0) {
            numEmpty++;
            if (i > 0 && i + 1 < parts.length) innerEmpty++;
        } else if (!parts[i].match(ipv6_hex_regex) &&
            // Accept an IPv6 with a valid IPv4 at the end.
            ((i + 1 < parts.length) || !to_ipv4(parts[i]))) {
            return
        }
    }
    return innerEmpty === 0 && parts.length === 8 || innerEmpty === 1 ? value : undefined;
}

function to_double(value) {
    return parseFloat(value);
}

function to_mac(value) {
    // ES doesn't have a mac datatype so it's safe to ingest whatever was captured.
    return value;
}

function to_lowercase(value) {
    // to_lowercase is used against keyword fields, which can accept
    // any other type (numbers, dates).
    return typeof(value) === 'string'? value.toLowerCase() : value;
}

function map_all(evt, targets, value) {
    for (var i = 0; i < targets.length; i++) {
        evt.Put(targets[i], value);
    }
}

function populate_fields(evt) {
    var base = evt.Get(FIELDS_OBJECT);
    if (base === null) return;
    for (var key in base) {
        if (!base.hasOwnProperty(key)) continue;
        var mapping = field_mappings[key];
        if (mapping !== undefined) {
            var value = base[key];
            if (mapping.convert !== undefined)
                value = mapping.convert(value);
            if (value !== undefined) {
                if (map_ecs && mapping.ecs) map_all(evt, mapping.ecs, value);
                if (map_rsa && mapping.rsa) map_all(evt, mapping.rsa, value);
            } else {
                console.debug("Failed to convert field '" + key + "' = '" + base[key] + "'");
            }
        } else {
            console.debug("No mapping for field '" + key + "'");
        }
    }
    if (keep_raw) {
        evt.Put("rsa.raw", base);
    }
    evt.Delete(FIELDS_OBJECT);
}

function test() {
    test_conversions();
}

function test_conversions() {
    var accept = function (input, output) {
        return {input: input, expected: output !== undefined ? output : input}
    }
    var drop = function (input) {
        return {input: input}
    }
    test_fn_call(to_ip, [
        accept("127.0.0.1"),
        accept("255.255.255.255"),
        accept("008.189.239.199"),
        drop(""),
        drop("not an IP"),
        drop("42"),
        drop("127.0.0.1."),
        drop("127.0.0."),
        drop("10.100.1000.1"),
        accept("fd00:1111:2222:3333:4444:5555:6666:7777"),
        accept("fd00::7777%eth0", "fd00::7777"),
        accept("[fd00::7777]", "fd00::7777"),
        accept("[fd00::7777%enp0s3]", "fd00::7777"),
        accept("::1"),
        accept("::"),
        drop(":::"),
        drop("fff::1::3"),
        accept("ffff::ffff"),
        drop("::1ffff"),
        drop(":1234:"),
        drop("::1234z"),
        accept("1::3:4:5:6:7:8"),
        accept("::255.255.255.255"),
        accept("64:ff9b::192.0.2.33"),
        drop("::255.255.255.255:8"),
    ]);
    test_fn_call(to_long, [
        accept("1234", 1234),
        accept("0x2a", 42),
        drop("9007199254740992"),
        drop("9223372036854775808"),
        drop("NaN"),
        accept("-0x1fffffffffffff", -9007199254740991),
        accept("+9007199254740991", 9007199254740991),
        drop("-0x20000000000000"),
        drop("+9007199254740992"),
        accept(42),
    ]);
    test_fn_call(to_date, [
        {
            input: new Date("2017-10-16T08:30:42Z"),
            expected: "2017-10-16T08:30:42.000Z",
            convert: Date.prototype.toISOString,
        },
        {
            input: "2017-10-16T08:30:42Z",
            expected: new Date("2017-10-16T08:30:42Z").toISOString(),
            convert: Date.prototype.toISOString,
        },
        drop("Not really a date."),
    ]);
    test_fn_call(to_lowercase, [
        accept("Hello", "hello"),
        accept(45),
        accept(Date.now()),
    ]);
}

function test_fn_call(fn, cases) {
    cases.forEach(function (test, idx) {
        var result = fn(test.input);
        if (test.convert !== undefined) {
            result = test.convert.call(result);
        }
        if (result !== test.expected) {
            throw "test " + fn.name + "#" + idx + " failed. Input:'" + test.input + "' Expected:'" + test.expected + "' Got:'" + result + "'";
        }
    });
    if (debug) console.warn("test " + fn.name + " PASS.");
}
