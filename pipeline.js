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
	builder.Add(processor_chain([
		linear_select([
			match({
				dissect: {
					tokenizer: "%{hfld1} %{hdate} %{htime} %{hfld2} %{messageid}: Alert Level: %{hfld3}; Rule:%{payload}",
					field: "message",
				},
				on_success: processor_chain([
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
			}),
		]),
		linear_select([
			all_match({
				processors: [
					dup0,
					dup1,
					match({
						dissect: {
							tokenizer: "%{fld1}/ossec/logs/active-responses.log;  %{fld2} %{fld3} %{fld4} %{fld5} %{timezone} %{fld7} %{action} %{param}",
							field: "nwparser.p1",
						},
					}),
				],
				on_success: processor_chain([
					dup2,
					dup3,
					date_time({
						dest: "event_time",
						args: ["fld3","fld4","fld7","fld5"],
						fmt: [dB,dF,dW,dH,dc(":"),dU,dc(":"),dO],
					}),
					set_field({
						dest: "nwparser.event_log",
						value: constant("/ossec/logs/active-responses.log"),
					}),
					dup4,
				]),
			}),
			all_match({
				processors: [
					dup0,
					dup1,
					match({
						dissect: {
							tokenizer: "%{fld1}\\ossec-agent\\active-response\\active-responses.log; %{event_time_string} \"%{action}\" %{param}",
							field: "nwparser.p1",
						},
					}),
				],
				on_success: processor_chain([
					dup2,
					dup3,
					set_field({
						dest: "nwparser.event_log",
						value: constant("\\ossec-agent\\active-response\\active-responses.log"),
					}),
					dup4,
				]),
			}),
			all_match({
				processors: [
					dup0,
					dup1,
					match({
						dissect: {
							tokenizer: "%{event_log}; %{info}",
							field: "nwparser.p1",
						},
					}),
				],
				on_success: processor_chain([
					set_field({
						dest: "nwparser.eventcategory",
						value: constant("1001000000"),
					}),
					dup3,
					dup4,
				]),
			}),
		]),
		set_field({
			dest: "@timestamp",
			value: field("event_time"),
		}),
	]));
	builder.Add(restore_flags);
	var chain = builder.Build();
	return {
		process: chain.Run,
	}
}

var dup0 = match({
	dissect: {
		tokenizer: "%{hostname} ossec: Alert Level: %{severity}; Rule: %{rule} - %{event_description}; Location: %{p0}",
		field: "nwparser.payload",
	},
});

var dup1 = linear_select([
	match({
		dissect: {
			tokenizer: "(%{shost}) %{saddr}-\u003e%{p1}",
			field: "nwparser.p0",
		},
	}),
	match({
		dissect: {
			tokenizer: "%{shost}-\u003e%{p1}",
			field: "nwparser.p0",
		},
	}),
]);

var dup2 = set_field({
	dest: "nwparser.eventcategory",
	value: constant("1001020200"),
});

var dup3 = set_field({
	dest: "nwparser.msg",
	value: field("$MSG"),
});

var dup4 = call({
	dest: "nwparser.",
	fn: SYSVAL,
	args: [
		field("$MSGID"),
		field("$ID1"),
	],
});
