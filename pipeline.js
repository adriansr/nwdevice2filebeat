//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

var processor = require("processor");
var console   = require("console");

var device;

// Register params from configuration.
function register(params) {
    device = new DeviceProcessor();
}

function process(evt) {
    return device.process(evt);
}

function DeviceProcessor() {
	var builder = new processor.Chain();
	builder.Add(save_flags);
	builder.Add(chain1);
	builder.Add(restore_flags);
	var chain = builder.Build();
	return {
		process: chain.Run,
	}
}

var dup0 = match({
	id: "MESSAGE#0:ossec:01/0",
	dissect: {
		tokenizer: "%{hostname->}ossec: Alert Level: %{severity->}; Rule: %{rule->}- %{event_description->}; Location: %{p0->}",
		field: "nwparser.payload",
	},
});

var dup1 = linear_select([
	match({
		id: "MESSAGE#0:ossec:01/1_0",
		dissect: {
			tokenizer: "(%{shost->}) %{saddr->}->%{p1->}",
			field: "nwparser.p0",
		},
	}),
	match({
		id: "MESSAGE#0:ossec:01/1_1",
		dissect: {
			tokenizer: "%{shost->}->%{p1->}",
			field: "nwparser.p0",
		},
	}),
]);

var dup2 = setf("msg","$MSG");

var dup3 = call({
	dest: "nwparser.",
	fn: SYSVAL,
	args: [
		field("$MSGID"),
		field("$ID1"),
	],
});

var hdr1 = match({
	id: "HEADER#0:0001",
	dissect: {
		tokenizer: "%{hfld1->} %{hdate->} %{htime->} %{hfld2->} %{messageid->}: Alert Level: %{hfld3->}; Rule:%{payload->}",
		field: "message",
	},
	on_success: processor_chain([
		set({
			"header_id": "0001",
		}),
		call({
			dest: "nwparser.payload",
			fn: STRCAT,
			args: [
				field("hfld2"),
				constant(" "),
				field("messageid"),
				constant(": Alert Level: "),
				field("hfld3"),
				constant("; Rule:"),
				field("payload"),
			],
		}),
	]),
});

var select1 = linear_select([
	hdr1,
]);

var msg1 = match({
	id: "MESSAGE#0:ossec:01/2",
	dissect: {
		tokenizer: "%{fld1->}/ossec/logs/active-responses.log; %{fld2->} %{fld3->} %{fld4->} %{fld5->} %{timezone->} %{fld7->} %{action->} %{param->}",
		field: "nwparser.p1",
	},
});

var all1 = all_match({
	processors: [
		dup0,
		dup1,
		msg1,
	],
	on_success: processor_chain([
		set({
			"event_log": "/ossec/logs/active-responses.log",
			"eventcategory": "1001020200",
			"msg_id1": "ossec:01",
		}),
		dup2,
		date_time({
			dest: "event_time",
			args: ["fld3","fld4","fld7","fld5"],
			fmt: [dB,dF,dW,dH,dc(":"),dU,dc(":"),dO],
		}),
		dup3,
	]),
});

var msg2 = match({
	id: "MESSAGE#1:ossec:02/2",
	dissect: {
		tokenizer: "%{fld1->}\\ossec-agent\\active-response\\active-responses.log; %{event_time_string->}\"%{action->}\" %{param->}",
		field: "nwparser.p1",
	},
});

var all2 = all_match({
	processors: [
		dup0,
		dup1,
		msg2,
	],
	on_success: processor_chain([
		set({
			"event_log": "\\ossec-agent\\active-response\\active-responses.log",
			"eventcategory": "1001020200",
			"msg_id1": "ossec:02",
		}),
		dup2,
		dup3,
	]),
});

var msg3 = match({
	id: "MESSAGE#2:ossec:03/2",
	dissect: {
		tokenizer: "%{event_log->}; %{info->}",
		field: "nwparser.p1",
	},
});

var all3 = all_match({
	processors: [
		dup0,
		dup1,
		msg3,
	],
	on_success: processor_chain([
		set({
			"eventcategory": "1001000000",
			"msg_id1": "ossec:03",
		}),
		dup2,
		dup3,
	]),
});

var select2 = linear_select([
	all1,
	all2,
	all3,
]);

var chain1 = processor_chain([
	select1,
	msgid_select({
		"ossec": select2,
	}),
	set_field({
		dest: "@timestamp",
		value: field("event_time"),
	}),
]);
