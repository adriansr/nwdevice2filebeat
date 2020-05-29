tcp:
host: "{{tcp_host}}:{{tcp_port}}"
tags: {{tags}}
processors:
((- setvar "basedir" (print "${path.home}/module/" .Module "/" .Fileset) -))
((- getvar "extra_processors" -))
- community_id:
- add_locale: ~
