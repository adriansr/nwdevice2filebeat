//  Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
//  or more contributor license agreements. Licensed under the Elastic License;
//  you may not use this file except in compliance with the Elastic License.

function DeviceProcessor() {
	var builder = new processor.Chain();
	builder.Add(save_flags);
	builder.Add(strip_syslog_priority);
	builder.Add(chain1);
	builder.Add(populate_fields);
	builder.Add(restore_flags);
	var chain = builder.Build();
	return {
		process: chain.Run,
	}
}

var map_srcDirName = {
	keyvaluepairs: {
		"0": dup2,
		"1": dup1,
	},
};

var map_dstDirName = {
	keyvaluepairs: {
		"0": dup1,
		"1": dup2,
	},
};

var map_dir2SumType = {
	keyvaluepairs: {
		"0": constant("2"),
		"1": constant("3"),
	},
	"default": constant("0"),
};

var map_dir2Address = {
	keyvaluepairs: {
		"0": field("saddr"),
		"1": field("daddr"),
	},
	"default": field("saddr"),
};

var map_dir2Port = {
	keyvaluepairs: {
		"0": field("sport"),
		"1": field("dport"),
	},
	"default": field("sport"),
};

var dup1 = constant("INSIDE");

var dup2 = constant("OUTSIDE");

var hdr1 = match("HEADER#0:0033", "message", "%{month->} %{day->} %{year->} %{hhour}:%{hmin}:%{hsec->} %{hostip}: %ASA-%{level}-%{messageid}: %{payload}", processor_chain([
	setc("header_id","0033"),
]));

var select1 = linear_select([
	hdr1,
]);

var part1 = match("MESSAGE#0:113019:02", "nwparser.payload", "Duration: %{hour}h:%{min}m:%{second}s, Bytes xmt: %{sbytes}, Bytes rcv: %{rbytes}, Reason: %{result->} %{d2}, URL: %{url}", processor_chain([
	setc("eventcategory","1801030100"),
	date_time({
		dest: "event_time",
		args: ["month","day","year","hhour","hmin","hsec"],
		fmts: [
			[dB,dF,dW,dN,dU,dO],
		],
	}),
	call({
		dest: "nwparser.bytes",
		fn: CALC,
		args: [
			field("sbytes"),
			constant("+"),
			field("rbytes"),
		],
	}),
	duration({
		dest: "duration",
		args: ["hour","min","second"],
		fmts: [
			[uN,uU,uO],
		],
	}),
	duration({
		dest: "d2",
		args: ["d2"],
		fmts: [
			[uN,uc(":"),uU,uc(":"),uO],
		],
	}),
	page("page","url"),
]));

var msg1 = msg("113019:02", part1);

var chain1 = processor_chain([
	select1,
	msgid_select({
		"A": msg1,
	}),
	set_field({
		dest: "@timestamp",
		value: field("event_time"),
	}),
]);
