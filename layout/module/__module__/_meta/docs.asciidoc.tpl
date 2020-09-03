[role="xpack"]

:modulename: ((.Module))
:has-dashboards: false

== ((.Module | title)) module

experimental[]

This is a module for receiving ((.DisplayName)) logs over Syslog or a file.

include::../include/gs-link.asciidoc[]

include::../include/configuring-intro.asciidoc[]

:fileset_ex: ((.Fileset))

include::../include/config-option-intro.asciidoc[]

[float]
==== `((.Fileset))` fileset settings

experimental[]

NOTE: This was converted from RSA NetWitness log parser XML ((.LogParser.Description.Name | printf "%q")) device revision ((.LogParser.Version.Revision)).

*`var.input`*::

The input from which messages are read. One of `file`, `tcp` or `udp`.

*`var.syslog_host`*::

The address to listen to UDP or TCP based syslog traffic.
Defaults to `localhost`.
Set to `0.0.0.0` to bind to all available interfaces.

*`var.syslog_port`*::

The port to listen for syslog traffic. Defaults to `((.Port))`

NOTE: Ports below 1024 require Filebeat to run as root.

*`var.tz_offset`*::

By default, datetimes in the logs will be interpreted as relative to
the timezone configured in the host where {beatname_uc} is running. If ingesting
logs from a host on a different timezone, use this field to set the timezone
offset so that datetimes are correctly parsed. Valid values are in the form
±HH:mm, for example, `-07:00` for `UTC-7`.

*`var.rsa_fields`*::

Flag to control the addition of non-ECS fields to the event. Defaults to true,
which causes both ECS and custom fields under `rsa` to be added.

*`var.keep_raw_fields`*::

Flag to control the addition of the raw parser fields to the event. This fields
will be found under `rsa.raw`. The default is false.

:has-dashboards!:

:fileset_ex!:

:modulename!:

