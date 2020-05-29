udp:
host: "{{udp_host}}:{{udp_port}}"
tags: {{tags}}
processors:
((- setvar "basedir" (print "${path.home}/module/" .Module "/" .Fileset) -))
((- getvar "extra_processors" -))
- community_id:
- add_locale: ~
