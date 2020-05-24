[role="xpack"]

:modulename: ((.Module))
:has-dashboards: false

== ((.Module)) module

This is a module for receiving ((.DisplayName)) data over Syslog.

include::../include/gs-link.asciidoc[]

include::../include/configuring-intro.asciidoc[]

:fileset_ex: ((.Fileset))

include::../include/config-option-intro.asciidoc[]

[float]
==== `((.Fileset))` fileset settings

*`var.input`*::

The input from which messages are read. One of `file`, `tcp` or `udp`.

*`var.syslog_host`*::

The address to listen to UDP or TCP based syslog traffic.
Defaults to `localhost`.
Set to `0.0.0.0` to bind to all available interfaces.

*`var.syslog_port`*::

The UDP port to listen for syslog traffic. Defaults to `((.Port))`

NOTE: Ports below 1024 require Filebeat to run as root.

:has-dashboards!:

:fileset_ex!:

:modulename!:

